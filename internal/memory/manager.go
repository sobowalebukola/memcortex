package memory

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sobowalebukola/memcortex/internal/classifier"
	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
)

type Manager struct {
	Store              *Store
	Embedder           *ollama.EmbeddingClient
	Classifier         *classifier.Classifier
	TopK               int
	DuplicateThreshold float64
}

type MemoryPrompt struct {
	Text  string `json:"text"`
	Added string `json:"added"`
}

func NewManager(store *Store, emb *ollama.EmbeddingClient, cls *classifier.Classifier) *Manager {
	topKStr := os.Getenv("TOP_K_MEMORIES")
	if topKStr == "" {
		topKStr = "10"
	}
	topK, err := strconv.Atoi(topKStr)
	if err != nil {
		topK = 10
	}
	dupThresh := 0.05
	if v, err := strconv.ParseFloat(os.Getenv("DUPLICATE_DISTANCE"), 64); err == nil && v > 0 {
		dupThresh = v
	}

	return &Manager{
		Store:              store,
		Embedder:           emb,
		Classifier:         cls,
		TopK:               topK,
		DuplicateThreshold: dupThresh,
	}
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
		_, _ = m.Save(ctx, userID, text)
	}()
}

// Save classifies the message and only persists it if it's meaningful.
// Returns (true, nil) if saved, (false, nil) if skipped, or (false, err) on failure.
func (m *Manager) Save(ctx context.Context, userID, text string) (bool, error) {
	result := m.Classifier.Classify(text)
	if !result.ShouldSave {
		log.Printf("Skipped low-value message for user %s (type=%s, weight=%.2f): %s",
			userID, result.Type, result.Weight, truncate(text, 60))
		return false, nil
	}

	log.Printf("Classified message for user %s as %s (weight=%.2f, ttl=%dd)",
		userID, result.Type, result.Weight, result.TTLDays)

	emb, err := m.Embedder.Embed(ctx, text)
	if err != nil {
		return false, err
	}

	// Check for near-duplicate memories before saving.
	if m.DuplicateThreshold > 0 {
		existing, searchErr := m.Store.Search(ctx, emb, userID, 1)
		if searchErr == nil && len(existing) > 0 && existing[0].Distance < m.DuplicateThreshold {
			log.Printf("Skipped duplicate memory for user %s (distance=%.4f): %s",
				userID, existing[0].Distance, truncate(text, 60))
			return false, nil
		}
	}

	emb32 := make([]float32, len(emb))
	for i, v := range emb {
		emb32[i] = float32(v)
	}
	_, err = m.Store.Save(ctx, userID, text, emb32, string(result.Type), result.Weight, result.TTLDays)
	if err != nil {
		return false, err
	}
	return true, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
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
