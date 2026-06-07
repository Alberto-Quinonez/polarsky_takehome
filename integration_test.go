//go:build integration

package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestIntegration_LocalMode runs the full BM25 pipeline against the real quotes.json.
// No API key required — always runs when the integration tag is set.
func TestIntegration_LocalMode(t *testing.T) {
	input, err := loadInput("quotes.json")
	if err != nil {
		t.Fatalf("loadInput: %v", err)
	}

	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), input.Query, input.Quotes)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	if len(results) != len(input.Quotes) {
		t.Errorf("expected %d results (all quotes), got %d", len(input.Quotes), len(results))
	}
	for i, r := range results {
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("results[%d].Score = %v, want in [0.0, 1.0]", i, r.Score)
		}
		if r.Text == "" || r.Movie == "" {
			t.Errorf("results[%d] has empty Text or Movie", i)
		}
	}
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d].Score=%v > [%d].Score=%v",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}

	t.Logf("Top 3 results (BM25) for %q:", input.Query)
	for i, r := range results[:min(3, len(results))] {
		t.Logf("  %d. [%.2f] %q (%s)", i+1, r.Score, r.Text, r.Movie)
	}
}

// TestIntegration_LlamaMode calls the real Ollama llama3 endpoint end-to-end.
// Skips if LLM_API_KEY is not set.
func TestIntegration_LlamaMode(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set — skipping llama integration test")
	}

	baseURL := firstNonEmpty(os.Getenv("LLM_BASE_URL"), "http://localhost:11434/v1")
	model := firstNonEmpty(os.Getenv("LLM_MODEL"), "llama3")

	input, err := loadInput("quotes.json")
	if err != nil {
		t.Fatalf("loadInput: %v", err)
	}

	ranker := &openAIRanker{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
	}

	results, err := ranker.Rank(context.Background(), input.Query, input.Quotes)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	assertResults(t, results, input.Query, "llama")
}

// TestIntegration_EmbedMode calls the real Ollama embeddings endpoint end-to-end.
// Skips if LLM_API_KEY is not set.
func TestIntegration_EmbedMode(t *testing.T) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		t.Skip("LLM_API_KEY not set — skipping embed integration test")
	}

	baseURL := firstNonEmpty(os.Getenv("LLM_BASE_URL"), "http://localhost:11434/v1")
	model := firstNonEmpty(os.Getenv("LLM_EMBED_MODEL"), os.Getenv("LLM_MODEL"), "nomic-embed-text")

	input, err := loadInput("quotes.json")
	if err != nil {
		t.Fatalf("loadInput: %v", err)
	}

	ranker := &embedRanker{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 60 * time.Second},
	}

	results, err := ranker.Rank(context.Background(), input.Query, input.Quotes)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}

	assertResults(t, results, input.Query, "embed")
}

// assertResults validates common result invariants and logs the top 3.
func assertResults(t *testing.T, results []RankedQuote, query, mode string) {
	t.Helper()
	if len(results) < 3 {
		t.Errorf("expected at least 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Text == "" {
			t.Errorf("results[%d].Text is empty", i)
		}
		if r.Movie == "" {
			t.Errorf("results[%d].Movie is empty", i)
		}
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("results[%d].Score = %v, want in [0.0, 1.0]", i, r.Score)
		}
	}
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d].Score=%v > [%d].Score=%v",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
	t.Logf("Top 3 results (%s) for %q:", mode, query)
	for i, r := range results[:min(3, len(results))] {
		t.Logf("  %d. [%.2f] %q (%s)", i+1, r.Score, r.Text, r.Movie)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
