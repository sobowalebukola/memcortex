package db

import (
    "context"
    "fmt"
    "time"
    "github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
    "github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
)

func (w *WeaviateClient) AddMemory(ctx context.Context, content string, userID string) error {
    // FORCE a value if userID is empty to ensure summarization works
    if userID == "" {
        userID = "emmanuel123"
    }

    properties := map[string]interface{}{
        "content":    content,
        "userId":     userID,   
        "timestamp":  time.Now().Format(time.RFC3339),
        "memoryType": "raw",
        "isSummary":  false,
    }

    // This log will appear in your terminal to confirm the save
    fmt.Printf("--- [DB] SAVING MEMORY FOR USER: %s ---\n", userID)

    _, err := w.client.Data().Creator().
        WithClassName("Memory_idx").
        WithProperties(properties).
        Do(ctx)
    
    if err != nil {
        return fmt.Errorf("failed to create memory: %w", err)
    }
    
    return nil
}

// RegisterUser adds a new user profile to Weaviate
func (w *WeaviateClient) RegisterUser(ctx context.Context, username string, bio string) (string, error) {
    userID := fmt.Sprintf("u-%d", time.Now().Unix())

    properties := map[string]interface{}{
        "userId":    userID,
        "username":  username,
        "bio":       bio,
        "createdAt": time.Now().Format(time.RFC3339),
    }

    _, err := w.client.Data().Creator().
        WithClassName("User").
        WithProperties(properties).
        Do(ctx)

    if err != nil {
        return "", fmt.Errorf("failed to register user: %w", err)
    }

    return userID, nil
}

// GetUserBio retrieves the specific project context for a user
func (w *WeaviateClient) GetUserBio(ctx context.Context, userID string) (string, error) {
    // If we're using the default user for testing
    if userID == "" {
        userID = "emmanuel123"
    }

    // Use GQL to find the user by their ID
    result, err := w.client.GraphQL().Get().
        WithClassName("User").
        WithFields(graphql.Field{Name: "bio"}).
        WithWhere(&filters.WhereBuilder{
            Path:     []string{"userId"},
            Operator: filters.Equal,
            ValueString: userID,
        }).
        Do(ctx)

    if err != nil {
        return "", err
    }

    // Handle extraction logic (checking for empty results)
    data := result.Data["Get"].(map[string]interface{})["User"].([]interface{})
    if len(data) == 0 {
        return "Software development project called MemCortex.", nil // Default fallback
    }

    user := data[0].(map[string]interface{})
    return user["bio"].(string), nil
}

func (s *WeaviateStore) GetUserBio(ctx context.Context, userID string) (string, error) {
	result, err := s.client.GraphQL().Get().
		WithClassName("User").
		WithFields(graphql.Field{Name: "bio"}).
		WithWhere(&filters.WhereBuilder{
			Path:        []string{"userId"},
			Operator:    filters.Equal,
			ValueString: userID,
		}).
		Do(ctx)

	if err != nil {
		return "", err
	}

	// Navigate the nested Weaviate response map
	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}

	users, ok := data["User"].([]interface{})
	if !ok || len(users) == 0 {
		return "", fmt.Errorf("user not found")
	}

	userMap := users[0].(map[string]interface{})
	bio, _ := userMap["bio"].(string)

	return bio, nil
}