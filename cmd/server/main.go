package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sobowalebukola/memcortex/internal/classifier"
	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
	"github.com/sobowalebukola/memcortex/internal/handlers"
	"github.com/sobowalebukola/memcortex/internal/memory"
	"github.com/sobowalebukola/memcortex/internal/middleware"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment")
	}

	dimStr := os.Getenv("EMBEDDING_DIM")
	if dimStr == "" {
		dimStr = "768"
	}
	dim, err := strconv.Atoi(dimStr)
	if err != nil {
		log.Fatalf("invalid EMBEDDING_DIM %q: %v", dimStr, err)
	}

	weaviateHost := os.Getenv("WEAVIATE_HOST")
	if weaviateHost == "" {
		weaviateHost = "weaviate:8080"
	}
	weaviateHost = strings.TrimPrefix(weaviateHost, "https://")
	weaviateHost = strings.TrimPrefix(weaviateHost, "http://")

	store, err := memory.NewStore("memory_idx", dim, weaviateHost)
	if err != nil {
		log.Fatalf("failed to create memory store: %v", err)
	}
	emb := ollama.NewEmbeddingClient(os.Getenv("EMBEDDING_MODEL"))
	cls := classifier.New()

	m := memory.NewManager(store, emb, cls)

	log.Printf("Event classifier enabled (min_weight=%.2f)", cls.MinWeight)

	log.Println("MemCortex initialized successfully!")

	chatHandler := handlers.NewChatHandler(m)
	mw := &middleware.MemoryMiddleware{Manager: m}

	mux := http.NewServeMux()
	mux.Handle("/chat", mw.Handler(chatHandler))

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
