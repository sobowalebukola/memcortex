package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sobowalebukola/memcortex/internal/memory"
)

type ChatRequest struct {
	Message string `json:"message"`
}

// Simplified Response for a cleaner human-readable output
type ChatResponse struct {
	Response string   `json:"new_message"`
	Memories []string `json:"related_memories"`
}

type ChatHandler struct {
	Manager *memory.Manager
}

func NewChatHandler(m *memory.Manager) *ChatHandler {
	return &ChatHandler{Manager: m}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
if userID == "" {
        userID = fmt.Sprintf("user_%d", time.Now().Unix())
        log.Printf("No X-User-ID header found. Assigning dynamic ID: %s", userID)
    }
// --- NEW: JUST-IN-TIME REGISTRATION ---
    // This addresses @sobowalebukola's comment: "write a logic that adds a user (register)"
    ctx := r.Context()
    if err := h.Manager.EnsureUserExists(ctx, userID); err != nil {
        log.Printf("Warning: JIT Registration failed for %s: %v", userID, err)
        // We continue because the user might already exist, but logging is important
    }

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	// 1. Retrieve Memories
	memories, err := h.Manager.Retrieve(ctx, userID, req.Message)
	if err != nil {
		log.Printf("Failed to retrieve memories: %v", err)
		memories = []memory.Memory{}
	}
	// --- NEW: FETCH USER BIO ---
    // We fetch the bio to provide permanent project context
    // 1. Fetch the bio (Keep this as is)
userBio, err := h.Manager.GetUserBio(ctx, userID)
if err != nil {
    log.Printf("Could not fetch user bio: %v", err)
    userBio = "A software project called MemCortex." 
}

// 2. FIXED: Change 'UserBio' (capital U) to 'userBio' (lowercase u) 
// and make sure it is the 4th argument.
aiResponse, err := h.callLLM(ctx, req.Message, memories, userBio) 
if err != nil {
    log.Printf("LLM generation failed: %v", err)
    http.Error(w, "Failed to generate AI response", http.StatusInternalServerError)
    return
}

	// 3. Background Processing (Decoupled from Request)
	go func(uID, msg, aiResp string) {
		bgCtx := context.Background()
		_ = h.Manager.Save(bgCtx, uID, msg)
		_ = h.Manager.Save(bgCtx, uID, "AI: "+aiResp)
		_ = h.Manager.CheckAndSummarize(bgCtx, uID)
	}(userID, req.Message, aiResponse)

	// 4. CLEAN OUTPUT: Extract only the content strings
	cleanMemories := make([]string, 0, len(memories))
	for _, m := range memories {
		cleanMemories = append(cleanMemories, m.Content)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{
		Response: aiResponse,
		Memories: cleanMemories,
	})
}

// ---------------------------------------------------------
// Helper: Talk to Ollama
// ---------------------------------------------------------

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func (h *ChatHandler) callLLM(ctx context.Context, userMessage string, memories []memory.Memory, userBio string) (string, error) {
	var contextBuilder strings.Builder
	for _, m := range memories {
		contextBuilder.WriteString(fmt.Sprintf("- %s\n", m.Content))
	}

	systemPrompt := fmt.Sprintf("You are the MemCortex Assistant. "+
    "Context: %s. "+
    "Rules: "+
    "1. Use the Context above to answer. "+
    "2. If unsure, say 'I don't have that in my memory.' "+
    "3. Be concise (under 3 sentences). ", userBio)

	fullPrompt := fmt.Sprintf("%s\n\nContext:\n%s\n\nUser: %s", systemPrompt, contextBuilder.String(), userMessage)

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "deepseek-r1:1.5b"
	}

	reqBody := ollamaRequest{
		Model:  model,
		Prompt: fullPrompt,
		Stream: false,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post("http://ollama:11434/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result ollamaResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Response, nil
}