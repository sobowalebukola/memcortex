package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate/entities/models"

	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
	"github.com/sobowalebukola/memcortex/internal/handlers"
	"github.com/sobowalebukola/memcortex/internal/memory"
	"github.com/sobowalebukola/memcortex/internal/middleware"
)

func main() {
	// 1. Load Environment Variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment")
	}

	// 2. Initialize Weaviate Client
	// We use the hostname 'weaviate' because that is the service name in docker-compose
	cfg := weaviate.Config{
		Host:   os.Getenv("WEAVIATE_HOST"),
		Scheme: os.Getenv("WEAVIATE_SCHEME"),
	}
	if cfg.Host == "" {
		cfg.Host = "weaviate:8080" // Default for Docker
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}

	wClient, err := weaviate.NewClient(cfg)
	if err != nil {
		log.Fatalf("failed to create weaviate client: %v", err)
	}

	// 3. Initialize/Check Schema (Create the table if missing)
	ensureSchema(wClient)

	// 4. Initialize Embedder (Ollama)
	emb := ollama.NewEmbeddingClient(os.Getenv("EMBEDDING_MODEL"))

	// 5. Initialize Memory Store (Connecting to Weaviate)
	store := memory.NewWeaviateStore(wClient, "Memory_idx")

	// 6. Initialize Manager
	m := memory.NewManager(store, emb)

	log.Println("MemCortex initialized with Weaviate successfully!")

	// 7. Setup Handlers & Middleware
	chatHandler := handlers.NewChatHandler(m)
	mw := &middleware.MemoryMiddleware{Manager: m}

	mux := http.NewServeMux()
	
	// Endpoint 1: Chat (with memory injection)
	mux.Handle("/chat", mw.Handler(chatHandler))

	// Endpoint 2: Manual Summarization
	mux.HandleFunc("/api/summarize", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, "Missing X-User-ID header", http.StatusBadRequest)
			return
		}

		// Trigger summarization manually
		if err := m.SummarizeUserMemories(r.Context(), userID); err != nil {
			http.Error(w, fmt.Sprintf("Summarization failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "summarization completed"})
	})
	// --- ADDED ENDPOINT 3: User Registration ---
    mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req struct {
            Username string `json:"username"`
            Bio      string `json:"bio"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid body", http.StatusBadRequest)
            return
        }

        // Generate ID
        newID := fmt.Sprintf("u-%d", time.Now().Unix())

        // Save to Weaviate
        _, err := wClient.Data().Creator().
            WithClassName("User").
            WithProperties(map[string]interface{}{
                "username":  req.Username,
                "userId":    newID,
                "bio":       req.Bio,
                "createdAt": time.Now().Format(time.RFC3339),
            }).Do(r.Context())

        if err != nil {
            log.Printf("Error saving user: %v", err)
            http.Error(w, "Failed to register", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "user_id": newID,
            "status":  "Registration successful!",
        })
    })

	// 8. Start Server
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ensureSchema checks if the 'Memory_idx' class exists and creates it if missing.
func ensureSchema(client *weaviate.Client) {
	className := "Memory_idx"
	ctx := context.Background()

	// Check existence
	exists, err := client.Schema().ClassExistenceChecker().
		WithClassName(className).
		Do(ctx)
	if err != nil {
		log.Printf("Warning: Failed to check schema existence: %v (Weaviate might still be starting)", err)
		return
	}

	if exists {
		log.Println("Schema 'Memory_idx' already exists.")
		return
	}

	log.Println("Creating schema for 'Memory_idx'...")

	classObj := &models.Class{
		Class:      className,
		Vectorizer: "none", // We handle vectorization manually with Ollama
		Properties: []*models.Property{
			{Name: "content", DataType: []string{"text"}},
			{Name: "userId", DataType: []string{"string"}}, // string allows exact filtering
			{Name: "timestamp", DataType: []string{"date"}},
			{Name: "memoryType", DataType: []string{"string"}},
			{Name: "isSummary", DataType: []string{"boolean"}},
			{Name: "originalIds", DataType: []string{"text[]"}},
		},
	}

	err = client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
	if err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}
	log.Println("Schema created successfully!")
	// --- 2. User Schema ---
    userClassName := "User"
    userExists, _ := client.Schema().ClassExistenceChecker().WithClassName(userClassName).Do(ctx)

    if !userExists {
        log.Println("Creating schema for 'User'...")
        userClass := &models.Class{
            Class:      userClassName,
            Vectorizer: "none",
            Properties: []*models.Property{
                {Name: "username", DataType: []string{"string"}},
                {Name: "userId", DataType: []string{"string"}}, // This is the X-User-ID
                {Name: "bio", DataType: []string{"text"}},      // This holds the "software project" context
                {Name: "createdAt", DataType: []string{"date"}},
            },
        }
        client.Schema().ClassCreator().WithClass(userClass).Do(ctx)
    }

}
