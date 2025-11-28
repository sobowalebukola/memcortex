package memory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	weaviate "github.com/weaviate/weaviate-go-client/v4/weaviate"

	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
)

type Manager struct {
	Store          *Store
	Embedder       *ollama.EmbeddingClient
	TopK           int
	WeaviateClient *weaviate.Client
}

func NewManager(store *Store, emb *ollama.EmbeddingClient) *Manager {
	return &Manager{Store: store, Embedder: emb, TopK: 6}
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

func FormatMemoryPrompt(memories []Memory) string {
	if len(memories) == 0 {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("MEMORIES:\n")
	for i, mem := range memories {
		ts := mem.Timestamp.Format(time.RFC3339)
		sb.WriteString("- ")
		sb.WriteString(mem.Text)
		sb.WriteString(" (added: ")
		sb.WriteString(ts)
		sb.WriteString(")\n")
		if i >= 20 {
			break
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

type MemoryManager struct {
	queue *EmbeddingQueue
	store *Store
}

func NewMemoryManager(queue *EmbeddingQueue, store *Store) *MemoryManager {
	return &MemoryManager{
		queue: queue,
		store: store,
	}
}

func (m *MemoryManager) SaveMemory(ctx context.Context, userID, text string) error {
	go func() {
		embedding, err := m.queue.Enqueue(userID, text)
		if err != nil {
			fmt.Println("Error generating embedding:", err)
			return
		}

		embedding32 := make([]float32, len(embedding))

		for i, v := range embedding {
			embedding32[i] = float32(v)
		}
		id, err := m.store.Save(ctx, userID, text, embedding32)
		if err != nil {
			fmt.Println("Error saving memory:", err)
			return
		}

		log.Printf("Memory saved with ID: %s", id)
	}()
	return nil
}
