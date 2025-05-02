package slack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"beebrain/internal/llm"
	"beebrain/internal/vectordb"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type BeeBrainSlackEventHandler struct {
	client            *slack.Client
	llmClient         *llm.Client
	vectorDB          *vectordb.Client
	logger            *logrus.Logger
	signingSecret     string
	verificationToken string
	// Add a map to track processed events with typed values
	processedEvents sync.Map // key: string, value: time.Time
	llmMode         string
}

func NewBeeBrainSlackEventHandler(client *slack.Client, llmClient *llm.Client, vectorDB *vectordb.Client, logger *logrus.Logger, signingSecret, verificationToken, llmMode string) *BeeBrainSlackEventHandler {
	return &BeeBrainSlackEventHandler{
		client:            client,
		llmClient:         llmClient,
		vectorDB:          vectorDB,
		logger:            logger,
		signingSecret:     signingSecret,
		verificationToken: verificationToken,
		llmMode:           llmMode,
	}
}

// HandleSlackEvents handles incoming Slack events
func (h *BeeBrainSlackEventHandler) HandleSlackEvents(c echo.Context) error {
	// Read the request body once
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		h.logger.Error("Failed to read request body:", err)
		// Return 200 OK to prevent Slack from retrying
		return c.String(http.StatusOK, "Invalid request")
	}
	defer c.Request().Body.Close()

	// Parse and verify the event using slackevents
	slackEvent, err := slackevents.ParseEvent(
		json.RawMessage(body),
		slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: h.verificationToken}),
	)
	if err != nil {
		h.logger.Error("Failed to parse and verify event:", err)
		// Return 200 OK to prevent Slack from retrying
		return c.String(http.StatusOK, "Invalid request")
	}

	// Log the parsed event type
	h.logger.Debugf("Parsed event type: %s", slackEvent.Type)

	// Handle URL verification
	if slackEvent.Type == slackevents.URLVerification {
		return h.handleURLVerification(c, body)
	}

	// Handle callback events
	if slackEvent.Type == slackevents.CallbackEvent {
		innerEvent := slackEvent.InnerEvent
		h.logger.Debugf("Inner event type: %T", innerEvent.Data)

		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			return h.handleAppMention(c, ev)
		case *slackevents.MessageEvent:
			// Handle different message subtypes
			switch ev.SubType {
			case "": // no subtype, i.e. normal message
				return h.handleIncomingMessage(c, ev)
			default:
				return h.handleUnknownEvent(c, ev)
			}
		case *slackevents.ReactionAddedEvent:
			h.logger.Debugf("Processing reaction event: %+v", ev)
			return h.handleReactionAdded(c, ev)
		default:
			h.logger.Debugf("Unhandled event type: %T", ev)
			if msgEvent, ok := innerEvent.Data.(*slackevents.MessageEvent); ok {
				return h.handleUnknownEvent(c, msgEvent)
			}
			return c.NoContent(http.StatusOK)
		}
	}

	// Return 200 OK for unhandled event types
	return c.NoContent(http.StatusOK)
}

// handleURLVerification handles the Slack URL verification challenge
func (h *BeeBrainSlackEventHandler) handleURLVerification(c echo.Context, body []byte) error {
	var challenge struct {
		Token     string `json:"token"`
		Challenge string `json:"challenge"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(body, &challenge); err != nil {
		h.logger.Error("Failed to parse challenge:", err)
		// Return 200 OK to prevent Slack from retrying
		return c.String(http.StatusOK, "Invalid challenge")
	}

	// Return 200 OK with the challenge response
	return c.JSON(http.StatusOK, map[string]string{
		"challenge": challenge.Challenge,
	})
}

func (h *BeeBrainSlackEventHandler) getThreadContext(channel, threadTimestamp string) ([]llm.Message, error) {
	if threadTimestamp == "" {
		return nil, nil
	}

	// Get thread messages
	threadMessages, _, _, err := h.client.GetConversationReplies(&slack.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: threadTimestamp,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	// Convert thread messages to LLM messages
	messages := make([]llm.Message, 0, len(threadMessages))
	for _, msg := range threadMessages {
		// Determine the role based on whether it's a bot message
		role := "user"
		if msg.BotID != "" || msg.SubType == "bot_message" {
			role = "assistant"
		}

		messages = append(messages, llm.Message{
			Role:    role,
			Content: msg.Text,
			User: &llm.User{
				SlackName: msg.Username,
				SlackID:   msg.User,
			},
		})
	}

	return messages, nil
}

// isDuplicateEvent checks if an event has already been processed and stores it if not
func (h *BeeBrainSlackEventHandler) isDuplicateEvent(eventTimestamp string) bool {
	if eventTimestamp == "" {
		return false
	}

	if _, exists := h.processedEvents.Load(eventTimestamp); exists {
		h.logger.Debugf("Skipping duplicate event: %s", eventTimestamp)
		return true
	}

	h.processedEvents.Store(eventTimestamp, time.Now())
	// Clean up old events
	h.cleanupOldEvents()
	return false
}

func (h *BeeBrainSlackEventHandler) handleAppMention(c echo.Context, ev *slackevents.AppMentionEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent(ev.EventTimeStamp) {
		return c.NoContent(http.StatusOK)
	}

	h.logger.Infof("APP MENTION: Processing message from %s on channel %s", ev.User, ev.Channel)

	// Add reaction to show we're processing
	if err := h.client.AddReaction("eyes", slack.ItemRef{
		Channel:   ev.Channel,
		Timestamp: ev.TimeStamp,
	}); err != nil {
		h.logger.Error("Failed to add reaction:", err)
	}

	// Get user info for the person mentioning the bot
	userInfo, err := h.client.GetUserInfo(ev.User)
	if err != nil {
		userInfo = &slack.User{
			Name: "Unknown UserName",
			ID:   ev.User,
		}
	}
	h.logger.Debugf("User info retrieved: %s (%s)", userInfo.Name, userInfo.ID)

	// Get thread context if available
	threadMessages, err := h.getThreadContext(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		h.logger.Error("Failed to get thread context:", err)
	}

	messages := make([]llm.Message, 0, len(threadMessages)+2)
	if len(threadMessages) > 0 {
		messages = append(messages, threadMessages...)
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: ev.Text,
			User: &llm.User{
				SlackName: userInfo.Name,
				SlackID:   userInfo.ID,
			},
		})
	}

	// Get response from LLM with thread context
	response, err := h.getLLMResponse(messages)
	if err != nil {
		h.logger.Error("Failed to get LLM response:", err)
		response = "Sorry, I encountered an error processing your request."
	}

	// Post response to Slack
	if err := h.postResponse(ev.Channel, response, ev.ThreadTimeStamp); err != nil {
		h.logger.Error("Failed to post message:", err)
		return c.String(http.StatusOK, "Error processing request")
	}

	// Remove reaction
	if err := h.client.RemoveReaction("eyes", slack.ItemRef{
		Channel:   ev.Channel,
		Timestamp: ev.TimeStamp,
	}); err != nil {
		h.logger.Error("Failed to remove reaction:", err)
		// Always return a proper response to Slack
		return c.String(http.StatusOK, "Failed to remove reaction")
	}

	return c.String(http.StatusOK, "Message processed")
}

func (h *BeeBrainSlackEventHandler) handleIncomingMessage(c echo.Context, ev *slackevents.MessageEvent) error {
	// Get user info from Slack API
	userInfo, err := h.client.GetUserInfo(ev.User)
	if err != nil {
		h.logger.Warnf("Failed to get user info for %s: %v", ev.User, err)
		userInfo = &slack.User{
			Name: "Unknown User",
			ID:   ev.User,
		}
	}

	h.logger.Infof("IncommingMessage - User: %s (%s), Channel: %s, Thread: %s, Text: %s",
		userInfo.Name, userInfo.ID, ev.Channel, ev.ThreadTimeStamp, ev.Text)

	// // Get embedding for the message
	// embedding, err := h.llmClient.GetEmbedding(ev.Text)
	// if err != nil {
	// 	h.logger.Errorf("Failed to get embedding for message: %v", err)
	// 	return c.NoContent(http.StatusOK)
	// }

	// // Store message in VectorDB with a proper UUID
	// msg := vectordb.Message{
	// 	ID:        "", // Let VectorDB client generate a proper UUID
	// 	Text:      ev.Text,
	// 	UserID:    ev.User,
	// 	ChannelID: ev.Channel,
	// 	Timestamp: ev.TimeStamp,
	// 	ThreadID:  ev.ThreadTimeStamp, // The thread's root message timestamp (unique identifier for the thread)
	// 	Embedding: embedding,
	// }

	// if err := h.vectorDB.StoreMessage(msg); err != nil {
	// 	h.logger.Errorf("Failed to store message in VectorDB: %v", err)
	// }

	return c.NoContent(http.StatusOK)
}

func (h *BeeBrainSlackEventHandler) handleUnknownEvent(c echo.Context, ev *slackevents.MessageEvent) error {
	userID := ev.User
	if userID == "" && ev.Message != nil {
		userID = ev.Message.User
	} else {
		userID = "Unknown User"
	}

	h.logger.Infof("Unimplemented event: %s(%s) - User: %s, Channel: %s, Thread: %s, Text: %s",
		ev.Type, ev.SubType, userID, ev.Channel, ev.ThreadTimeStamp, ev.Text)

	return c.NoContent(http.StatusOK)
}

func (h *BeeBrainSlackEventHandler) getLLMResponse(messages []llm.Message) (string, error) {
	// Choose between Chat and Generate based on LLM_MODE
	if h.llmMode == "chat" {
		return h.llmClient.Chat(messages)
	} else {
		// Default to Generate mode
		// Concatenate all messages into a single string
		var fullContext strings.Builder
		for _, msg := range messages {
			fullContext.WriteString(fmt.Sprintf("%s|%s: %s\n", msg.User.SlackID, msg.User.SlackName, msg.Content))
		}
		resp, err := h.llmClient.Generate(fullContext.String())
		// Response comes markdown change to have it formated for slack

		return resp, err
	}
}

func (h *BeeBrainSlackEventHandler) postResponse(channel, response, threadTimestamp string) error {
	// Create message options with formatting enabled
	opts := []slack.MsgOption{
		slack.MsgOptionText(response, false), // false means don't escape special characters
		slack.MsgOptionEnableLinkUnfurl(),    // Enable link unfurling
		slack.MsgOptionAsUser(true),          // Post as the bot user
	}

	// Add thread timestamp if available
	if threadTimestamp != "" {
		opts = append(opts, slack.MsgOptionTS(threadTimestamp))
	}

	// Post the message
	_, _, err := h.client.PostMessage(channel, opts...)
	if err != nil {
		h.logger.Errorf("Failed to post message: %v", err)
		return err
	}

	return nil
}

// cleanupOldEvents removes events older than 1 hour from the processed events map
func (h *BeeBrainSlackEventHandler) cleanupOldEvents() {
	now := time.Now()

	// Type-safe wrapper for the Range callback
	processEvent := func(key, value interface{}) bool {
		// We know the types, so we can safely assert them
		eventKey := key.(string)
		timestamp := value.(time.Time)

		if now.Sub(timestamp) > time.Hour {
			h.processedEvents.Delete(eventKey)
		}
		return true
	}

	h.processedEvents.Range(processEvent)
}

// handleReactionAdded handles when a reaction is added to a message
func (h *BeeBrainSlackEventHandler) handleReactionAdded(c echo.Context, ev *slackevents.ReactionAddedEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent(ev.EventTimestamp) {
		return c.NoContent(http.StatusOK)
	}

	// Get user info for the person who added the reaction
	userInfo, err := h.client.GetUserInfo(ev.User)
	if err != nil {
		userInfo = &slack.User{
			Name: "Unknown User",
			ID:   ev.User,
		}
	}

	// Get the message that was reacted to
	message, err := h.client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: ev.Item.Channel,
		Latest:    ev.Item.Timestamp,
		Limit:     1,
		Inclusive: true,
	})
	if err != nil {
		h.logger.Errorf("Failed to get message info: %v", err)
		return c.NoContent(http.StatusOK)
	}

	// Log the reaction event
	h.logger.Infof("Reaction added by %s (%s): %s to message: %s",
		userInfo.Name,
		userInfo.ID,
		ev.Reaction,
		message.Messages[0].Text)

	// Check if the reaction is eyes
	if ev.Reaction == "robot_face" {
		h.logger.Infof("Detected 'robot_face' reaction from %s", userInfo.Name)

		// Get thread context if available
		threadMessages, err := h.getThreadContext(ev.Item.Channel, message.Messages[0].ThreadTimestamp)
		if err != nil {
			h.logger.Error("Failed to get thread context:", err)
		}

		// Prepare messages for summarization
		messages := make([]llm.Message, 0, len(threadMessages)+1)
		if len(threadMessages) > 0 {
			messages = append(messages, threadMessages...)
		}

		// Add the current message
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: message.Messages[0].Text,
			User: &llm.User{
				SlackName: userInfo.Name,
				SlackID:   userInfo.ID,
			},
		})

		// Get summary from LLM
		summary, err := h.llmClient.Summarize(messages)
		if err != nil {
			h.logger.Error("Failed to get summary:", err)
			summary = "Sorry, I encountered an error generating the summary."
		}

		// Format the response
		response := fmt.Sprintf("*Summary of the conversation:*\n\n%s", summary)

		// Determine the correct thread timestamp
		threadTimestamp := message.Messages[0].ThreadTimestamp
		if threadTimestamp == "" {
			// If no thread exists, use the message's timestamp as the thread starter
			threadTimestamp = message.Messages[0].Timestamp
		}

		// Post response to Slack in the correct thread
		if err := h.postResponse(ev.Item.Channel, response, threadTimestamp); err != nil {
			h.logger.Error("Failed to post message:", err)
		}
	}

	return c.NoContent(http.StatusOK)
}
