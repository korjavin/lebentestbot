package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/korjavin/lebentestbot/models"
)

const (
	deepseekAPIURL = "https://api.deepseek.com/v1/chat/completions"
	apiTimeoutSec  = 60 // Increased to 60 seconds to allow for more thorough responses
)

// DeepseekClient manages interactions with Deepseek API
type DeepseekClient struct {
	apiKey string
}

// NewDeepseekClient creates a new Deepseek API client
func NewDeepseekClient(apiKey string) *DeepseekClient {
	return &DeepseekClient{
		apiKey: apiKey,
	}
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekRequest struct {
	Model    string            `json:"model"`
	Messages []deepseekMessage `json:"messages"`
	Timeout  int               `json:"timeout,omitempty"`
}

type deepseekResponseChoice struct {
	Message deepseekMessage `json:"message"`
}

type deepseekResponse struct {
	Choices []deepseekResponseChoice `json:"choices"`
	ID      string                   `json:"id,omitempty"`
	Usage   map[string]interface{}   `json:"usage,omitempty"`
}

// AnalyzeQuestion uses Deepseek to analyze a question and provide insights
func (c *DeepseekClient) AnalyzeQuestion(question *models.Question) (string, int, error) {
	startTime := time.Now()
	log.Printf("Starting analysis of question %d with Deepseek", question.Number)

	// Construct the prompt
	prompt := fmt.Sprintf(`
I have a question from a German citizen test. Please help me with the following tasks:

1. Translate the question to English
2. Determine the correct answer and explain why this is the correct answer
4. Suggest a mnemonic or memory aid to help remember this fact
5. If there are challenging German words, explain them and suggest ways to remember them

Question: %s

Answers: %v

Please organize your response in clearly labeled sections and be concise. Answer in plain text.
`, question.Question, question.Answers)

	// Create request body
	reqBody := deepseekRequest{
		Model: "deepseek-chat",
		Messages: []deepseekMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return "", -1, err
	}

	// Log the request payload (truncated for clarity)
	reqJSONStr := string(reqJSON)
	if len(reqJSONStr) > 200 {
		log.Printf("Deepseek request payload (truncated): %s...", reqJSONStr[:200])
	} else {
		log.Printf("Deepseek request payload: %s", reqJSONStr)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeoutSec*time.Second)
	defer cancel()

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", deepseekAPIURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return "", -1, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Send the request with timing
	log.Printf("Sending request to Deepseek API...")
	client := &http.Client{
		Timeout: time.Duration(apiTimeoutSec) * time.Second,
	}

	reqSentTime := time.Now()
	resp, err := client.Do(req)
	reqDuration := time.Since(reqSentTime)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Deepseek API request timed out after %v", reqDuration)
			return "Sorry, the AI analysis timed out. Please try again later.", -1, err
		}
		log.Printf("Error sending request to Deepseek: %v after %v", err, reqDuration)
		return "", -1, err
	}
	defer resp.Body.Close()

	log.Printf("Received response from Deepseek API in %v with status code: %d", reqDuration, resp.StatusCode)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return "", -1, err
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("API request failed with status %d: %s", resp.StatusCode, string(body))
		return "", -1, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Log response (truncated for large responses)
	bodyStr := string(body)
	if len(bodyStr) > 300 {
		log.Printf("Deepseek response (truncated): %s...", bodyStr[:300])
	} else {
		log.Printf("Deepseek response: %s", bodyStr)
	}

	// Parse the response
	var deepseekResp deepseekResponse
	if err := json.Unmarshal(body, &deepseekResp); err != nil {
		log.Printf("Error parsing Deepseek response: %v", err)
		return "", -1, err
	}

	if len(deepseekResp.Choices) == 0 {
		log.Printf("No choices in API response")
		return "", -1, fmt.Errorf("no choices in API response")
	}

	// Attempt to determine the correct answer from the response
	content := deepseekResp.Choices[0].Message.Content
	rightAnswer := extractRightAnswerFromContent(content, question)

	totalDuration := time.Since(startTime)
	log.Printf("Analysis of question %d completed in %v. Content length: %d",
		question.Number, totalDuration, len(content))

	return content, rightAnswer, nil
}

// extractRightAnswerFromContent tries to determine the right answer index from the AI response
// This is a very simplified implementation
func extractRightAnswerFromContent(content string, question *models.Question) int {
	// If there's already a known right answer, use it
	if question.RightAnswer >= 0 && question.RightAnswer < len(question.Answers) {
		return question.RightAnswer
	}

	// This is a placeholder for more sophisticated answer extraction logic
	// In a real implementation, you would analyze the content to try to determine
	// which answer the AI believes is correct

	// For now, just returning -1 (unknown)
	return -1
}
