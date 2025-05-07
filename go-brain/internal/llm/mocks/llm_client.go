package mocks

import (
	"beebrain/internal/llm"

	"github.com/stretchr/testify/mock"
)

// MockLLMClient is a mock implementation of LLMClient
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) Chat(messages []llm.Message) (string, error) {
	args := m.Called(messages)
	return args.String(0), args.Error(1)
}

func (m *MockLLMClient) Generate(prompt string) (string, error) {
	args := m.Called(prompt)
	return args.String(0), args.Error(1)
}

func (m *MockLLMClient) GetEmbedding(text string) ([]float32, error) {
	args := m.Called(text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}
