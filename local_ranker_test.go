package main

import (
	"context"
	"testing"
)

func TestLocalRanker_ReturnsAllQuotesSorted(t *testing.T) {
	quotes := []Quote{
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"},
		{Text: "Get busy living.", Movie: "The Shawshank Redemption", Character: "Andy Dufresne"},
		{Text: "Why so serious?", Movie: "The Dark Knight", Character: "Joker"},
	}

	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), "motivation keep going", quotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	// Results must be sorted descending by score.
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: [%d].Score=%v > [%d].Score=%v",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestLocalRanker_ScoresInRange(t *testing.T) {
	quotes := []Quote{
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"},
		{Text: "To infinity and beyond!", Movie: "Toy Story", Character: "Buzz"},
	}
	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), "keep going", quotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range results {
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("results[%d].Score = %v, want in [0.0, 1.0]", i, r.Score)
		}
	}
}

func TestLocalRanker_ExactMatchScoresHigher(t *testing.T) {
	// "keep swimming" overlaps directly with the first quote
	quotes := []Quote{
		{Text: "keep swimming forward always", Movie: "M1", Character: "C1"},
		{Text: "why so serious clown face", Movie: "M2", Character: "C2"},
	}
	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), "keep swimming", quotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Movie != "M1" {
		t.Errorf("expected M1 to rank first (direct word overlap), got %q", results[0].Movie)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("M1 score (%v) should be > M2 score (%v)", results[0].Score, results[1].Score)
	}
}

func TestLocalRanker_EmptyQueryReturnsZeroScores(t *testing.T) {
	quotes := []Quote{
		{Text: "Just keep swimming.", Movie: "M1", Character: "C1"},
	}
	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), "", quotes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Score != 0 {
		t.Errorf("empty query score = %v, want 0", results[0].Score)
	}
}

func TestLocalRanker_PreservesQuoteFields(t *testing.T) {
	q := Quote{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"}
	ranker := NewLocalRanker()
	results, err := ranker.Rank(context.Background(), "swimming", []Quote{q})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Text != q.Text {
		t.Errorf("Text = %q, want %q", results[0].Text, q.Text)
	}
	if results[0].Movie != q.Movie {
		t.Errorf("Movie = %q, want %q", results[0].Movie, q.Movie)
	}
	if results[0].Character != q.Character {
		t.Errorf("Character = %q, want %q", results[0].Character, q.Character)
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Just keep swimming.", []string{"keep", "swimming"}},           // "just" is stopword
		{"Why so serious?", []string{"why", "serious"}},                 // "so" is stopword
		{"", []string(nil)},                                              // empty input
		{"THE quick brown fox", []string{"quick", "brown", "fox"}},     // "the" stopword, lowercase
		{"hello-world", []string{"hello", "world"}},                     // hyphen splits tokens
	}
	for _, tc := range tests {
		got := tokenize(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("tokenize(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestTermFreq(t *testing.T) {
	tf := termFreq([]string{"a", "b", "a", "c", "a"})
	if tf["a"] != 3 {
		t.Errorf("tf[a] = %v, want 3", tf["a"])
	}
	if tf["b"] != 1 {
		t.Errorf("tf[b] = %v, want 1", tf["b"])
	}
}

func TestBM25Score(t *testing.T) {
	corpus := [][]string{
		{"keep", "swimming"},
		{"serious", "clown"},
	}
	avgLen := avgDocLen(corpus)

	tests := []struct {
		name    string
		query   []string
		doc     []string
		wantPos bool
	}{
		{"zero overlap", []string{"hello"}, []string{"keep", "swimming"}, false},
		{"exact match", []string{"keep"}, []string{"keep", "swimming"}, true},
		{"empty query", []string{}, []string{"keep", "swimming"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score := bm25Score(tc.query, tc.doc, corpus, avgLen)
			if tc.wantPos && score <= 0 {
				t.Errorf("bm25Score = %v, want > 0", score)
			}
			if !tc.wantPos && score != 0 {
				t.Errorf("bm25Score = %v, want 0", score)
			}
		})
	}
}

func TestBM25Score_HigherOverlapScoresHigher(t *testing.T) {
	corpus := [][]string{
		{"keep", "swimming", "forward"},
		{"keep"},
		{"hello"},
	}
	avgLen := avgDocLen(corpus)
	query := []string{"keep", "swimming"}

	scoreA := bm25Score(query, corpus[0], corpus, avgLen)
	scoreB := bm25Score(query, corpus[1], corpus, avgLen)

	if scoreA <= scoreB {
		t.Errorf("doc with 2 matching terms (%.4f) should score higher than doc with 1 (%.4f)", scoreA, scoreB)
	}
}

func TestNewRanker_LocalMode(t *testing.T) {
	cfg := &Config{Mode: ModeLocal}
	r := NewRanker(cfg)
	if _, ok := r.(*localRanker); !ok {
		t.Errorf("NewRanker(ModeLocal) should return *localRanker, got %T", r)
	}
}

func TestNewRanker_LLMModes(t *testing.T) {
	cfg := &Config{Mode: ModeLlama, BaseURL: "http://x", Model: "m", APIKey: "k"}
	r := NewRanker(cfg)
	if _, ok := r.(*openAIRanker); !ok {
		t.Errorf("NewRanker(ModeLlama) should return *openAIRanker, got %T", r)
	}
}
