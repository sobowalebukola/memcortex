package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type EmbeddingClient struct {
	BaseURL string
	Model   string
}

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

func NewEmbeddingClient(model string) *EmbeddingClient {

	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment")
	}

	ollamaAddr := os.Getenv("OLLAMA_ADDR")
	if ollamaAddr == "" {
		ollamaAddr = "11434"
	}
	return &EmbeddingClient{
		BaseURL: "http://ollama:" + ollamaAddr,
		Model:   model,
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float64, error) {
	reqBody := embeddingRequest{
		Model:  c.Model,
		Prompt: text,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/embeddings", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed embeddingResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("invalid embedding json: %w", err)
	}

	return parsed.Embedding, nil
}
