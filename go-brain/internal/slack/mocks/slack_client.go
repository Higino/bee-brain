package mocks

import (
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/mock"
)

// MockSlackClient is a mock implementation of SlackClient
type MockSlackClient struct {
	mock.Mock
}

func (m *MockSlackClient) GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*slack.GetConversationHistoryResponse), args.Error(1)
}

func (m *MockSlackClient) GetConversationReplies(params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error) {
	args := m.Called(params)
	return args.Get(0).([]slack.Message), args.Bool(1), args.String(2), args.Error(3)
}

func (m *MockSlackClient) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	args := m.Called(channelID, options)
	return args.String(0), args.String(1), args.Error(2)
}
