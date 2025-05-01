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
	ollamaEndpoint         = "http://ollama:11434/api/chat"
	ollamaGenerateEndpoint = "http://ollama:11434/api/generate"
	defaultModel           = "llama3"
)

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
