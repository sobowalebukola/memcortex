package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
)

type Memory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Embedding []float32 `json:"embedding"`
}

type Store struct {
	Client *weaviate.Client
	Class  string
	Dim    int
}

type SearchResult struct {
	ID         string
	Properties map[string]interface{}
}

// ---------------------------
// Initialize Weaviate store
// ---------------------------
func NewStore(class string, dim int) (*Store, error) {
	client, err := weaviate.NewClient(weaviate.Config{
		Host:   "weaviate:8080",
		Scheme: "http",
	})

	if err != nil {
		return nil, err
	}

	return &Store{
		Client: client,
		Class:  class,
		Dim:    dim,
	}, nil
}

// ---------------------------
// Save Memory
// ---------------------------
func (s *Store) Save(ctx context.Context, userID, text string, embedding []float32) (string, error) {
	if len(embedding) != s.Dim {
		return "", fmt.Errorf("embedding dimension mismatch")
	}

	id := uuid.New().String()

	data := map[string]any{
		"user_id":   userID,
		"text":      text,
		"timestamp": time.Now().Unix(),
		"embedding": embedding,
	}

	_, err := s.Client.Data().
		Creator().
		WithClassName(s.Class).
		WithID(id).
		WithProperties(data).
		WithVector(embedding).
		Do(ctx)

	if err != nil {
		return "", err
	}

	return id, nil
}

// ---------------------------
// Vector Search in Weaviate
// ---------------------------
func (s *Store) Search(ctx context.Context, queryEmbedding []float64, userID string, k int) ([]Memory, error) {
	if len(queryEmbedding) != s.Dim {
		return nil, fmt.Errorf("embedding dim mismatch: expected %d, got %d", s.Dim, len(queryEmbedding))
	}

	vec := float64ToFloat32Slice(queryEmbedding)

	nearVector := s.Client.GraphQL().NearVectorArgBuilder().
		WithVector(vec).WithDistance(0.7)

	where := filters.Where().
		WithPath([]string{"user_id"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	query := s.Client.GraphQL().Get().
		WithClassName("Memory_idx").
		WithWhere(where).
		WithNearVector(nearVector).
		WithLimit(k).
		WithFields(
			graphql.Field{Name: "text"},
			graphql.Field{Name: "timestamp"},
			graphql.Field{Name: "user_id"},
			graphql.Field{Name: "embedding"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "distance"},
				},
			},
		)
	resp, err := query.Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("graphql error: %w", err)
	}

	if resp.Errors != nil {
		return nil, fmt.Errorf("weaviate error: %v", resp.Errors)
	}

	getNode, ok := resp.Data["Get"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	raw, ok := getNode["Memory_idx"].([]interface{})
	if !ok {
		return nil, nil
	}

	results := make([]Memory, 0, len(raw))

	for _, item := range raw {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		mem := Memory{}

		if v, ok := obj["text"].(string); ok {
			mem.Text = v
		}

		if v, ok := obj["user_id"].(string); ok {
			mem.UserID = v
		}

		if ts, ok := obj["timestamp"].(float64); ok {
			mem.Timestamp = time.Unix(int64(ts), 0)
		}

		if ts, ok := obj["embedding"].([]interface{}); ok {
			emb := make([]float32, len(ts))
			for i, val := range ts {
				if f, ok := val.(float64); ok {
					emb[i] = float32(f)
				}
			}
			mem.Embedding = emb
		}

		if add, ok := obj["_additional"].(map[string]interface{}); ok {
			if id, ok := add["id"].(string); ok {
				mem.ID = id
			}
		}

		results = append(results, mem)
	}
	return results, nil
}

func float64ToFloat32Slice(f []float64) []float32 {
	out := make([]float32, len(f))
	for i, v := range f {
		out[i] = float32(v)
	}
	return out
}
