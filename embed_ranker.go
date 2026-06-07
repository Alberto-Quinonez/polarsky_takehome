package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"
)

type embedRanker struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func newEmbedRanker(cfg *Config) Ranker {
	return &embedRanker{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type embeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // string or []string for batching
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (r *embedRanker) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", path, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func (r *embedRanker) embed(ctx context.Context, input interface{}) ([][]float64, error) {
	var resp embeddingResponse
	if err := r.post(ctx, "/embeddings", embeddingRequest{Model: r.model, Input: input}, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", resp.Error.Message)
	}
	vecs := make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// Rank makes 2 HTTP calls: one for the query and one batched call for all quotes.
// Results are ranked by dense cosine similarity between query and quote embeddings.
func (r *embedRanker) Rank(ctx context.Context, query string, quotes []Quote) ([]RankedQuote, error) {
	queryVecs, err := r.embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	if len(queryVecs) == 0 {
		return nil, fmt.Errorf("empty embedding response for query")
	}

	texts := make([]string, len(quotes))
	for i, q := range quotes {
		texts[i] = q.Text
	}
	quoteVecs, err := r.embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embedding quotes: %w", err)
	}
	if len(quoteVecs) != len(quotes) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(quotes), len(quoteVecs))
	}

	rankings := make([]RankedQuote, len(quotes))
	for i, q := range quotes {
		rankings[i] = RankedQuote{
			Text:      q.Text,
			Movie:     q.Movie,
			Character: q.Character,
			Score:     denseCosineSim(queryVecs[0], quoteVecs[i]),
		}
	}
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})
	return rankings, nil
}

// denseCosineSim returns cosine similarity in [0, 1] for normalized embedding vectors.
func denseCosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
