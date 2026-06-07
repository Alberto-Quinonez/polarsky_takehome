package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLoadInput(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		wantQ   string
		wantLen int
	}{
		{
			name:    "valid single quote",
			content: `{"query":"test query","quotes":[{"text":"hello","movie":"M1","character":"C1"}]}`,
			wantQ:   "test query",
			wantLen: 1,
		},
		{
			name: "valid multiple quotes",
			content: `{"query":"motivation","quotes":[
				{"text":"Just keep swimming.","movie":"Finding Nemo","character":"Dory"},
				{"text":"Get busy living.","movie":"Shawshank","character":"Andy"}
			]}`,
			wantQ:   "motivation",
			wantLen: 2,
		},
		{
			name:    "invalid JSON",
			content: `{bad json}`,
			wantErr: true,
		},
		{
			name:    "empty quotes array",
			content: `{"query":"q","quotes":[]}`,
			wantErr: true,
		},
		{
			name:    "missing quotes field",
			content: `{"query":"q"}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "quotes*.json")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())
			f.WriteString(tc.content)
			f.Close()

			input, err := loadInput(f.Name())
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if input.Query != tc.wantQ {
				t.Errorf("Query = %q, want %q", input.Query, tc.wantQ)
			}
			if len(input.Quotes) != tc.wantLen {
				t.Errorf("len(Quotes) = %d, want %d", len(input.Quotes), tc.wantLen)
			}
		})
	}
}

func TestLoadInput_MissingFile(t *testing.T) {
	_, err := loadInput("/nonexistent/path/quotes.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildPrompt(t *testing.T) {
	quotes := []Quote{
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"},
		{Text: "To infinity and beyond!", Movie: "Toy Story", Character: "Buzz Lightyear"},
	}
	prompt := buildPrompt("I need motivation", quotes)

	checks := []struct {
		needle string
		desc   string
	}{
		{`"I need motivation"`, "query"},
		{"Just keep swimming", "first quote text"},
		{"Finding Nemo", "first quote movie"},
		{"Dory", "first quote character"},
		{"To infinity and beyond", "second quote text"},
		{"2", "quote count"},
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c.needle) {
			t.Errorf("prompt missing %s (looking for %q)", c.desc, c.needle)
		}
	}
}

func TestPrintResults_RespectsN(t *testing.T) {
	rankings := []RankedQuote{
		{Text: "First", Movie: "M1", Character: "C1", Score: 0.92},
		{Text: "Second", Movie: "M2", Character: "C2", Score: 0.87},
		{Text: "Third", Movie: "M3", Character: "C3", Score: 0.75},
		{Text: "Fourth", Movie: "M4", Character: "C4", Score: 0.50},
	}

	// n=3: include first 3, exclude 4th
	output := captureStdout(t, func() { printResults("test query", rankings, 3) })
	if !strings.Contains(output, `"test query"`) {
		t.Error("output missing query")
	}
	if !strings.Contains(output, "Top 3") {
		t.Error("output missing 'Top 3' header")
	}
	if !strings.Contains(output, "M1") {
		t.Error("output missing top movie")
	}
	if strings.Contains(output, "Fourth") {
		t.Error("output should not include 4th result when n=3")
	}

	// n=2: only first 2
	output2 := captureStdout(t, func() { printResults("test query", rankings, 2) })
	if !strings.Contains(output2, "Top 2") {
		t.Error("output missing 'Top 2' header")
	}
	if strings.Contains(output2, "Third") {
		t.Error("output should not include 3rd result when n=2")
	}

	// n=4: all 4
	output4 := captureStdout(t, func() { printResults("test query", rankings, 4) })
	if !strings.Contains(output4, "Fourth") {
		t.Error("output should include 4th result when n=4")
	}
}

func TestPrintResults_ScoreClamped(t *testing.T) {
	rankings := []RankedQuote{
		{Text: "q1", Movie: "m1", Character: "c1", Score: 1.5},  // over 1.0
		{Text: "q2", Movie: "m2", Character: "c2", Score: -0.1}, // negative
		{Text: "q3", Movie: "m3", Character: "c3", Score: 0.5},
	}

	output := captureStdout(t, func() { printResults("q", rankings, 3) })

	if strings.Contains(output, "1.50") {
		t.Error("score > 1.0 should be clamped to 1.00")
	}
	if strings.Contains(output, "-0") {
		t.Error("negative score should be clamped to 0.00")
	}
}

func TestPrintResults_FewerResultsThanN(t *testing.T) {
	rankings := []RankedQuote{
		{Text: "Only one", Movie: "M1", Character: "C1", Score: 0.9},
	}
	output := captureStdout(t, func() { printResults("q", rankings, 3) })
	if !strings.Contains(output, "Only one") {
		t.Error("output missing the single result")
	}
}

// mockRanker is a test double for Ranker that returns pre-set results.
type mockRanker struct {
	results []RankedQuote
	err     error
}

func (m *mockRanker) Rank(_ context.Context, _ string, _ []Quote) ([]RankedQuote, error) {
	return m.results, m.err
}

func TestExecute_HappyPath(t *testing.T) {
	cfg := &Config{Query: "", TopN: 3}
	input := &Input{
		Query:  "motivation",
		Quotes: []Quote{{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory"}},
	}
	ranker := &mockRanker{results: []RankedQuote{
		{Text: "Just keep swimming.", Movie: "Finding Nemo", Character: "Dory", Score: 0.9},
	}}

	output := captureStdout(t, func() {
		if err := execute(cfg, ranker, input); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "motivation") {
		t.Error("output missing query")
	}
	if !strings.Contains(output, "Finding Nemo") {
		t.Error("output missing movie")
	}
}

func TestExecute_QueryOverride(t *testing.T) {
	cfg := &Config{Query: "custom override", TopN: 3}
	input := &Input{Query: "original", Quotes: []Quote{{Text: "q", Movie: "m", Character: "c"}}}
	ranker := &mockRanker{results: []RankedQuote{{Text: "q", Movie: "m", Character: "c", Score: 0.5}}}

	output := captureStdout(t, func() {
		execute(cfg, ranker, input) //nolint:errcheck
	})

	if !strings.Contains(output, "custom override") {
		t.Error("output should use overridden query, not original")
	}
}

func TestExecute_RankerError(t *testing.T) {
	cfg := &Config{TopN: 3}
	input := &Input{Query: "q", Quotes: []Quote{{Text: "t", Movie: "m", Character: "c"}}}
	ranker := &mockRanker{err: fmt.Errorf("LLM unavailable")}

	err := execute(cfg, ranker, input)
	if err == nil {
		t.Error("expected error when ranker fails")
	}
	if !strings.Contains(err.Error(), "LLM unavailable") {
		t.Errorf("error = %v, want to contain 'LLM unavailable'", err)
	}
}

// captureStdout runs fn and returns everything written to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}
