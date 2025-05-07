package slack

import (
	"beebrain/internal/llm"
	"beebrain/internal/vectordb"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type BeeBrainSlackHandler struct {
	client              *slack.Client
	logger              *logrus.Logger
	signingSecret       string
	verificationToken   string
	processedEvents     sync.Map // key: string, value: time.Time
	botUserID           string
	conversationManager *ConversationManager
}

func NewBeeBrainSlackHandler(client *slack.Client, llmClient *llm.Client, vectorDB *vectordb.Client, logger *logrus.Logger, signingSecret, verificationToken, llmMode string) *BeeBrainSlackHandler {
	// Get bot user ID
	auth, err := client.AuthTest()
	if err != nil {
		logger.Fatal("Failed to get bot user ID")
	}

	return &BeeBrainSlackHandler{
		client:              client,
		logger:              logger,
		signingSecret:       signingSecret,
		verificationToken:   verificationToken,
		botUserID:           auth.UserID,
		conversationManager: NewConversationManager(client, llmClient, logger, llmMode, vectorDB),
	}
}

// HandleSlackEvents handles incoming Slack events
func (h *BeeBrainSlackHandler) HandleSlackEvents(c echo.Context) error {
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
				return h.handleIncommingMessage(c, ev)
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
func (h *BeeBrainSlackHandler) handleURLVerification(c echo.Context, body []byte) error {
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

// isDuplicateEvent checks if an event has already been processed and stores it if not
func (h *BeeBrainSlackHandler) isDuplicateEvent(eventType, eventTimestamp string) bool {
	if eventTimestamp == "" {
		return false
	}

	// Create a composite key of event type and timestamp
	eventKey := fmt.Sprintf("%s:%s", eventType, eventTimestamp)

	if _, exists := h.processedEvents.Load(eventKey); exists {
		h.logger.Debugf("Skipping duplicate event: %s", eventKey)
		return true
	}

	h.processedEvents.Store(eventKey, time.Now())
	// Clean up old events
	h.cleanupOldEvents()
	return false
}

func (h *BeeBrainSlackHandler) handleAppMention(c echo.Context, ev *slackevents.AppMentionEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent("app_mention", ev.EventTimeStamp) {
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
	threadMessages, err := h.conversationManager.GetThreadContext(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		h.logger.Error("Failed to get thread context:", err)
	}

	// Process the message and get response
	response, err := h.conversationManager.ProcessMessage(threadMessages, ev.Text, userInfo)
	if err != nil {
		h.logger.Error("Failed to process message:", err)
		response = "Sorry, I encountered an error processing your request."
	}

	// Post response to Slack
	if err := h.conversationManager.PostResponse(ev.Channel, response, ev.ThreadTimeStamp); err != nil {
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

func (h *BeeBrainSlackHandler) handleIncommingMessage(c echo.Context, ev *slackevents.MessageEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent("message", ev.EventTimeStamp) {
		return c.NoContent(http.StatusOK)
	}

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

	h.conversationManager.ProcessIncommingMessage(ev.Text, userInfo, ev.Channel)
	return c.NoContent(http.StatusOK)
}

func (h *BeeBrainSlackHandler) handleUnknownEvent(c echo.Context, ev *slackevents.MessageEvent) error {
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

func (h *BeeBrainSlackHandler) handleReactionAdded(c echo.Context, ev *slackevents.ReactionAddedEvent) error {
	// Skip if this is a duplicate event
	if h.isDuplicateEvent("reaction_added", ev.EventTimestamp) {
		return c.NoContent(http.StatusOK)
	}

	// Check if this is a reaction to a bot message
	if ev.ItemUser != h.botUserID {
		h.logger.Info("Reaction is not on a bot message, skipping processing")
		return c.NoContent(http.StatusOK)
	}

	// Process the reaction
	response, err := h.conversationManager.ProcessReaction(ev.Reaction)
	if err != nil {
		h.logger.Error("Failed to process reaction:", err)
		return c.String(http.StatusOK, "Error processing reaction")
	}

	// Post the response
	if err := h.conversationManager.PostResponse(ev.Item.Channel, response, ev.Item.Timestamp); err != nil {
		h.logger.Error("Failed to post message:", err)
		return c.String(http.StatusOK, "Error posting response")
	}

	return c.NoContent(http.StatusOK)
}

// cleanupOldEvents removes events older than 1 hour from the processed events map
func (h *BeeBrainSlackHandler) cleanupOldEvents() {
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
