package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRanker_EmbedMode(t *testing.T) {
	cfg := &Config{Mode: ModeEmbed, BaseURL: "http://x", Model: "m", APIKey: "k"}
	r := NewRanker(cfg)
	if _, ok := r.(*embedRanker); !ok {
		t.Errorf("NewRanker(ModeEmbed) should return *embedRanker, got %T", r)
	}
}

func TestEmbedRanker_HappyPath(t *testing.T) {
	// Query vec ~ [1,0]; quote A gets [0,1] (sim=0), quote B gets [1,0] (sim=1).
	// B should rank first.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// Query embedding
			w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`)) //nolint:errcheck
		} else {
			// Batch quote embeddings: A=[0,1], B=[1,0]
			w.Write([]byte(`{"data":[{"embedding":[0,1]},{"embedding":[1,0]}]}`)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	quotes := []Quote{
		{Text: "quote A", Movie: "M1", Character: "C1"},
		{Text: "quote B", Movie: "M2", Character: "C2"},
	}

	ranker := &embedRanker{baseURL: srv.URL, model: "test", apiKey: "k", client: srv.Client()}
	results, err := ranker.Rank(context.Background(), "query", quotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Movie != "M2" {
		t.Errorf("expected M2 to rank first (sim=1.0), got %q", results[0].Movie)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("results[0].Score (%v) should be > results[1].Score (%v)", results[0].Score, results[1].Score)
	}
}

func TestEmbedRanker_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":{"message":"unauthorized"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	ranker := &embedRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for API error response, got nil")
	}
}

func TestEmbedRanker_MalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json")) //nolint:errcheck
	}))
	defer srv.Close()

	ranker := &embedRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for malformed response body, got nil")
	}
}

func TestEmbedRanker_ZeroNormEmbedding(t *testing.T) {
	// All-zero embeddings should produce score=0 without panicking.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			w.Write([]byte(`{"data":[{"embedding":[0,0]}]}`)) //nolint:errcheck
		} else {
			w.Write([]byte(`{"data":[{"embedding":[0,0]},{"embedding":[0,0]}]}`)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	ranker := &embedRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	results, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range results {
		if r.Score != 0 {
			t.Errorf("results[%d].Score = %v, want 0 for zero-norm embedding", i, r.Score)
		}
	}
}

func TestEmbedRanker_EmbeddingCountMismatch(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`)) //nolint:errcheck
		} else {
			// Only 1 embedding returned for 2 quotes — mismatch
			w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`)) //nolint:errcheck
		}
	}))
	defer srv.Close()

	ranker := &embedRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for embedding count mismatch, got nil")
	}
}

func TestDenseCosineSim(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []float64
		wantPos bool
	}{
		{"identical", []float64{1, 0}, []float64{1, 0}, true},
		{"orthogonal", []float64{1, 0}, []float64{0, 1}, false},
		{"zero norm a", []float64{0, 0}, []float64{1, 0}, false},
		{"zero norm b", []float64{1, 0}, []float64{0, 0}, false},
		{"length mismatch", []float64{1, 0}, []float64{1}, false},
		{"empty", []float64{}, []float64{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := denseCosineSim(tc.a, tc.b)
			if tc.wantPos && score <= 0 {
				t.Errorf("denseCosineSim = %v, want > 0", score)
			}
			if !tc.wantPos && score != 0 {
				t.Errorf("denseCosineSim = %v, want 0", score)
			}
			if score < 0 || score > 1 {
				t.Errorf("denseCosineSim = %v out of [0, 1]", score)
			}
		})
	}
}
