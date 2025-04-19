package slack

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"beebrain/internal/llm"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type EventHandler struct {
	client        *slack.Client
	llmClient     *llm.Client
	logger        *logrus.Logger
	signingSecret string
	// Add a map to track processed events
	processedEvents sync.Map
}

func NewEventHandler(client *slack.Client, llmClient *llm.Client, logger *logrus.Logger, signingSecret string) *EventHandler {
	return &EventHandler{
		client:        client,
		llmClient:     llmClient,
		logger:        logger,
		signingSecret: signingSecret,
	}
}

func (h *EventHandler) HandleEvents(c echo.Context) error {
	// Read the request body first
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		h.logger.Error("Failed to read request body:", err)
		return c.String(http.StatusOK, "Invalid request")
	}

	// Create a new request body reader for the verifier
	bodyReader := bytes.NewReader(body)
	c.Request().Body = io.NopCloser(bodyReader)

	// Create verifier with the original request
	verifier, err := slack.NewSecretsVerifier(c.Request().Header, h.signingSecret)
	if err != nil {
		h.logger.Error("Failed to create verifier:", err)
		return c.String(http.StatusOK, "Invalid request")
	}

	// Write the body to the verifier
	if _, err := verifier.Write(body); err != nil {
		h.logger.Error("Failed to write to verifier:", err)
		return c.String(http.StatusOK, "Invalid request")
	}

	if err := verifier.Ensure(); err != nil {
		h.logger.Error("Failed to verify request:", err)
		return c.String(http.StatusOK, "Invalid request")
	}

	// Parse the event
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		h.logger.Error("Failed to parse event:", err)
		return c.String(http.StatusOK, "Invalid request")
	}

	// Handle Slack challenge
	if eventsAPIEvent.Type == slackevents.URLVerification {
		h.logger.Info("Handling URL verification challenge")
		var challenge struct {
			Token     string `json:"token"`
			Challenge string `json:"challenge"`
			Type      string `json:"type"`
		}
		if err := json.Unmarshal(body, &challenge); err != nil {
			h.logger.Error("Failed to parse challenge:", err)
			return c.String(http.StatusOK, "Invalid challenge")
		}
		return c.JSON(http.StatusOK, map[string]string{
			"challenge": challenge.Challenge,
		})
	}

	// Check for retry headers
	retryNum := c.Request().Header.Get("X-Slack-Retry-Num")
	if retryNum != "" {
		h.logger.Debugf("Received retry attempt #%s", retryNum)
		// Always acknowledge retries with 200 OK
		return c.NoContent(http.StatusOK)
	}

	// Handle other events
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent

		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			// Skip if this is a duplicate event
			if ev.EventTimeStamp != "" {
				if _, exists := h.processedEvents.Load(ev.EventTimeStamp); exists {
					h.logger.Debugf("Skipping duplicate app mention event: %s", ev.EventTimeStamp)
					return c.NoContent(http.StatusOK)
				}
				h.processedEvents.Store(ev.EventTimeStamp, time.Now())
				// Clean up old events
				h.cleanupOldEvents()
			}

			h.logger.Infof("Received app mention from %s in %s", ev.User, ev.Channel)

			// Add reaction to show we're processing
			if err := h.client.AddReaction("eyes", slack.ItemRef{
				Channel:   ev.Channel,
				Timestamp: ev.TimeStamp,
			}); err != nil {
				h.logger.Error("Failed to add reaction:", err)
			}

			// Prepare messages for LLM
			messages := []llm.Message{
				{
					Role:    "user",
					Content: ev.Text,
				},
			}

			// Get response from LLM
			response, err := h.llmClient.Chat(messages)
			if err != nil {
				h.logger.Error("Failed to get LLM response:", err)
				response = "Sorry, I encountered an error processing your request."
			}

			// Post response to Slack in the same thread if it's a thread
			if ev.ThreadTimeStamp != "" {
				_, _, err = h.client.PostMessage(ev.Channel,
					slack.MsgOptionText(response, false),
					slack.MsgOptionTS(ev.ThreadTimeStamp))
			} else {
				_, _, err = h.client.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
			}
			if err != nil {
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

		case *slackevents.MessageEvent:
			// Skip if this is a duplicate event
			if ev.EventTimeStamp != "" {
				if _, exists := h.processedEvents.Load(ev.EventTimeStamp); exists {
					h.logger.Debugf("Skipping duplicate message event: %s", ev.EventTimeStamp)
					return c.NoContent(http.StatusOK)
				}
				h.processedEvents.Store(ev.EventTimeStamp, time.Now())
				// Clean up old events
				h.cleanupOldEvents()
			}

			// Only respond to direct messages or messages in channels where the bot is mentioned
			if ev.BotID != "" || ev.SubType == "bot_message" {
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

			// Prepare messages for LLM
			messages := []llm.Message{
				{
					Role:    "user",
					Content: ev.Text,
				},
			}

			// Get response from LLM
			response, err := h.llmClient.Chat(messages)
			if err != nil {
				h.logger.Error("Failed to get LLM response:", err)
				response = "Sorry, I encountered an error processing your request."
			}

			// Post response to Slack in the same thread if it's a thread
			if ev.ThreadTimeStamp != "" {
				_, _, err = h.client.PostMessage(ev.Channel,
					slack.MsgOptionText(response, false),
					slack.MsgOptionTS(ev.ThreadTimeStamp))
			} else {
				_, _, err = h.client.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
			}
			if err != nil {
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
		}
	}

	return c.NoContent(http.StatusOK)
}

// cleanupOldEvents removes events older than 1 hour from the processed events map
func (h *EventHandler) cleanupOldEvents() {
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
