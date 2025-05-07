package tests

import (
	"testing"

	"beebrain/internal/llm"
	"beebrain/internal/llm/mocks"
	slackinternal "beebrain/internal/slack"
	slackmocks "beebrain/internal/slack/mocks"
	"beebrain/internal/vectordb"
	vectordbmocks "beebrain/internal/vectordb/mocks"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Ensure mock types implement their respective interfaces
var (
	_ slackinternal.SlackClient = (*slackmocks.MockSlackClient)(nil)
	_ llm.LLMClient             = (*mocks.MockLLMClient)(nil)
	_ vectordb.VectorDBClient   = (*vectordbmocks.MockVectorDBClient)(nil)
)

func TestNewConversationManager(t *testing.T) {
	// Create mock dependencies
	mockSlackClient := &slackmocks.MockSlackClient{}
	mockLLMClient := &mocks.MockLLMClient{}
	mockVectorDBClient := &vectordbmocks.MockVectorDBClient{}
	logger := logrus.New()

	tests := []struct {
		name      string
		vectorDB  vectordb.VectorDBClient
		wantNil   bool
		wantError bool
	}{
		{
			name:      "Valid initialization",
			vectorDB:  mockVectorDBClient,
			wantNil:   false,
			wantError: false,
		},
		{
			name:      "Nil vectorDB",
			vectorDB:  nil,
			wantNil:   true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := slackinternal.NewConversationManager(mockSlackClient, mockLLMClient, logger, "chat", tt.vectorDB)
			if tt.wantNil {
				assert.Nil(t, cm)
			} else {
				assert.NotNil(t, cm)
			}
		})
	}
}

func TestProcessIncommingMessage(t *testing.T) {
	// Create mock dependencies
	mockSlackClient := &slackmocks.MockSlackClient{}
	mockLLMClient := &mocks.MockLLMClient{}
	mockVectorDBClient := &vectordbmocks.MockVectorDBClient{}
	logger := logrus.New()

	// Create conversation manager
	cm := slackinternal.NewConversationManager(mockSlackClient, mockLLMClient, logger, "chat", mockVectorDBClient)
	assert.NotNil(t, cm)

	// Test data
	text := "Hello, world!"
	user := &slack.User{
		ID:   "U123456",
		Name: "Test User",
	}
	channelID := "C123456"
	embedding := []float32{0.1, 0.2, 0.3}

	// Set up expectations for loadHistory
	mockSlackClient.On("GetConversationHistory", mock.MatchedBy(func(params *slack.GetConversationHistoryParameters) bool {
		return params.ChannelID == channelID && params.Limit == 100
	})).Return(&slack.GetConversationHistoryResponse{
		Messages: []slack.Message{},
	}, nil)

	// Set up expectations for storing message
	mockLLMClient.On("GetEmbedding", text).Return(embedding, nil)
	mockVectorDBClient.On("StoreMessage", mock.MatchedBy(func(msg vectordb.Message) bool {
		return msg.Text == text && msg.UserID == user.ID && msg.ChannelID == channelID
	})).Return(nil)

	// Test ProcessIncommingMessage
	cm.ProcessIncommingMessage(text, user, channelID)

	// Verify expectations
	mockSlackClient.AssertExpectations(t)
	mockLLMClient.AssertExpectations(t)
	mockVectorDBClient.AssertExpectations(t)
}

func TestGetLastHourConversation(t *testing.T) {
	// Create mock dependencies
	mockSlackClient := &slackmocks.MockSlackClient{}
	mockLLMClient := &mocks.MockLLMClient{}
	mockVectorDBClient := &vectordbmocks.MockVectorDBClient{}
	logger := logrus.New()

	// Create conversation manager
	cm := slackinternal.NewConversationManager(mockSlackClient, mockLLMClient, logger, "chat", mockVectorDBClient)
	assert.NotNil(t, cm)

	// Test data
	channelID := "C123456"
	// Messages are returned in reverse chronological order by the Slack API
	mockMessages := []slack.Message{
		{
			Msg: slack.Msg{
				Text:     "Hi there",
				User:     "U789012",
				Username: "User2",
				BotID:    "B123456", // This should be marked as assistant
			},
		},
		{
			Msg: slack.Msg{
				Text:     "Hello",
				User:     "U123456",
				Username: "User1",
			},
		},
	}

	// Set up expectations
	mockSlackClient.On("GetConversationHistory", mock.AnythingOfType("*slack.GetConversationHistoryParameters")).
		Return(&slack.GetConversationHistoryResponse{
			Messages: mockMessages,
		}, nil)

	// Test GetLastHourConversation
	messages, err := cm.GetLastHourConversation(channelID)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	// After reversal, messages should be in chronological order
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)

	// Verify expectations
	mockSlackClient.AssertExpectations(t)
}

func TestGetThreadContext(t *testing.T) {
	// Create mock dependencies
	mockSlackClient := &slackmocks.MockSlackClient{}
	mockLLMClient := &mocks.MockLLMClient{}
	mockVectorDBClient := &vectordbmocks.MockVectorDBClient{}
	logger := logrus.New()

	// Create conversation manager
	cm := slackinternal.NewConversationManager(mockSlackClient, mockLLMClient, logger, "chat", mockVectorDBClient)
	assert.NotNil(t, cm)

	// Test data
	channelID := "C123456"
	threadTimestamp := "1234567890.123456"
	mockThreadMessages := []slack.Message{
		{
			Msg: slack.Msg{
				Text:     "Thread message 1",
				User:     "U123456",
				Username: "User1",
			},
		},
		{
			Msg: slack.Msg{
				Text:     "Thread message 2",
				User:     "U789012",
				Username: "User2",
				BotID:    "B123456", // This should be marked as assistant
			},
		},
	}

	// Set up expectations
	mockSlackClient.On("GetConversationReplies", mock.AnythingOfType("*slack.GetConversationRepliesParameters")).
		Return(mockThreadMessages, false, "", nil)

	// Test GetThreadContext
	messages, err := cm.GetThreadContext(channelID, threadTimestamp)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)

	// Verify expectations
	mockSlackClient.AssertExpectations(t)
}
