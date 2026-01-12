package memory

import (
	"context"
	"fmt"
	"time"
	"log"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

type Memory struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Timestamp  string    `json:"timestamp"`
	UserID     string    `json:"userId"`
	MemoryType string    `json:"memoryType"`
}

type Store struct {
	Client *weaviate.Client
	Class  string
}

func NewWeaviateStore(client *weaviate.Client, class string) *Store {
	return &Store{
		Client: client,
		Class:  class,
	}
}

func (s *Store) Save(ctx context.Context, userID, text string, embedding []float32) (string, error) {
	id := uuid.New().String()
	if userID == "" {
		userID = fmt.Sprintf("user_%d", time.Now().Unix())
		log.Printf("Warning: Store received empty userID. Generated dynamic ID: %s", userID)
	}

	data := map[string]interface{}{
		"userId":      userID,
		"content":     text,
		"timestamp":   time.Now().Format(time.RFC3339),
		"memoryType":  "raw",
		"isSummary":   false,
	}

	_, err := s.Client.Data().
		Creator().
		WithClassName(s.Class).
		WithID(id).
		WithProperties(data).
		WithVector(embedding).
		Do(ctx)

	return id, err
}

func (s *Store) GetMemoryCount(ctx context.Context, userID string) (int, error) {
	fmt.Printf(">>> [DB] Counting memories for user: %s\n", userID)
	
	where := filters.Where().
		WithPath([]string{"userId"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	resp, err := s.Client.GraphQL().Aggregate().
		WithClassName(s.Class).
		WithWhere(where).
		WithFields(graphql.Field{
			Name: "meta",
			Fields: []graphql.Field{{Name: "count"}},
		}).
		Do(ctx)

	if err != nil { return 0, err }

	data, ok := resp.Data["Aggregate"].(map[string]interface{})
	if !ok { return 0, nil }
	
	classData, ok := data[s.Class].([]interface{})
	if !ok || len(classData) == 0 { return 0, nil }
	
	fields, ok := classData[0].(map[string]interface{})
	if !ok { return 0, nil }
	
	meta, ok := fields["meta"].(map[string]interface{})
	if !ok { return 0, nil }
	
	count, ok := meta["count"].(float64)
	if !ok { return 0, nil }

	fmt.Printf(">>> [DB] Database count for %s is: %d\n", userID, int(count))
	return int(count), nil 
}

func (s *Store) GetOldMemories(ctx context.Context, userID string, olderThanDays int, limit int) ([]Memory, error) {
	where := filters.Where().
		WithOperator(filters.And).
		WithOperands([]*filters.WhereBuilder{
			filters.Where().
				WithPath([]string{"userId"}).
				WithOperator(filters.Equal).
				WithValueString(userID),
			filters.Where().
				WithPath([]string{"isSummary"}).
				WithOperator(filters.Equal).
				WithValueBoolean(false),
		})

	fields := []graphql.Field{
		{Name: "content"},
		{Name: "timestamp"},
		{Name: "memoryType"},
		{Name: "_additional", Fields: []graphql.Field{{Name: "id"}}},
	}

	resp, err := s.Client.GraphQL().Get().
		WithClassName(s.Class).
		WithWhere(where).
		WithLimit(limit).
		WithFields(fields...).
		Do(ctx)

	if err != nil { return nil, err }
	return s.parseGraphQLResponse(resp)
}

func (s *Store) SaveSummary(ctx context.Context, summary string, userID string, originalIDs []string, embedding []float32) error {
	data := map[string]interface{}{
		"content":     summary,
		"userId":      userID,
		"timestamp":   time.Now().Format(time.RFC3339),
		"memoryType":  "summary",
		"isSummary":   true,
		"originalIds": originalIDs,
	}

	_, err := s.Client.Data().
		Creator().
		WithClassName(s.Class).
		WithProperties(data).
		WithVector(embedding).
		Do(ctx)

	return err
}

func (s *Store) DeleteMemories(ctx context.Context, ids []string) error {
	for _, id := range ids {
		_ = s.Client.Data().Deleter().WithClassName(s.Class).WithID(id).Do(ctx)
	}
	return nil
}

func (s *Store) parseGraphQLResponse(resp *models.GraphQLResponse) ([]Memory, error) {
	var memories []Memory

	data, ok := resp.Data["Get"].(map[string]interface{})
	if !ok { return nil, nil }

	objects, ok := data[s.Class].([]interface{})
	if !ok { return nil, nil }

	for _, obj := range objects {
		item, ok := obj.(map[string]interface{})
		if !ok { continue }

		mem := Memory{}
		if content, ok := item["content"].(string); ok {
			mem.Content = content
		}
		if ts, ok := item["timestamp"].(string); ok {
			mem.Timestamp = ts
		}
		if mt, ok := item["memoryType"].(string); ok {
			mem.MemoryType = mt
		}

		if additional, ok := item["_additional"].(map[string]interface{}); ok {
			if id, ok := additional["id"].(string); ok {
				mem.ID = id
			}
		}
		memories = append(memories, mem)
	}
	return memories, nil
}

func (s *Store) Search(ctx context.Context, queryEmbedding []float32, userID string, k int) ([]Memory, error) {
	// Re-added specific fields needed for Search
	fields := []graphql.Field{
		{Name: "content"},
		{Name: "timestamp"},
		{Name: "memoryType"},
		{Name: "_additional", Fields: []graphql.Field{
			{Name: "id"},
			{Name: "distance"},
		}},
	}

	where := filters.Where().
		WithPath([]string{"userId"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	nearVector := s.Client.GraphQL().NearVectorArgBuilder().
		WithVector(queryEmbedding)

	resp, err := s.Client.GraphQL().Get().
		WithClassName(s.Class).
		WithWhere(where).
		WithNearVector(nearVector).
		WithLimit(k).
		WithFields(fields...).
		Do(ctx)

	if err != nil { return nil, err }
	return s.parseGraphQLResponse(resp)
}

// GetUserBio fetches the permanent project context for a specific user from the User class.
// GetUserBio queries the 'User' class in Weaviate for the project bio.
func (s *Store) GetUserBio(ctx context.Context, userID string) (string, error) {
	// Use the Builder Pattern instead of struct literals
	where := filters.Where().
		WithPath([]string{"userId"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	result, err := s.Client.GraphQL().Get().
		WithClassName("User") .
		WithFields(graphql.Field{Name: "bio"}).
		WithWhere(where). // Pass the builder here
		Do(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to fetch user bio: %w", err)
	}

	// Navigate the response map
	if result.Data == nil || result.Data["Get"] == nil {
		return "", fmt.Errorf("no data found in Weaviate")
	}

	getMap := result.Data["Get"].(map[string]interface{})
	users, ok := getMap["User"].([]interface{})
	
	// Fallback if the user isn't in the registry yet
	if !ok || len(users) == 0 {
		return "A software project called MemCortex focusing on long-term AI memory.", nil
	}

	userFields := users[0].(map[string]interface{})
	bio, _ := userFields["bio"].(string)

	return bio, nil
}
// internal/memory/store.go

// EnsureUser checks if a user exists in the 'User' class. If not, it creates them.
func (s *Store) EnsureUser(ctx context.Context, userID string) error {
	// 1. Build the filter using the Builder Pattern
	where := filters.Where().
		WithPath([]string{"userId"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	// 2. Query the User class
	result, err := s.Client.GraphQL().Get().
		WithClassName("User").
		WithFields(graphql.Field{Name: "userId"}).
		WithWhere(where).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	// 3. Check if we found any results
	if result.Data != nil && result.Data["Get"] != nil {
		getMap := result.Data["Get"].(map[string]interface{})
		users, ok := getMap["User"].([]interface{})
		if ok && len(users) > 0 {
			// User already exists, nothing to do
			return nil
		}
	}

	// 4. User doesn't exist, create them (JIT Registration)
	log.Printf(">>> [DB] New user detected: %s. Performing JIT registration...", userID)
	
	properties := map[string]interface{}{
		"userId":   userID,
		"username": "User_" + userID,
		"bio":      "A new user of the MemCortex system.",
	}

	_, err = s.Client.Data().Creator().
		WithClassName("User").
		WithProperties(properties).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to create new user: %w", err)
	}

	return nil
}