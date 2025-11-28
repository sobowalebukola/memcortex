package main

import (
	// "context"

	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
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
	dim, _ := strconv.Atoi(dimStr)

	store, err := memory.NewStore("memory_idx", dim)
	if err != nil {
		log.Fatalf("failed to create memory store: %v", err)
	}
	emb := ollama.NewEmbeddingClient("nomic-embed-text:latest")

	m := memory.NewManager(store, emb)

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
