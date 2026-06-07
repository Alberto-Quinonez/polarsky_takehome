package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	input, err := loadInput(cfg.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ranker := NewRanker(cfg)
	if err := execute(cfg, ranker, input); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// execute runs the ranking pipeline with injected dependencies.
// Extracted from main to allow unit testing with mock rankers.
func execute(cfg *Config, ranker Ranker, input *Input) error {
	query := input.Query
	if cfg.Query != "" {
		query = cfg.Query
	}

	rankings, err := ranker.Rank(context.Background(), query, input.Quotes)
	if err != nil {
		return fmt.Errorf("ranking quotes: %w", err)
	}

	printResults(query, rankings, cfg.TopN)
	return nil
}
