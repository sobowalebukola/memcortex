package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Summarizer struct {
	ollamaHost string
	model      string
	client     *http.Client
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewSummarizer() *Summarizer {
	return &Summarizer{
		ollamaHost: getEnv("OLLAMA_HOST", "http://ollama:11434"),
		model:      getEnv("SUMMARIZATION_MODEL", "deepseek-r1:1.5b"),
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (s *Summarizer) SummarizeMemories(ctx context.Context, memories []string, userID string) (string, error) {
	if len(memories) == 0 {
		return "", fmt.Errorf("no memories to summarize")
	}

	// This is the line that will finally show up in your terminal
	fmt.Printf("\n[SUMMARIZER] Triggered for user: %s | Batch size: %d\n", userID, len(memories))

	prompt := s.buildSummarizationPrompt(memories, userID)
	
	reqBody := OllamaRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", 
		fmt.Sprintf("%s/api/generate", s.ollamaHost), 
		bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	fmt.Println("[SUMMARIZER] Calling Ollama (DeepSeek)...")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println("[SUMMARIZER] Successfully generated summary.")
	return ollamaResp.Response, nil
}

func (s *Summarizer) buildSummarizationPrompt(memories []string, userID string) string {
	memoriesText := ""
	for _, mem := range memories {
		memoriesText += fmt.Sprintf("- %s\n", mem)
	}

	return fmt.Sprintf(`Summarize these memories for user "%s" into a single concise paragraph. 
Do not include <think> tags. Just provide the raw summary.
Memories:
%s`, userID, memoriesText)
}

func GetSummaryThreshold() int {
	val, _ := strconv.Atoi(getEnv("SUMMARY_THRESHOLD", "2"))
	return val
}

func GetSummaryBatchSize() int {
	val, _ := strconv.Atoi(getEnv("SUMMARY_BATCH_SIZE", "2"))
	return val
}

func GetSummaryMaxAgeDays() int {
	val, _ := strconv.Atoi(getEnv("SUMMARY_MAX_AGE_DAYS", "0"))
	return val
}