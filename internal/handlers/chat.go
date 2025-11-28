package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/sobowalebukola/memcortex/internal/memory"
	"github.com/sobowalebukola/memcortex/internal/middleware"
)

type ChatReq struct {
	Message string `json:"message"`
}

type ChatResp struct {
	Response     string          `json:"response"`
	MemoriesUsed []memory.Memory `json:"memories_used"`
}

type ChatHandler struct {
	Manager *memory.Manager
}

func NewChatHandler(m *memory.Manager) *ChatHandler { return &ChatHandler{Manager: m} }

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	var req ChatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	memIf := r.Context().Value(middleware.MemoriesCtxKey)
	memories := []memory.Memory{}
	if memIf != nil {
		if ms, ok := memIf.([]memory.Memory); ok {
			memories = ms
		}
	}

	response := "LLM ECHO: " + req.Message

	if err := h.Manager.Save(r.Context(), userID, req.Message); err != nil {
		log.Printf("Failed to save message for user %s: %v", userID, err)
		http.Error(w, "failed to save message", http.StatusInternalServerError)
		return
	}

	log.Printf("Saved message for user %s: %s", userID, req.Message)
	out := ChatResp{
		Response:     response,
		MemoriesUsed: memories,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
