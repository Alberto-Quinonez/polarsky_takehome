package main

import (
	"os"
	"strings"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"first wins", []string{"a", "b", "c"}, "a"},
		{"skip empty first", []string{"", "b", "c"}, "b"},
		{"skip two empty", []string{"", "", "c"}, "c"},
		{"all empty", []string{"", "", ""}, ""},
		{"no args", []string{}, ""},
		{"single value", []string{"only"}, "only"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := firstNonEmpty(tc.vals...)
			if got != tc.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tc.vals, got, tc.want)
			}
		})
	}
}

func TestResolveConfig_Defaults(t *testing.T) {
	cfg, err := resolveConfig("", "", "my-api-key", "", "quotes.json", 3, ModeLlama)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default openai URL", cfg.BaseURL)
	}
	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want gpt-4o-mini", cfg.Model)
	}
	if cfg.APIKey != "my-api-key" {
		t.Errorf("APIKey = %q, want my-api-key", cfg.APIKey)
	}
	if cfg.File != "quotes.json" {
		t.Errorf("File = %q, want quotes.json", cfg.File)
	}
	if cfg.TopN != 3 {
		t.Errorf("TopN = %d, want 3", cfg.TopN)
	}
}

func TestResolveConfig_CustomValues(t *testing.T) {
	cfg, err := resolveConfig(
		"http://localhost:11434/v1",
		"llama3",
		"ollama",
		"custom query",
		"custom.json",
		5,
		ModeLlama,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want Ollama URL", cfg.BaseURL)
	}
	if cfg.Model != "llama3" {
		t.Errorf("Model = %q, want llama3", cfg.Model)
	}
	if cfg.Query != "custom query" {
		t.Errorf("Query = %q, want custom query", cfg.Query)
	}
	if cfg.TopN != 5 {
		t.Errorf("TopN = %d, want 5", cfg.TopN)
	}
}

func TestResolveConfig_MissingFile(t *testing.T) {
	_, err := resolveConfig("", "", "key", "", "", 3, ModeLlama)
	if err == nil {
		t.Error("expected error when file is empty")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Errorf("error should contain usage info, got: %v", err)
	}
}

func TestResolveConfig_MissingAPIKey(t *testing.T) {
	_, err := resolveConfig("", "", "", "", "quotes.json", 3, ModeLlama)
	if err == nil {
		t.Error("expected error when API key is empty")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("error should mention API key, got: %v", err)
	}
}

func TestResolveConfig_LocalSkipsAPIKeyCheck(t *testing.T) {
	cfg, err := resolveConfig("", "", "", "", "quotes.json", 3, ModeLocal)
	if err != nil {
		t.Fatalf("ModeLocal with no API key should not error, got: %v", err)
	}
	if cfg.Mode != ModeLocal {
		t.Errorf("cfg.Mode = %v, want ModeLocal", cfg.Mode)
	}
}

func TestLoadConfigFrom_LocalFlag(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")

	cfg, err := loadConfigFrom([]string{"--mode", "local", "quotes.json"})
	if err != nil {
		t.Fatalf("--mode local should not require API key, got: %v", err)
	}
	if cfg.Mode != ModeLocal {
		t.Errorf("cfg.Mode = %v, want ModeLocal", cfg.Mode)
	}
}

func TestResolveConfig_InvalidTopN(t *testing.T) {
	tests := []struct{ name string; n int }{
		{"zero", 0},
		{"negative", -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveConfig("", "", "key", "", "quotes.json", tc.n, ModeLlama)
			if err == nil {
				t.Error("expected error for invalid top-n")
			}
			if !strings.Contains(err.Error(), "--top-n") {
				t.Errorf("error should mention --top-n, got: %v", err)
			}
		})
	}
}

func TestLoadConfigFrom_BasicArgs(t *testing.T) {
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg, err := loadConfigFrom([]string{"quotes.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.File != "quotes.json" {
		t.Errorf("File = %q, want quotes.json", cfg.File)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
	}
}

func TestLoadConfigFrom_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("LLM_MODEL", "env-model")
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_BASE_URL", "")

	cfg, err := loadConfigFrom([]string{
		"--model", "flag-model",
		"--base-url", "http://custom:8080/v1",
		"quotes.json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "flag-model" {
		t.Errorf("Model = %q, want flag-model (flag beats env)", cfg.Model)
	}
	if cfg.BaseURL != "http://custom:8080/v1" {
		t.Errorf("BaseURL = %q, want flag value", cfg.BaseURL)
	}
}

func TestLoadConfigFrom_QueryFlag(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg, err := loadConfigFrom([]string{"--query", "custom question", "quotes.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Query != "custom question" {
		t.Errorf("Query = %q, want 'custom question'", cfg.Query)
	}
}

func TestLoadConfigFrom_TopNFlag(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg, err := loadConfigFrom([]string{"--top-n", "5", "quotes.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TopN != 5 {
		t.Errorf("TopN = %d, want 5", cfg.TopN)
	}
}

func TestLoadConfigFrom_TopNDefault(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg, err := loadConfigFrom([]string{"quotes.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TopN != 3 {
		t.Errorf("TopN = %d, want default 3", cfg.TopN)
	}
}

func TestLoadConfigFrom_NoFile(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	_, err := loadConfigFrom([]string{})
	if err == nil {
		t.Error("expected error when no file arg provided")
	}
}

func TestLoadConfigFrom_NoAPIKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	_, err := loadConfigFrom([]string{"quotes.json"})
	if err == nil {
		t.Error("expected error when no API key provided")
	}
}

func TestLoadConfigFrom_UnknownFlag(t *testing.T) {
	_, err := loadConfigFrom([]string{"--unknown-flag", "quotes.json"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestHoistFlags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"flags before positional — unchanged",
			[]string{"--mode", "local", "quotes.json"},
			[]string{"--mode", "local", "quotes.json"},
		},
		{
			"positional before flags — reordered",
			[]string{"quotes.json", "--mode", "local"},
			[]string{"--mode", "local", "quotes.json"},
		},
		{
			"mixed flags and positional",
			[]string{"quotes.json", "--top-n", "5", "--mode", "local"},
			[]string{"--top-n", "5", "--mode", "local", "quotes.json"},
		},
		{
			"equals form — single token",
			[]string{"quotes.json", "--mode=local"},
			[]string{"--mode=local", "quotes.json"},
		},
		{
			"positional only",
			[]string{"quotes.json"},
			[]string{"quotes.json"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hoistFlags(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("hoistFlags(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("hoistFlags[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input   string
		want    RankingMode
		wantErr bool
	}{
		{"local", ModeLocal, false},
		{"llama", ModeLlama, false},
		{"embed", ModeEmbed, false},
		{"", ModeLlama, false},
		{"bogus", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseMode(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseMode(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLoadConfigFrom_ModeFlag(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	for _, tc := range []struct{ flag string; want RankingMode }{
		{"llama", ModeLlama},
		{"embed", ModeEmbed},
	} {
		cfg, err := loadConfigFrom([]string{"--mode", tc.flag, "quotes.json"})
		if err != nil {
			t.Fatalf("--mode %s unexpected error: %v", tc.flag, err)
		}
		if cfg.Mode != tc.want {
			t.Errorf("--mode %s: cfg.Mode = %v, want %v", tc.flag, cfg.Mode, tc.want)
		}
	}
}

func TestLoadConfigFrom_InvalidMode(t *testing.T) {
	_, err := loadConfigFrom([]string{"--mode", "bogus", "quotes.json"})
	if err == nil {
		t.Error("expected error for unknown --mode value")
	}
}

func TestLoadConfig_DelegatesCorrectly(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	t.Setenv("LLM_API_KEY", "os-args-key")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	os.Args = []string{"quote-finder", "quotes.json"}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig unexpected error: %v", err)
	}
	if cfg.File != "quotes.json" {
		t.Errorf("File = %q, want quotes.json", cfg.File)
	}
	if cfg.APIKey != "os-args-key" {
		t.Errorf("APIKey = %q, want os-args-key", cfg.APIKey)
	}
}
