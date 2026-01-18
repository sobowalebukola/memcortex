package memory

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
	"github.com/sobowalebukola/memcortex/internal/summarizer"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
)

type Manager struct {
	Store          *Store
	Embedder       *ollama.EmbeddingClient
	TopK           int
	WeaviateClient *weaviate.Client
	summarizer     *summarizer.Summarizer
}

type MemoryPrompt struct {
	Text  string `json:"text"`
	Added string `json:"added"`
}

func NewManager(store *Store, emb *ollama.EmbeddingClient) *Manager {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment")
	}

	topKStr := os.Getenv("TOP_K_MEMORIES")
	if topKStr == "" {
		topKStr = "10"
	}
	topK, err := strconv.Atoi(topKStr)
	if err != nil {
		topK = 10
	}

	return &Manager{
		Store:      store,
		Embedder:   emb,
		TopK:       topK,
		summarizer: summarizer.NewSummarizer(),
	}
}

func (m *Manager) Retrieve(ctx context.Context, userID, query string) ([]Memory, error) {
	q := strings.TrimSpace(query)
	emb, err := m.Embedder.Embed(ctx, q)
	if err != nil {
		return nil, err
	}

	emb32 := make([]float32, len(emb))
	for i, v := range emb {
		emb32[i] = float32(v)
	}

	return m.Store.Search(ctx, emb32, userID, m.TopK)
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
		result = append(result, MemoryPrompt{
			Text:  mem.Content,
			Added: mem.Timestamp,
		})

		if i >= 20 {
			break
		}
	}

	return result
}



func (m *Manager) CheckAndSummarize(ctx context.Context, userID string) error {
	if !m.isAutoSummaryEnabled() {
		return nil
	}

	count, err := m.Store.GetMemoryCount(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get memory count: %w", err)
	}

	threshold := summarizer.GetSummaryThreshold()
	if count < threshold {
		return nil 
	}

	log.Printf("Memory count (%d) exceeded threshold (%d) for user %s, triggering summarization",
		count, threshold, userID)

	return m.SummarizeUserMemories(ctx, userID)
}

func (m *Manager) SummarizeUserMemories(ctx context.Context, userID string) error {
	batchSize := summarizer.GetSummaryBatchSize()
	maxAge := summarizer.GetSummaryMaxAgeDays()

	memories, err := m.Store.GetOldMemories(ctx, userID, maxAge, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get old memories: %w", err)
	}

	if len(memories) == 0 {
		log.Printf("No memories to summarize for user %s", userID)
		return nil
	}

	contents := make([]string, len(memories))
	ids := make([]string, len(memories))
	for i, mem := range memories {
		contents[i] = mem.Content
		ids[i] = mem.ID
	}

	summary, err := m.summarizer.SummarizeMemories(ctx, contents, userID)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	summaryEmb, err := m.Embedder.Embed(ctx, summary)
	if err != nil {
		return fmt.Errorf("failed to embed summary: %w", err)
	}

	emb32 := make([]float32, len(summaryEmb))
	for i, v := range summaryEmb {
		emb32[i] = float32(v)
	}

	log.Printf("Generated summary for %d memories (user %s)", len(memories), userID)

	if err := m.Store.SaveSummary(ctx, summary, userID, ids, emb32); err != nil {
		return fmt.Errorf("failed to save summary: %w", err)
	}

	if err := m.Store.DeleteMemories(ctx, ids); err != nil {
		return fmt.Errorf("failed to delete original memories: %w", err)
	}

	log.Printf("Successfully summarized %d memories for user %s", len(memories), userID)
	return nil
}

func (m *Manager) isAutoSummaryEnabled() bool {
	enabled := os.Getenv("ENABLE_AUTO_SUMMARY")
	if enabled == "" {
		return true 
	}
	val, _ := strconv.ParseBool(enabled)
	return val
}
func (m *Manager) GetUserBio(ctx context.Context, userID string) (string, error) {

    return m.Store.GetUserBio(ctx, userID)
}


func (m *Manager) EnsureUserExists(ctx context.Context, userID string) error {

    return m.Store.EnsureUser(ctx, userID)
}