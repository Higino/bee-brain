package slack

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"beebrain/internal/llm"
	"beebrain/internal/vectordb"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// SlackClient interface defines the methods we need from slack.Client
type SlackClient interface {
	GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
	GetConversationReplies(params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error)
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
}

// TruncatingFormatter is a custom formatter that truncates long messages
type TruncatingFormatter struct {
	Formatter logrus.Formatter
}

func (f *TruncatingFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Truncate message if it's too long
	msg := entry.Message
	if len(msg) > 50 {
		msg = msg[:50] + "..."
	}
	entry.Message = msg
	return f.Formatter.Format(entry)
}

type ConversationManager struct {
	client         SlackClient
	llmClient      llm.LLMClient
	logger         *logrus.Logger
	messageHistory *sync.Map
	llmMode        string
	vectorDB       vectordb.VectorDBClient
}

func NewConversationManager(client SlackClient, llmClient llm.LLMClient, logger *logrus.Logger, llmMode string, vectorDB vectordb.VectorDBClient) *ConversationManager {
	if vectorDB == nil {
		logger.Error("vectorDB client is not initialized")
		return nil
	}

	// Set up custom formatter that truncates long messages
	logger.SetFormatter(&TruncatingFormatter{
		Formatter: &logrus.TextFormatter{
			DisableQuote: true,
		},
	})

	return &ConversationManager{
		client:         client,
		llmClient:      llmClient,
		logger:         logger,
		messageHistory: &sync.Map{},
		llmMode:        llmMode,
		vectorDB:       vectorDB,
	}
}

func (m *ConversationManager) GetLastHourConversation(channel string) ([]llm.Message, error) {
	// Get the last hour of conversation
	oneHourAgo := time.Now().Add(-1 * time.Hour).Unix()
	history, err := m.client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Oldest:    fmt.Sprintf("%d.000000", oneHourAgo),
		Limit:     100, // Maximum number of messages to fetch
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Convert history messages to LLM messages
	messages := make([]llm.Message, 0, len(history.Messages))
	for _, msg := range history.Messages {
		// Skip thread replies as they're handled separately
		if msg.ThreadTimestamp != "" {
			continue
		}

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

	// Reverse the messages to maintain chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (m *ConversationManager) GetThreadContext(channel, threadTimestamp string) ([]llm.Message, error) {
	if threadTimestamp != "" {
		// Get thread messages
		threadMessages, _, _, err := m.client.GetConversationReplies(&slack.GetConversationRepliesParameters{
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

	// If no thread timestamp, get the last hour of conversation
	return m.GetLastHourConversation(channel)
}
func (m *ConversationManager) ProcessMessage(threadMessages []llm.Message, text string, userInfo *slack.User) (string, error) {
	messages := make([]llm.Message, 0, len(threadMessages)+2)
	if len(threadMessages) > 0 {
		messages = append(messages, threadMessages...)
	}
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: text,
		User: &llm.User{
			SlackName: userInfo.Name,
			SlackID:   userInfo.ID,
		},
	})

	// Get response from LLM with thread context
	return m.getLLMResponse(messages)
}

func (m *ConversationManager) ProcessReaction(reaction string) (string, error) {
	return m.llmClient.Generate(fmt.Sprintf("User reacted with :%s: to my message", reaction))
}

func (m *ConversationManager) ProcessIncommingMessage(text string, user *slack.User, channelID string) {
	if _, exists := m.messageHistory.Load(channelID); !exists {
		m.loadHistory(channelID)
	}

	// Check if vectorDB is initialized
	if m.vectorDB == nil {
		m.logger.Error("vectorDB client is not initialized")
		return
	}

	// Get embedding for the message
	embedding, err := m.llmClient.GetEmbedding(text)
	if err != nil {
		m.logger.Errorf("Failed to get embedding for message: %v", err)
		return
	}

	// Create message for vectorDB
	msg := vectordb.Message{
		Text:      text,
		UserID:    user.ID,
		ChannelID: channelID,
		Timestamp: time.Now().Format(time.RFC3339),
		Embedding: embedding,
	}

	// Store message in vectorDB
	if err := m.vectorDB.StoreMessage(msg); err != nil {
		m.logger.Errorf("Failed to store message in vectorDB: %v", err)
		return
	}

	m.logger.Infof("Successfully stored message in vectorDB for channel %s", channelID)
}

func (m *ConversationManager) loadHistory(channelID string) {
	history, err := m.client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     100,
	})
	if err != nil {
		m.logger.Errorf("Failed to get conversation history: %v", err)
		return
	}

	for _, msg := range history.Messages {
		m.logger.Infof("Message: %.50s belonging to thread %s", msg.Text, msg.ThreadTimestamp)

		// Check for attachments
		if len(msg.Attachments) > 0 {
			// Printout the attachments
			for _, attachment := range msg.Attachments {
				m.logger.Infof("Attachment: %v", attachment)
			}
		}
	}

	m.messageHistory.Store(channelID, history.Messages)
}

func (m *ConversationManager) getLLMResponse(messages []llm.Message) (string, error) {
	// Choose between Chat and Generate based on LLM_MODE
	if m.llmMode == "chat" {
		return m.llmClient.Chat(messages)
	} else {
		// Default to Generate mode
		// Concatenate all messages into a single string
		var fullContext strings.Builder
		for _, msg := range messages {
			fullContext.WriteString(fmt.Sprintf("%s|%s: %s\n", msg.User.SlackID, msg.User.SlackName, msg.Content))
		}
		return m.llmClient.Generate(fullContext.String())
	}
}

func (m *ConversationManager) PostResponse(channel, response, threadTimestamp string) error {
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
	_, _, err := m.client.PostMessage(channel, opts...)
	if err != nil {
		m.logger.Errorf("Failed to post message: %v", err)
		return err
	}

	return nil
}
