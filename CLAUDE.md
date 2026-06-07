# quote-finder — Project Conventions

## Architecture

The `Ranker` interface isolates the LLM backend from all business logic. All providers
speak the OpenAI chat completions protocol — swap `LLM_BASE_URL` + `LLM_MODEL` to change
the backend without touching any Go code.

```
main.go      — wire deps, parse args, call Rank, print
config.go    — Config struct; env vars + CLI flags; priority: flag > env > default
loader.go    — loadInput (JSON), buildPrompt, printResults
ranker.go    — Ranker interface + openAIRanker (raw net/http, stdlib only)
```

## Error Handling

- Always wrap with context: `fmt.Errorf("doing X: %w", err)`
- Fatal errors go to stderr and `os.Exit(1)` — no panics in user-facing paths
- Validate at boundaries (file load, HTTP response) — trust internal code

## Testing

- **Unit tests**: use `httptest.NewServer` to mock the LLM endpoint. Never call real APIs.
- **Integration tests**: `//go:build integration` tag; skip automatically if `LLM_API_KEY` unset.
- Table-driven tests for pure functions.

## Validation Loop

```
make vet && make test
```

Both must pass before committing.

## Environment Variables

| Variable      | Default                        | Description              |
|---------------|--------------------------------|--------------------------|
| LLM_BASE_URL  | https://api.openai.com/v1      | OpenAI-compatible API URL |
| LLM_MODEL     | gpt-4o-mini                    | Model name to request    |
| LLM_API_KEY   | (required)                     | Bearer token             |
