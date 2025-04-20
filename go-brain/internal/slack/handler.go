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

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type BeeBrainSlackEventHandler struct {
	client            *slack.Client
	llmClient         *llm.Client
	logger            *logrus.Logger
	signingSecret     string
	verificationToken string
	// Add a map to track processed events
	processedEvents sync.Map
}

func NewBeeBrainSlackEventHandler(client *slack.Client, llmClient *llm.Client, logger *logrus.Logger, signingSecret, verificationToken string) *BeeBrainSlackEventHandler {
	return &BeeBrainSlackEventHandler{
		client:            client,
		llmClient:         llmClient,
		logger:            logger,
		signingSecret:     signingSecret,
		verificationToken: verificationToken,
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

	// Handle URL verification
	if slackEvent.Type == slackevents.URLVerification {
		return h.handleURLVerification(c, body)
	}

	// Handle callback events
	if slackEvent.Type == slackevents.CallbackEvent {
		innerEvent := slackEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			return h.handleAppMention(c, ev)
		case *slackevents.MessageEvent:
			// Handle different message subtypes
			switch ev.SubType {
			case "message_changed":
				return h.handleMessageChanged(c, ev)
			default:
				// Only respond to messages that mention the bot
				if !strings.Contains(ev.Text, "<@") {
					// Return 200 OK for non-mention messages
					return c.NoContent(http.StatusOK)
				}
				return h.handleMessage(c, ev)
			}
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

	// Skip the first message as it's the parent message
	if len(threadMessages) <= 1 {
		return nil, nil
	}

	// Convert thread messages to LLM messages
	messages := make([]llm.Message, 0, len(threadMessages)-1)
	for _, msg := range threadMessages[1:] { // Skip the first message
		// Get user info for better context
		user, err := h.client.GetUserInfo(msg.User)
		if err != nil {
			h.logger.Warnf("Failed to get user info for %s: %v", msg.User, err)
			user = &slack.User{Name: "Unknown User"}
		}

		// Determine the role based on whether it's a bot message
		role := "user"
		if msg.BotID != "" || msg.SubType == "bot_message" {
			role = "assistant"
		}

		messages = append(messages, llm.Message{
			Role:    role,
			Content: fmt.Sprintf("%s: %s", user.Name, msg.Text),
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

	h.logger.Infof("Received app mention from %s in %s", ev.User, ev.Channel)

	// Get thread context if available
	threadMessages, err := h.getThreadContext(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		h.logger.Error("Failed to get thread context:", err)
	}

	// Add reaction to show we're processing
	if err := h.client.AddReaction("eyes", slack.ItemRef{
		Channel:   ev.Channel,
		Timestamp: ev.TimeStamp,
	}); err != nil {
		h.logger.Error("Failed to add reaction:", err)
	}

	// Get response from LLM with thread context
	response, err := h.getLLMResponse(ev.Text, threadMessages)
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
	}

	return c.String(http.StatusOK, "Message processed")
}

func (h *BeeBrainSlackEventHandler) handleMessage(c echo.Context, ev *slackevents.MessageEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent(ev.EventTimeStamp) {
		return c.NoContent(http.StatusOK)
	}

	// Skip if this is a bot message or if the bot is not mentioned
	if ev.BotID != "" || ev.SubType == "bot_message" || !strings.Contains(ev.Text, "<@") {
		h.logger.Debugf("Skipping bot message or non-mention message: %s", ev.Text)
		return c.NoContent(http.StatusOK)
	}

	h.logger.Infof("Received message from %s in %s", ev.User, ev.Channel)

	// Add reaction to show we're processing
	if err := h.client.AddReaction("eyes", slack.ItemRef{
		Channel:   ev.Channel,
		Timestamp: ev.TimeStamp,
	}); err != nil {
		h.logger.Error("Failed to add reaction:", err)
	}

	// Get thread context if available
	threadMessages, err := h.getThreadContext(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		h.logger.Error("Failed to get thread context:", err)
	}

	// Get response from LLM with thread context
	response, err := h.getLLMResponse(ev.Text, threadMessages)
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
	}

	return c.String(http.StatusOK, "Message processed")
}

func (h *BeeBrainSlackEventHandler) handleMessageChanged(c echo.Context, ev *slackevents.MessageEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent(ev.EventTimeStamp) {
		return c.NoContent(http.StatusOK)
	}

	// Skip if this is a bot message or if the bot is not mentioned
	if ev.BotID != "" || ev.SubType == "bot_message" || !strings.Contains(ev.Text, "<@") {
		h.logger.Debugf("Skipping bot message or non-mention message: %s", ev.Text)
		return c.NoContent(http.StatusOK)
	}

	h.logger.Infof("Received message edit from %s in %s", ev.User, ev.Channel)

	// Add reaction to show we're processing
	if err := h.client.AddReaction("eyes", slack.ItemRef{
		Channel:   ev.Channel,
		Timestamp: ev.TimeStamp,
	}); err != nil {
		h.logger.Error("Failed to add reaction:", err)
	}

	// Get thread context if available
	threadMessages, err := h.getThreadContext(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		h.logger.Error("Failed to get thread context:", err)
	}

	// Get response from LLM with thread context
	response, err := h.getLLMResponse(ev.Text, threadMessages)
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
	}

	return c.NoContent(http.StatusOK)
}

func (h *BeeBrainSlackEventHandler) getLLMResponse(text string, threadMessages []llm.Message) (string, error) {
	messages := make([]llm.Message, 0, len(threadMessages)+2)

	// Add system message for context
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("You are %s, a helpful AI assistant. Answer questions concisely and prevent repeating yourself.", h.llmClient.Name),
	})

	// Add thread messages if available
	if len(threadMessages) > 0 {
		messages = append(messages, threadMessages...)
	}

	// Add the current message
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: text,
	})

	return h.llmClient.Chat(messages)
}

func (h *BeeBrainSlackEventHandler) postResponse(channel, response, threadTimestamp string) error {
	// Log the thread context for debugging
	h.logger.Debugf("Posting response to channel %s, thread: %s", channel, threadTimestamp)

	// Create message options
	opts := []slack.MsgOption{
		slack.MsgOptionText(response, false),
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
	h.processedEvents.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok {
			if now.Sub(timestamp) > time.Hour {
				h.processedEvents.Delete(key)
			}
		}
		return true
	})
}
