package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/sobowalebukola/memcortex/internal/memory"
)

type MemoryMiddleware struct {
	Manager *memory.Manager
}

type ChatRequest struct {
	Memories []memory.MemoryPrompt `json:"memories"`
	Message  string                `json:"message"`
}

type ctxKey string

const MemoriesCtxKey ctxKey = "retrieved_memories"

func (mw *MemoryMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")

		if userID == "" {
			http.Error(w, "missing X-User-ID header", http.StatusBadRequest)
			return
		}

		var in ChatRequest
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		if len(bodyBytes) == 0 {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(bodyBytes, &in); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		memories, _ := mw.Manager.Retrieve(r.Context(), userID, in.Message)

		ctx := context.WithValue(r.Context(), MemoriesCtxKey, memories)

		memPrompts := memory.FormatMemoryPrompt(memories)

		out := ChatRequest{
			Memories: memPrompts,
			Message:  in.Message,
		}

		outB, _ := json.Marshal(out)

		r = r.WithContext(ctx)
		r.Body = io.NopCloser(bytes.NewReader(outB))
		r.ContentLength = int64(len(outB))
		r.Header.Set("Content-Type", "application/json")

		next.ServeHTTP(w, r)
	})
}
