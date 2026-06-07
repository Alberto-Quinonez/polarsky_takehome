package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRanker_SetsFields(t *testing.T) {
	cfg := &Config{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
		APIKey:  "test-key",
		Mode:    ModeLlama,
	}
	r := NewRanker(cfg)
	impl, ok := r.(*openAIRanker)
	if !ok {
		t.Fatal("NewRanker(Local=false) should return *openAIRanker")
	}
	if impl.baseURL != cfg.BaseURL {
		t.Errorf("baseURL = %q, want %q", impl.baseURL, cfg.BaseURL)
	}
	if impl.model != cfg.Model {
		t.Errorf("model = %q, want %q", impl.model, cfg.Model)
	}
	if impl.client == nil {
		t.Error("http client is nil")
	}
}

// mockLLMResponse builds a chatResponse whose first choice contains the given rankings.
func mockLLMResponse(rankings []RankedQuote) chatResponse {
	content, _ := json.Marshal(rankingsEnvelope{Rankings: rankings})
	return chatResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{{Message: struct {
			Content string `json:"content"`
		}{Content: string(content)}}},
	}
}

var testQuotes = []Quote{
	{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"},
	{Text: "Get busy living.", Movie: "The Shawshank Redemption", Character: "Andy Dufresne"},
}

func TestOpenAIRanker_HappyPath(t *testing.T) {
	want := []RankedQuote{
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory", Score: 0.92},
		{Text: "Get busy living.", Movie: "The Shawshank Redemption", Character: "Andy Dufresne", Score: 0.75},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		var body chatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if body.Model != "test-model" {
			t.Errorf("model = %q, want test-model", body.Model)
		}
		if len(body.Messages) != 2 {
			t.Errorf("messages count = %d, want 2", len(body.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockLLMResponse(want))
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "test-model", apiKey: "test-key", client: srv.Client()}
	results, err := ranker.Rank(context.Background(), "motivation", testQuotes)
	if err != nil {
		t.Fatalf("Rank() unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Score != 0.92 {
		t.Errorf("results[0].Score = %v, want 0.92", results[0].Score)
	}
	if results[0].Movie != "Finding Nemo" {
		t.Errorf("results[0].Movie = %q, want Finding Nemo", results[0].Movie)
	}
}

func TestOpenAIRanker_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "invalid API key"},
		})
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for API error response, got nil")
	}
}

func TestOpenAIRanker_MalformedContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "not valid json"}}},
		})
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for malformed JSON content, got nil")
	}
}

func TestOpenAIRanker_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{})
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for empty choices, got nil")
	}
}

func TestOpenAIRanker_EmptyRankings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := json.Marshal(rankingsEnvelope{Rankings: []RankedQuote{}})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: string(content)}}},
		})
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for empty rankings, got nil")
	}
}

func TestOpenAIRanker_SortsClientSide(t *testing.T) {
	// Return unsorted rankings (using testQuotes texts) — verify client sorts them.
	unsorted := []RankedQuote{
		{Text: "Get busy living.", Movie: "The Shawshank Redemption", Character: "Andy Dufresne", Score: 0.5},
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory", Score: 0.9},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockLLMResponse(unsorted))
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	results, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err != nil {
		t.Fatalf("Rank() unexpected error: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d].Score=%v > [%d].Score=%v",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
	if results[0].Text != "Just keep swimming." {
		t.Errorf("highest-score result = %q, want 'Just keep swimming.'", results[0].Text)
	}
}

func TestOpenAIRanker_UnreachableServer(t *testing.T) {
	ranker := &openAIRanker{
		baseURL: "http://127.0.0.1:1", // nothing listening
		model:   "m",
		apiKey:  "k",
		client:  &http.Client{},
	}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for unreachable server, got nil")
	}
}

func TestOpenAIRanker_MalformedResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("this is not json")) //nolint:errcheck
	}))
	defer srv.Close()

	ranker := &openAIRanker{baseURL: srv.URL, model: "m", apiKey: "k", client: srv.Client()}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for malformed response body, got nil")
	}
}

func TestOpenAIRanker_InvalidURL(t *testing.T) {
	ranker := &openAIRanker{
		baseURL: "://not-a-url", // malformed — NewRequestWithContext fails
		model:   "m",
		apiKey:  "k",
		client:  &http.Client{},
	}
	_, err := ranker.Rank(context.Background(), "q", testQuotes)
	if err == nil {
		t.Error("expected error for malformed base URL, got nil")
	}
}
