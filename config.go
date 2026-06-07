package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime configuration resolved from CLI flags and env vars.
// Priority: CLI flag → env var → default.
type Config struct {
	BaseURL string      // LLM API base URL (OpenAI-compatible)
	Model   string      // model name to request
	APIKey  string      // bearer token for the LLM provider
	File    string      // path to quotes.json
	Query   string      // optional query override (empty = use value from JSON)
	TopN    int         // number of top quotes to display (default 3)
	Mode    RankingMode // ranking backend (see RankingMode constants)
}

// LoadConfig parses os.Args and environment variables to build a Config.
func LoadConfig() (*Config, error) {
	return loadConfigFrom(os.Args[1:])
}

// loadConfigFrom parses the given args slice — extracted for testability.
func loadConfigFrom(args []string) (*Config, error) {
	fs := flag.NewFlagSet("quote-finder", flag.ContinueOnError)
	baseURL := fs.String("base-url", "", "LLM API base URL (overrides LLM_BASE_URL)")
	model := fs.String("model", "", "Model name (overrides LLM_MODEL)")
	apiKey := fs.String("api-key", "", "API key (overrides LLM_API_KEY)")
	query := fs.String("query", "", "Override the query from the JSON file")
	topN := fs.Int("top-n", 3, "Number of top quotes to display")
	modeStr := fs.String("mode", "llama", "Ranking backend: local, llama, embed")

	// flag.Parse stops at the first non-flag arg, so reorder to support any positional placement.
	if err := fs.Parse(hoistFlags(args)); err != nil {
		return nil, err
	}

	mode, err := parseMode(*modeStr)
	if err != nil {
		return nil, err
	}

	file := ""
	if fs.NArg() >= 1 {
		file = fs.Arg(0)
	}

	return resolveConfig(
		firstNonEmpty(*baseURL, os.Getenv("LLM_BASE_URL")),
		firstNonEmpty(*model, os.Getenv("LLM_MODEL")),
		firstNonEmpty(*apiKey, os.Getenv("LLM_API_KEY")),
		*query,
		file,
		*topN,
		mode,
	)
}

// parseMode converts a --mode flag string to a RankingMode constant.
func parseMode(s string) (RankingMode, error) {
	switch s {
	case "local":
		return ModeLocal, nil
	case "llama", "":
		return ModeLlama, nil
	case "embed":
		return ModeEmbed, nil
	default:
		return 0, fmt.Errorf("unknown --mode %q: valid values are local, llama, embed", s)
	}
}

// resolveConfig builds a Config from already-resolved values (flags + env merged by caller).
// Extracted for testability without touching flag.Parse().
func resolveConfig(baseURL, model, apiKey, query, file string, topN int, mode RankingMode) (*Config, error) {
	if file == "" {
		return nil, fmt.Errorf("usage: quote-finder [flags] <quotes.json>\n\nFlags:\n" +
			"  --base-url  LLM API base URL (default: $LLM_BASE_URL or https://api.openai.com/v1)\n" +
			"  --model     Model name       (default: $LLM_MODEL or gpt-4o-mini)\n" +
			"  --api-key   API key          (default: $LLM_API_KEY)\n" +
			"  --query     Override the query from the JSON file\n" +
			"  --top-n     Number of top quotes to display (default: 3)\n" +
			"  --mode      Ranking backend: local, llama, embed (default: llama)")
	}
	if mode != ModeLocal && apiKey == "" {
		return nil, fmt.Errorf("API key required: set LLM_API_KEY or use --api-key (or use --mode local for offline ranking)")
	}
	if topN <= 0 {
		return nil, fmt.Errorf("--top-n must be greater than 0, got %d", topN)
	}
	return &Config{
		BaseURL: firstNonEmpty(baseURL, "https://api.openai.com/v1"),
		Model:   firstNonEmpty(model, "gpt-4o-mini"),
		APIKey:  apiKey,
		Query:   query,
		File:    file,
		TopN:    topN,
		Mode:    mode,
	}, nil
}

// hoistFlags reorders args so all flag tokens precede positional args,
// working around flag.Parse stopping at the first non-flag argument.
// Handles both "--flag value" (two tokens) and "--flag=value" (one token) forms.
func hoistFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		// "--flag value" form: consume the next token as the flag's value.
		if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
