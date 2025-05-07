package vectordb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	go_client "github.com/qdrant/go-client/qdrant"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	collectionName = "slack_messages"
	vectorSize     = 4096 // Size of embeddings from Ollama
)

// VectorDBClient interface defines the methods for vector database operations
type VectorDBClient interface {
	StoreMessage(msg Message) error
	SearchSimilar(ctx context.Context, embedding []float32, limit uint64) ([]Message, error)
}

type Client struct {
	collectionsClient go_client.CollectionsClient
	pointsClient      go_client.PointsClient
	logger            *logrus.Logger
}

func NewClient(logger *logrus.Logger) (*Client, error) {
	// Set default values
	host := os.Getenv("QDRANT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("QDRANT_PORT")
	if port == "" {
		port = "6334"
	}

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	address := fmt.Sprintf("%s:%s", host, port)
	logger.Infof("Attempting to connect to Qdrant at %s", address)

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("connection to Qdrant timed out after 5 seconds. Please check if Qdrant is running at %s", address)
		}
		return nil, fmt.Errorf("failed to connect to Qdrant at %s: %w", address, err)
	}

	logger.Info("Successfully connected to Qdrant")

	return &Client{
		collectionsClient: go_client.NewCollectionsClient(conn),
		pointsClient:      go_client.NewPointsClient(conn),
		logger:            logger,
	}, nil
}

type Message struct {
	ID        string
	Text      string
	UserID    string
	ChannelID string
	Timestamp string
	ThreadID  string
	Embedding []float32
}

func (c *Client) InitializeCollection(ctx context.Context) error {
	// Check if collection exists
	collections, err := c.collectionsClient.List(ctx, &go_client.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	exists := false
	for _, collection := range collections.Collections {
		if collection.Name == collectionName {
			exists = true
			break
		}
	}

	if !exists {
		// Create collection if it doesn't exist
		_, err := c.collectionsClient.Create(ctx, &go_client.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: &go_client.VectorsConfig{
				Config: &go_client.VectorsConfig_Params{
					Params: &go_client.VectorParams{
						Size:     vectorSize,
						Distance: go_client.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		c.logger.Infof("Created new collection for slack messages with vector size %d", vectorSize)
	}

	return nil
}

func (c *Client) StoreMessage(msg Message) error {
	// Generate a valid UUID for the message ID if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	c.logger.Debugf("Storing message with ID: %s, Text: %s", msg.ID, msg.Text)

	// Create a new background context for the upsert operation
	upsertCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert message to Qdrant point
	point := &go_client.PointStruct{
		Id: &go_client.PointId{
			PointIdOptions: &go_client.PointId_Uuid{
				Uuid: msg.ID,
			},
		},
		Vectors: &go_client.Vectors{
			VectorsOptions: &go_client.Vectors_Vector{
				Vector: &go_client.Vector{
					Data: msg.Embedding,
				},
			},
		},
		Payload: map[string]*go_client.Value{
			"text":       {Kind: &go_client.Value_StringValue{StringValue: msg.Text}},
			"user_id":    {Kind: &go_client.Value_StringValue{StringValue: msg.UserID}},
			"channel_id": {Kind: &go_client.Value_StringValue{StringValue: msg.ChannelID}},
			"timestamp":  {Kind: &go_client.Value_StringValue{StringValue: msg.Timestamp}},
			"thread_id":  {Kind: &go_client.Value_StringValue{StringValue: msg.ThreadID}},
		},
	}

	c.logger.Debugf("Upserting point to collection: %s with ID: %s", collectionName, msg.ID)

	// Upsert the point
	upsertResponse, err := c.pointsClient.Upsert(upsertCtx, &go_client.UpsertPoints{
		CollectionName: collectionName,
		Points:         []*go_client.PointStruct{point},
	})
	if err != nil {
		c.logger.Errorf("Failed to upsert point: %v, Response: %+v", err, upsertResponse)
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	c.logger.Debugf("Successfully stored message in Qdrant: %s", msg.ID)
	return nil
}

func (c *Client) SearchSimilar(ctx context.Context, embedding []float32, limit uint64) ([]Message, error) {
	// Create a new context with timeout for the search operation
	searchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Search for similar points
	searchResult, err := c.pointsClient.Search(searchCtx, &go_client.SearchPoints{
		CollectionName: collectionName,
		Vector:         embedding,
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search points: %w", err)
	}

	// Convert results to Message structs
	messages := make([]Message, 0, len(searchResult.Result))
	for _, result := range searchResult.Result {
		payload := result.Payload
		messages = append(messages, Message{
			ID:        result.Id.GetUuid(),
			Text:      payload["text"].GetStringValue(),
			UserID:    payload["user_id"].GetStringValue(),
			ChannelID: payload["channel_id"].GetStringValue(),
			Timestamp: payload["timestamp"].GetStringValue(),
			ThreadID:  payload["thread_id"].GetStringValue(),
			Embedding: result.Vectors.GetVector().Data,
		})
	}

	return messages, nil
}
