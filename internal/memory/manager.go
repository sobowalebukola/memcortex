package memory

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
)

type Manager struct {
	Store    *Store
	Embedder *ollama.EmbeddingClient
	TopK     int
}

type MemoryPrompt struct {
	Text  string `json:"text"`
	Added string `json:"added"`
}

func NewManager(store *Store, emb *ollama.EmbeddingClient) *Manager {
	topKStr := os.Getenv("TOP_K_MEMORIES")
	if topKStr == "" {
		topKStr = "10"
	}
	topK, err := strconv.Atoi(topKStr)
	if err != nil {
		topK = 10
	}
	return &Manager{Store: store, Embedder: emb, TopK: topK}
}

func (m *Manager) Retrieve(ctx context.Context, userID, query string) ([]Memory, error) {
	q := strings.TrimSpace(query)
	emb, err := m.Embedder.Embed(ctx, q)
	if err != nil {
		return nil, err
	}
	return m.Store.Search(ctx, emb, userID, m.TopK)
}

func (m *Manager) SaveAsync(ctx context.Context, userID, text string) {
	go func() {
		_ = m.Save(ctx, userID, text)
	}()
}

func (m *Manager) Save(ctx context.Context, userID, text string) error {
	emb, err := m.Embedder.Embed(ctx, text)

	if err != nil {
		return err
	}
	emb32 := make([]float32, len(emb))
	for i, v := range emb {
		emb32[i] = float32(v)
	}
	_, err = m.Store.Save(ctx, userID, text, emb32)
	return err
}

func FormatMemoryPrompt(memories []Memory) []MemoryPrompt {
	if len(memories) == 0 {
		return []MemoryPrompt{}
	}

	result := make([]MemoryPrompt, 0, len(memories))

	for i, mem := range memories {
		ts := mem.Timestamp.Format(time.RFC3339)

		result = append(result, MemoryPrompt{
			Text:  mem.Text,
			Added: ts,
		})

		if i >= 20 {
			break
		}
	}

	return result
}
