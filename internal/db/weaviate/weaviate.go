package weaviate

import (
    "context"
    "fmt"
	"log"
    "time"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
    "github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
    "github.com/weaviate/weaviate-go-client/v4/weaviate/filters"
	"github.com/weaviate/weaviate/entities/models"
)

type WeaviateClient struct {
	client *weaviate.Client
}

func NewWeaviateClient(client *weaviate.Client) *WeaviateClient {
	return &WeaviateClient{client: client}
}

func (w *WeaviateClient) AddMemory(ctx context.Context, content string, userID string) error {

    if userID == "" {
        userID = fmt.Sprintf("user_%d", time.Now().Unix())
        log.Printf("Warning: Empty userID provided. Falling back to generated ID: %s", userID)
    }

    properties := map[string]interface{}{
        "content":    content,
        "userId":     userID,   
        "timestamp":  time.Now().Format(time.RFC3339),
        "memoryType": "raw",
        "isSummary":  false,
    }


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


func (w *WeaviateClient) GetUserBio(ctx context.Context, userID string) (string, error) {

    if userID == "" {
        return "", nil 
    }


	where := filters.Where().
		WithPath([]string{"userId"}).
		WithOperator(filters.Equal).
		WithValueString(userID)

	result, err := w.client.GraphQL().Get().
		WithClassName("User").
		WithFields(graphql.Field{Name: "bio"}).
		WithWhere(where).
		Do(ctx)
    if err != nil {
        return "", fmt.Errorf("weaviate query failed: %w", err)
    }

    // Safe navigation of the nested map
    if result.Data["Get"] == nil {
        return "", nil
    }

    data, ok := result.Data["Get"].(map[string]interface{})["User"].([]interface{})
    if !ok || len(data) == 0 {
        return "", nil 
    }

    user, ok := data[0].(map[string]interface{})
    if !ok {
        return "", nil
    }

    bio, _ := user["bio"].(string)
    return bio, nil
}

func EnsureSchema(client *weaviate.Client) {
	ctx := context.Background()


	ensureClass(client, ctx, &models.Class{
		Class:      "Memory_idx",
		Vectorizer: "none",
		Properties: []*models.Property{
			{Name: "content", DataType: []string{"text"}},
			{Name: "userId", DataType: []string{"string"}},
			{Name: "timestamp", DataType: []string{"date"}},
			{Name: "memoryType", DataType: []string{"string"}},
			{Name: "isSummary", DataType: []string{"boolean"}},
			{Name: "originalIds", DataType: []string{"text[]"}},
		},
	})


	ensureClass(client, ctx, &models.Class{
		Class:      "User",
		Vectorizer: "none",
		Properties: []*models.Property{
			{Name: "username", DataType: []string{"string"}},
			{Name: "userId", DataType: []string{"string"}},
			{Name: "bio", DataType: []string{"text"}},
			{Name: "createdAt", DataType: []string{"date"}},
		},
	})
}

func ensureClass(client *weaviate.Client, ctx context.Context, classObj *models.Class) {
	exists, err := client.Schema().ClassExistenceChecker().WithClassName(classObj.Class).Do(ctx)
	if err != nil {
		log.Printf("Error checking schema for %s: %v", classObj.Class, err)
		return
	}
	if !exists {
		err := client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
		if err != nil {
			log.Fatalf("Failed to create class %s: %v", classObj.Class, err)
		}
	}
}

func (w *WeaviateClient) EnsureUser(ctx context.Context, userID string) error {

	where := filters.Where().
        WithPath([]string{"userId"}).
        WithOperator(filters.Equal).
        WithValueString(userID)

    result, err := w.client.GraphQL().Get().
        WithClassName("User").
        WithFields(graphql.Field{Name: "userId"}).
        WithWhere(where). 
        Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	data := result.Data["Get"].(map[string]interface{})["User"].([]interface{})
	

	if len(data) > 0 {
		return nil 
	}

	
	log.Printf("New user detected: %s. Performing registration...", userID)
	_, err = w.client.Data().Creator().
		WithClassName("User").
		WithProperties(map[string]interface{}{
			"userId":    userID,
			"username":  "User_" + userID, 
			"bio":       "New MemCortex user",
			"createdAt": time.Now().Format(time.RFC3339),
		}).Do(ctx)

	return err
}