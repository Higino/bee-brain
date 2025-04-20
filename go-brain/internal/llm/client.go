package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	ollamaEndpoint = "http://ollama:11434/api/chat"
	defaultModel   = "llama3"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	logger *logrus.Logger
	Name   string
}

func NewClient(logger *logrus.Logger, name string) *Client {
	return &Client{
		logger: logger,
		Name:   name,
	}
}

func (c *Client) Chat(messages []Message) (string, error) {
	// Log the messages being sent
	c.logger.Debug("Sending messages to LLM:")
	for i, msg := range messages {
		c.logger.Debugf("Message %d [%s]: %s", i+1, msg.Role, msg.Content)
	}

	reqBody := map[string]interface{}{
		"model":    defaultModel,
		"messages": messages,
		"stream":   false, // Disable streaming for now
	}

	// Marshal the request
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Infof("Sending request to LLM (model: %s, messages: %d)", defaultModel, len(messages))

	// Make the request
	resp, err := http.Post(ollamaEndpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Log the raw response for debugging
	c.logger.Debugf("Raw LLM response: %s", string(body))

	// Parse the response
	var response struct {
		Model     string `json:"model"`
		CreatedAt string `json:"created_at"`
		Message   struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Done bool `json:"done"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.Errorf("Failed to decode LLM response: %v, body: %s", err, string(body))
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Done {
		return "", fmt.Errorf("response not complete")
	}

	c.logger.Infof("Received response from LLM (model: %s, length: %d)", response.Model, len(response.Message.Content))
	c.logger.Debugf("LLM response content: %s", response.Message.Content)
	return response.Message.Content, nil
}
