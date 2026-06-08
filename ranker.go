package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

// RankingMode selects the backend used to rank quotes.
// To add a new provider: add a constant here, implement Ranker, add a case in NewRanker.
type RankingMode int

const (
	ModeLocal  RankingMode = iota // 0 — BM25 lexical ranking, no API key
	ModeLlama                     // 1 — Ollama/Llama via OpenAI-compat chat completions
	ModeEmbed                     // 2 — Ollama embeddings via /v1/embeddings (dense cosine sim)
)

// RankedQuote is a quote with its relevance score assigned by the ranker.
type RankedQuote struct {
	Text      string  `json:"text"`
	Movie     string  `json:"movie"`
	Character string  `json:"character"`
	Score     float64 `json:"score"`
}

// Ranker ranks quotes by relevance to a query.
// Implementations can swap the underlying LLM provider without changing callers.
type Ranker interface {
	Rank(ctx context.Context, query string, quotes []Quote) ([]RankedQuote, error)
}

// openAIRanker calls any OpenAI-compatible chat completions endpoint.
// Compatible providers: OpenAI, Ollama, Azure OpenAI, any custom inference server.
type openAIRanker struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

// NewRanker returns the Ranker for the given mode.
func NewRanker(cfg *Config) Ranker {
	switch cfg.Mode {
	case ModeLocal:
		return NewLocalRanker()
	case ModeLlama:
		return &openAIRanker{
			baseURL: cfg.BaseURL,
			model:   cfg.Model,
			apiKey:  cfg.APIKey,
			client:  &http.Client{Timeout: 120 * time.Second},
		}
	case ModeEmbed:
		return newEmbedRanker(cfg)
	default:
		return NewLocalRanker()
	}
}

const systemPrompt = `You are an expert at emotional resonance and semantic relevance.
Given a user query describing a feeling or situation, rank movie quotes
by how well they emotionally connect to that experience.
Respond ONLY with valid JSON — no explanation, no markdown fences.
Format: {"rankings": [{"text":"...","movie":"...","character":"...","score":0.95}, ...]}
Scores must be 0.0–1.0. Return ALL quotes ranked from highest to lowest score.`

// OpenAI chat completions request/response shapes (subset used here).
type chatRequest struct {
	Model          string      `json:"model"`
	Messages       []chatMsg   `json:"messages"`
	ResponseFormat respFormat  `json:"response_format"`
	MaxTokens      int         `json:"max_tokens"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type rankingsEnvelope struct {
	Rankings []RankedQuote `json:"rankings"`
}

// post marshals body as JSON, POSTs to baseURL+path with auth headers,
// and decodes the response into out. It handles all HTTP transport concerns
// so that endpoint-specific methods only deal with application logic.
func (r *openAIRanker) post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.baseURL+path, bytes.NewReader(data))
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

func (r *openAIRanker) Rank(ctx context.Context, query string, quotes []Quote) ([]RankedQuote, error) {
	var chatResp chatResponse
	err := r.post(ctx, "/chat/completions", chatRequest{
		Model: r.model,
		Messages: []chatMsg{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: buildPrompt(query, quotes)},
		},
		ResponseFormat: respFormat{Type: "json_object"},
		MaxTokens:      2048,
	}, &chatResp)
	if err != nil {
		return nil, err
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	var envelope rankingsEnvelope
	if err := json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), &envelope); err != nil {
		return nil, fmt.Errorf("parsing rankings JSON: %w", err)
	}
	if len(envelope.Rankings) == 0 {
		return nil, fmt.Errorf("LLM returned no rankings")
	}

	// Ground results to input quotes — small models can hallucinate texts not in the list.
	// Re-apply Movie/Character from original input so metadata is always authoritative.
	quoteByText := make(map[string]Quote, len(quotes))
	for _, q := range quotes {
		quoteByText[q.Text] = q
	}
	grounded := make([]RankedQuote, 0, len(envelope.Rankings))
	for _, r := range envelope.Rankings {
		if q, ok := quoteByText[r.Text]; ok {
			r.Movie = q.Movie
			r.Character = q.Character
			grounded = append(grounded, r)
		}
	}
	if len(grounded) == 0 {
		// Model hallucinated quotes not in the input list — fall back to local BM25.
		fmt.Fprintf(os.Stderr, "warning: LLM returned no matching quotes, falling back to local ranking\n")
		return NewLocalRanker().Rank(ctx, query, quotes)
	}

	// Sort client-side for defensive correctness — LLM should already return sorted.
	sort.Slice(grounded, func(i, j int) bool {
		return grounded[i].Score > grounded[j].Score
	})

	return grounded, nil
}
