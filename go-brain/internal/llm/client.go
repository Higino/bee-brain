package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	ollamaEndpoint = "http://0.0.0.0:11434/api/generate"
	defaultModel   = "llama2"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	logger *logrus.Logger
	name   string
}

func NewClient(logger *logrus.Logger, name string) *Client {
	return &Client{
		logger: logger,
		name:   name,
	}
}

func (c *Client) Chat(messages []Message) (string, error) {
	// Add system message with bot identity
	systemMessage := Message{
		Role:    "system",
		Content: fmt.Sprintf("You are %s, a helpful AI assistant. Answer questions concisely and accurately.", c.name),
	}

	// Prepare the prompt from messages
	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("%s: %s\n", systemMessage.Role, systemMessage.Content))
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"model":  defaultModel,
		"prompt": prompt.String(),
		"stream": false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	c.logger.Infof("Sending request to LLM (model: %s, prompt length: %d)", defaultModel, len(prompt.String()))

	resp, err := http.Post(ollamaEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var response struct {
		Model     string `json:"model"`
		CreatedAt string `json:"created_at"`
		Response  string `json:"response"`
		Done      bool   `json:"done"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.Errorf("Failed to decode LLM response: %v", err)
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	c.logger.Infof("Received response from LLM (model: %s, length: %d)", response.Model, len(response.Response))

	return response.Response, nil
}
