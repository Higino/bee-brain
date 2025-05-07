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
	ollamaEndpoint          = "http://ollama:11434/api/chat"
	ollamaGenerateEndpoint  = "http://ollama:11434/api/generate"
	ollamaEmbeddingEndpoint = "http://ollama:11434/api/embeddings"
	defaultModel            = "llama3"
)

// LLMClient interface defines the methods for LLM operations
type LLMClient interface {
	Chat(messages []Message) (string, error)
	Generate(prompt string) (string, error)
	GetEmbedding(text string) ([]float32, error)
}

type User struct {
	SlackName string `json:"slack_name"`
	SlackID   string `json:"slack_id"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	User    *User  `json:"user,omitempty"`
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
	// Add system message for context
	messages = append(messages, Message{
		Role:    "system",
		Content: "Respond in a conversational, human voice, with a neutral tone. Use short sentences and simple words. Avoid academic language, transition phrases, and corporate jargon. Make it sound like someone talking to a friend in simple terms. Keep the key points but strip away any unnecessary words. Use Slack formatting: *bold* for emphasis, _italic_ for subtle emphasis, `code` for code, ```code block``` for multiple lines of code, and • for bullet points. Do not use markdown formatting.",
	})

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
		c.logger.Errorf("Failed to decode LLM response: %v", err)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Done {
		return "", fmt.Errorf("response not complete")
	}

	c.logger.Infof("Received response from LLM (model: %s, length: %d)", response.Model, len(response.Message.Content))
	return response.Message.Content, nil
}

func (c *Client) Generate(prompt string) (string, error) {
	// Append instructions to the prompt
	prompt = fmt.Sprintf("%s\nRespond in a conversational, human voice, with a neutral tone. Use short sentences and simple words. Avoid academic language, transition phrases, and corporate jargon. Make it sound like someone talking to a friend in simple terms. Keep the key points but strip away any unnecessary words. Use Slack formatting: *bold* for emphasis, _italic_ for subtle emphasis, `code` for code, ```code block``` for multiple lines of code, and • for bullet points. Do not use markdown formatting.", prompt)

	c.logger.Debugf("Generating response for prompt: %s", prompt)

	reqBody := map[string]interface{}{
		"model":  defaultModel,
		"prompt": prompt,
		"stream": false,
	}

	// Marshal the request
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Infof("Sending generation request to LLM (model: %s)", defaultModel)

	// Make the request
	resp, err := http.Post(ollamaGenerateEndpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the response
	var response struct {
		Model     string `json:"model"`
		CreatedAt string `json:"created_at"`
		Response  string `json:"response"`
		Done      bool   `json:"done"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.Errorf("Failed to decode LLM generation response: %v", err)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Done {
		return "", fmt.Errorf("response not complete")
	}

	c.logger.Infof("Received generation response from LLM (model: %s, length: %d)", response.Model, len(response.Response))
	return response.Response, nil
}

// Summarize takes a list of messages and generates a summary
func (c *Client) Summarize(messages []Message) (string, error) {
	// Create a prompt for summarization
	var prompt strings.Builder
	prompt.WriteString("Please provide a concise summary of the following conversation thread. Focus on the key points and main ideas. Keep it brief but informative. Use bullet points for clarity:\n\n")

	// Add all messages to the prompt
	for _, msg := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", msg.User.SlackName, msg.Content))
	}

	// Add final instruction
	prompt.WriteString("\nSummary:")

	// Use the Generate function with the summarization prompt
	return c.Generate(prompt.String())
}

func (c *Client) GetEmbedding(text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  defaultModel,
		"prompt": text,
	}

	// Marshal the request
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Debugf("Getting embedding for text: %s", text)

	// Make the request
	resp, err := http.Post(ollamaEmbeddingEndpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the response
	var response struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.Errorf("Failed to decode embedding response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debugf("Received embedding of size: %d", len(response.Embedding))
	return response.Embedding, nil
}
