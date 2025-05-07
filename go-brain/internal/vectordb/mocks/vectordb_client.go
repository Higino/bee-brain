package mocks

import (
	"beebrain/internal/vectordb"
	"context"

	"github.com/stretchr/testify/mock"
)

// MockVectorDBClient is a mock implementation of VectorDBClient
type MockVectorDBClient struct {
	mock.Mock
}

func (m *MockVectorDBClient) StoreMessage(msg vectordb.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockVectorDBClient) SearchSimilar(ctx context.Context, embedding []float32, limit uint64) ([]vectordb.Message, error) {
	args := m.Called(ctx, embedding, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]vectordb.Message), args.Error(1)
}
