package memory

import (
	"fmt"
	"sync"

	"context"
	"strings"
	"time"

	ollama "github.com/sobowalebukola/memcortex/internal/embedder"
)

type EmbeddingJob struct {
	UserID string
	Text   string
	Result chan []float64
	Err    chan error
}

type EmbeddingQueue struct {
	jobs     chan EmbeddingJob
	embedder *ollama.EmbeddingClient
	wg       sync.WaitGroup
}

func NewEmbeddingQueue(embedder *ollama.EmbeddingClient, workerCount int) *EmbeddingQueue {
	q := &EmbeddingQueue{
		jobs:     make(chan EmbeddingJob, 100),
		embedder: embedder,
	}
	for i := 0; i < workerCount; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

func (q *EmbeddingQueue) worker() {
	defer q.wg.Done()
	for job := range q.jobs {
		emb, err := embedWithRetry(q.embedder, job.Text)
		if err != nil {
			fmt.Println("Embedding failed, using mock embedding:", err)
			emb = mockEmbedding(job.Text)
			err = nil
		}
		job.Result <- emb
		job.Err <- err
	}
}

func (q *EmbeddingQueue) Enqueue(userID, text string) ([]float64, error) {
	job := EmbeddingJob{
		UserID: userID,
		Text:   text,
		Result: make(chan []float64, 1),
		Err:    make(chan error, 1),
	}
	q.jobs <- job

	return <-job.Result, <-job.Err
}

func (q *EmbeddingQueue) Close() {
	close(q.jobs)
	q.wg.Wait()
}

func embedWithRetry(embedder *ollama.EmbeddingClient, text string) ([]float64, error) {
	maxRetries := 5
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		embedding, err := embedder.Embed(context.Background(), text)
		if err == nil {
			return embedding, nil
		}
		if strings.Contains(err.Error(), "429") {
			wait := time.Duration((i+1)*2) * time.Second
			time.Sleep(wait)
			lastErr = err
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

func mockEmbedding(text string) []float64 {
	dim := 768
	emb := make([]float64, dim)
	for i := 0; i < dim; i++ {
		emb[i] = float64(len(text)) / float64(dim)
	}
	return emb
}
