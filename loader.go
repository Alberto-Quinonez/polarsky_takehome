package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// Input mirrors the quotes.json file structure.
type Input struct {
	Query  string  `json:"query"`
	Quotes []Quote `json:"quotes"`
}

// Quote is a single movie quote entry.
type Quote struct {
	Text      string `json:"text"`
	Movie     string `json:"movie"`
	Character string `json:"character"`
}

// Loader loads a query and quote list from a source.
// The source interpretation is implementation-specific — a file path today,
// a quote-set ID when backed by a database.
type Loader interface {
	Load(ctx context.Context, source string) (*Input, error)
}

// jsonFileLoader implements Loader by reading a JSON file from disk.
type jsonFileLoader struct{}

func NewLoader() Loader {
	return &jsonFileLoader{}
}

func (l *jsonFileLoader) Load(_ context.Context, path string) (*Input, error) {
	return loadInput(path)
}

func loadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	if len(input.Quotes) == 0 {
		return nil, fmt.Errorf("no quotes found in %q", path)
	}
	return &input, nil
}

// buildPrompt formats the query and quotes into the user message sent to the LLM.
func buildPrompt(query string, quotes []Quote) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Query: %q\n\n", query)
	fmt.Fprintf(&sb, "Rank these %d movie quotes by emotional relevance to the query above:\n\n", len(quotes))
	for i, q := range quotes {
		fmt.Fprintf(&sb, "%d. %q — %s (%s)\n", i+1, q.Text, q.Character, q.Movie)
	}
	return sb.String()
}

// printResults writes the top n ranked quotes to stdout in the expected format.
func printResults(query string, rankings []RankedQuote, n int) {
	fmt.Printf("Top %d quotes for: %q\n\n", n, query)
	for i, r := range rankings {
		if i >= n {
			break
		}
		score := math.Max(0, math.Min(1, r.Score))
		fmt.Printf("%d. [%.2f] %q - %s (%s)\n", i+1, score, r.Text, r.Character, r.Movie)
	}
}
