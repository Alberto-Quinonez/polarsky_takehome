# quote-finder

A Go CLI tool that ranks movie quotes by emotional and semantic relevance to a user query, built for the PolarSky take-home challenge.

## Approach

Rather than keyword matching, `quote-finder` delegates ranking to an LLM via a structured JSON prompt. All 20 quotes are sent in a single request alongside the query; the model returns them scored by emotional resonance and semantic fit. The tool uses the **OpenAI chat completions protocol** as its universal inference standard — meaning any compatible backend (OpenAI, Ollama, Azure, or a custom inference server) can be swapped in by changing two environment variables, with no code changes. Results are sorted client-side after the LLM response as a defensive measure.

## Key Prompts Used

These are the prompts I gave to [Claude Code](https://claude.ai/code) (my AI assistant) during this session, in order:

---

**Prompt 1 — Initial context and planning**

> "Hello, I need to work on my polarsky takehome exam, can you assist? Here is the job description for context: /c/Users/test/dev/interview_prep/polar_sky. Can we pull my global Golang rules into the claude.md file? Once you have the context, lets go into plan mode"

_What happened:_ Claude reviewed the job description, challenge requirements, and my personal Go conventions (error wrapping, interface-driven design, table-driven tests, `httptest` mocking). It then proposed a plan I reviewed and refined — adding the configurable LLM backend, scalability roadmap, and Docker — before any implementation started.

---

**Prompt 2 — Architecture decision: configurable LLM**

> "Can we make the LLM configurable? and not necessarily bind us to the anthropic API? I know polarsky runs their own custom models and likes to modulate the model based on the input prompt. This would align well to the inference layer that I would be working on at polarsky."

_What happened:_ This was the most important architectural prompt. Claude proposed using the OpenAI chat completions protocol as a universal standard (raw `net/http`, no SDK), configurable via `LLM_BASE_URL` / `LLM_MODEL` / `LLM_API_KEY`. This decision shapes the entire architecture and directly mirrors PolarSky's inference layer work.

---

**Prompt 3 — Quality and deployment requirements**

> "I need to also include the prompts I used. Can you give me different options? what other solutions are possible for this problem? can you also ensure that we get 80% test coverage? can we maybe create integration tests as well? I would like to have a deployable docker file as well for easy integration"

_What happened:_ Claude walked through three ranking approaches (LLM, TF-IDF, hybrid), added comprehensive unit tests with a mock HTTP server (`httptest.NewServer`), integration tests behind a build tag, and a multi-stage Dockerfile. Coverage reached 91%.

---

**Prompt 4 — Scalability roadmap**

> "yeah, lets add into the 'what I'd do next' a plan to deploy this as a simple rest server that can scale up and potentially plug it to a DB/cache for quick response"

_What happened:_ Claude added a phased "What I'd Do Next" section covering REST service lift, Redis caching, Postgres + pgvector embeddings, async job queuing, and user accounts with preference persistence.

---

**Prompt 5 — Local offline mode**

> "udpate readme for running the CLI. we require an API key. 1 more thing, re-reading the challenge, does it seem like I should be ranking the quotes in my CLI app? maybe lets create a simple local mode option which would be a different implementation?"

_What happened:_ Claude implemented a `--mode local` backend using TF cosine similarity with stopword filtering — no API key required, runs fully offline. This also prompted a discussion on the ranking decision itself (the CLI does rank; the `--mode` flag makes the approach configurable).

---

**Prompt 6 — Iota-based mode enum**

> "I really do not like the boolean approach, lets use iota with potential approaches. Example: 0 -> local, 1 -> Lama, 2 -> anthropic, etc. that way the code is more legible and it is explicit we can cycle through different API's for the classifying."

_What happened:_ Claude replaced `Local bool` in `Config` with a `RankingMode int` iota enum (`ModeLocal`, `ModeLlama`), replaced `--local` with a `--mode` string flag, and added `parseMode()` + `hoistFlags()` helpers. The factory `NewRanker` now switches on the mode constant, making future backends a one-line addition.

---

**Prompt 7 — User engagement feature**

> "I want to go over the extended goal of the challenge... [discussed feedback loop concept] ...this would be only added into the readme, we do not need to implement it yet"

_What happened:_ Claude designed a resonance feedback loop — after displaying results the CLI prompts the user to pick which quote resonated, persisting selections locally and using them to personalize future LLM system prompts. Added to README with a Phase 5 evolution path to user accounts + Postgres.

---

**Prompt 8 — Testing against local Ollama**

> "ok lets test the ollama implementation. I have my local server setup and running."

_What happened:_ First run timed out (30s limit hit before the 8B model finished generating). Claude diagnosed it with a direct `curl` smoke test, bumped the HTTP timeout to 120s, and the second run succeeded. Noted that Ollama keeps the model warm in memory — subsequent runs are significantly faster, directly illustrating the "pre-warm on startup" mitigation in the scalability table.

---

**Prompt 9 — Simplify to testable modes only**

> "lets remove everything except for local mode and llama, since I will be unable to test the other versions."

_What happened:_ Claude removed `ModeAnthropic` and `ModeOpenAI` from the iota, updated `parseMode`, all tests, the Makefile, and the README. Only modes that can actually be run are exposed.

---

**Prompt 10 — BM25 + Ollama embeddings + engagement alternatives**

> "I want to switch from TF cosine for local mode to BM25, I want to also implement ollama embeddings, and document the following user engagement alternatives (social sharing, friend gifting and emotional insights)"

_What happened:_ Claude replaced the TF cosine ranker with BM25 (k1=1.5, b=0.75) — a ranking function that handles term saturation and document length normalization, making it meaningfully better for short quote texts. A new `--mode embed` backend was added using Ollama's `/v1/embeddings` endpoint: one batched HTTP call retrieves dense vector representations for all quotes, and results are ranked by cosine similarity against the query embedding — capturing semantic meaning that BM25 misses entirely (e.g. "perseverance" → "Just keep swimming"). Three engagement alternatives — social sharing, friend gifting, and emotional insights — were documented in the README User Engagement section.

---

**Prompt 11 — Observability: logging, metrics, and alerting**

> "lets add into the stretch goals, observability, logging, metrics and alerting"

_What happened:_ Claude added Phase 6 to the "What I'd Do Next" section covering three observability layers: structured JSON logging (via `log/slog`) with trace IDs and latency per request; Prometheus metrics (counters and histograms for request volume, latency, error rate, cache hit ratio, and embedding quality); and alerting rules for error rate spikes, P95 latency, cache degradation, and silent failures. The implementation path was documented using a `metricsRanker` decorator pattern — wrapping any `Ranker` backend to record metrics without touching business logic.

---

**Prompt 12 — Resilience: fallback to local ranking on API failure**
> "what about anything similar to the previous error we just found? anywhere else that can be made a bit more bulletproof by falling back on the local ranker?"

_What happened:_ Claude identified two failure modes not yet covered: the LLM/embed API failing entirely (network error, Ollama down, timeout), and the existing hallucination fallback only living inside `openAIRanker`. Added a second fallback layer in `execute()` in `main.go` — if the primary ranker errors and the mode is not local, it warns to stderr and retries with BM25. Local mode is unaffected. A test was added to verify the fallback produces valid output when the ranker fails.

---

**Key iteration on the LLM prompt itself:**

My first system prompt asked the model to return only the top 3. Claude suggested changing it to rank _all_ quotes and slice top 3 client-side — the model produces more calibrated relative scores when it sees the full distribution. This was the right call.

## Reflections

**What worked well:**

- Claude Code's ability to plan before writing was valuable — having a written plan that I reviewed and amended (adding the configurable LLM, scalability roadmap, Docker) before any code was written avoided costly mid-implementation pivots.
- The `httptest.NewServer` pattern in Go makes HTTP mocking clean and self-contained. Claude suggested it immediately and the tests are straightforward to read.
- Structured JSON output (`response_format: json_object`) eliminated parsing fragility — the LLM returns a predictable schema rather than free-text I have to regex out.
- Extracting `execute()` from `main()` to enable dependency injection was a last-minute refactor that pushed coverage from 79% to 86%.

**What was challenging:**

- Go's `flag` package holds global state — you can't call `flag.Parse()` twice in the same process. Claude refactored `LoadConfig` to use `flag.NewFlagSet` internally and expose a `loadConfigFrom(args []string)` function, making the config layer fully testable without touching `os.Args`.
- Go's explicit type system for JSON response structs is more verbose than Python's duck-typing or dynamic SDKs. The anonymous struct fields inside `chatResponse.Choices` required care to get right in both the implementation and the test mock.

**Surprises:**

- `gpt-4o-mini` handles emotional nuance surprisingly well for a small model. The structured prompt with the full quote list outperformed what I expected — "Just keep swimming" consistently ranks first for perseverance queries even though the word never appears in the query.
- The `//go:build integration` build tag pattern in Go is cleaner than I expected for separating tests that need real credentials from the unit test suite.

## User Engagement Feature

**Resonance feedback loop**

After displaying results, the CLI prompts the user to pick which quote actually resonated:

```
Top 3 quotes for: "I feel like giving up"

1. [0.95] "Just keep swimming." - Dory (Finding Nemo)
2. [0.88] "Get busy living..." - Andy Dufresne (The Shawshank Redemption)
3. [0.82] "The only way out is through." - John Ottway (The Grey)

Which quote resonated most? (1/2/3 or Enter to skip): 1
✓ Saved. Future results will reflect your preference.
```

That selection is persisted locally (`~/.quote-finder/feedback.json`). On future runs, past selections are loaded and injected into the LLM system prompt as personalization context — nudging rankings toward the emotional tones the user has actually responded to, not just the ones the model predicts.

**Why this drives engagement:**

- **Flywheel**: the product gets better the more you use it, so users come back
- **Emotional stickiness**: people return to tools that feel like they _know_ them
- **Data asset**: the preference corpus becomes training signal for PolarSky's own models — see Phase 5 below

**Additional engagement ideas**

- **Social sharing** — After displaying results, generate a plain-text shareable card (quote + movie + a deep link back to the app). People share quotes when one hits them emotionally, giving the product organic growth with zero ad spend. The share itself carries context ("my top result for 'feeling lost'"), making it personal and compelling to click.

- **Friend gifting** — "Send this to someone who needs it" — route a specific quote to a friend via a share link pre-seeded with that quote and an optional message. A prosocial mechanic that's also viral: every gift is an implicit recommendation and a new user acquisition event.

- **Emotional insights** — Monthly digest: _"You've resonated most with perseverance themes this month."_ Aggregates a user's feedback history into a pattern (themes, movies, characters) and surfaces it as a reflection. Turns passive usage into an active self-awareness tool — users return because the digest is a mirror, not just a feature.

## How to Run

There are two ranking modes:

| Mode           | `--mode` value      | Requires API key           | Notes                                               |
| -------------- | ------------------- | -------------------------- | --------------------------------------------------- |
| Llama / Ollama | `llama` _(default)_ | Yes (`LLM_API_KEY=ollama`) | LLM chat completions via local Ollama               |
| Embed          | `embed`             | Yes (`LLM_API_KEY=ollama`) | Dense vector similarity via Ollama `/v1/embeddings` |
| Local          | `local`             | No                         | BM25 lexical ranking, fully offline                 |

### Prerequisites

**Local mode** requires no API key and no external services — just Go.

**Llama mode** requires [Ollama](https://ollama.com) running locally:

```bash
ollama serve          # start the Ollama server
make init-llama       # pull llama3
```

**Embed mode** requires a dedicated embedding model — `nomic-embed-text` is used by default. Pull it once:

```bash
make init-embed   # ollama pull nomic-embed-text
```

Embeddings are supported natively in current Ollama versions with no extra flags needed.

### Customizing the query

The query describes the feeling or situation you want quotes for.

**Option 1 — `--query` flag (no file editing needed):**

```bash
go run . quotes.json --mode local --query "I need courage to face something scary"
```

**Option 2 — edit `quotes.json` directly:**

```json
{
  "query": "I feel lost and don't know what to do",
  "quotes": [ ... ]
}
```

The `--query` flag takes priority over the value in the JSON file.

### Run directly

```bash
# Local mode — no API key, fully offline (BM25 ranking)
go run . quotes.json --mode local
go run . quotes.json --mode local --query "I need courage" --top-n 5

# Llama mode via Ollama (LLM chat completions)
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=llama3 LLM_API_KEY=ollama go run . quotes.json
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=llama3 LLM_API_KEY=ollama go run . quotes.json --query "I feel lost" --top-n 5

# Embed mode via Ollama (dense vector similarity)
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=nomic-embed-text LLM_API_KEY=ollama go run . quotes.json --mode embed
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=nomic-embed-text LLM_API_KEY=ollama go run . quotes.json --mode embed --query "I feel lost"
```

### Make targets

```bash
make run-local                               # local mode — no API key required (BM25)
make run-local QUERY="need courage" TOPN=5  # custom query and result count

make init-llama                              # pull llama3 model (first time only)
make run-llama                               # llama mode, query from quotes.json, top 3
make run-llama QUERY="I feel lost" TOPN=5   # custom query and result count

make init-embed                              # pull nomic-embed-text model (first time only)
make run-embed                               # embed mode, semantic ranking via Ollama embeddings
make run-embed QUERY="I feel lost" TOPN=5   # custom query and result count

make build                                   # compile to ./quote-finder binary
make test                                    # unit tests with coverage report
make vet                                     # go vet

# Integration test — requires Ollama running with llama3 pulled
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=llama3 LLM_API_KEY=ollama go test -tags integration -v ./...
```

### Docker

```bash
# Build
docker build -t quote-finder .

# Local mode — no API key, no network needed
docker run quote-finder quotes.json --mode local

# Llama mode via Ollama
# macOS / Windows: use host.docker.internal to reach Ollama on the host
docker run \
  -e LLM_BASE_URL=http://host.docker.internal:11434/v1 \
  -e LLM_MODEL=llama3 \
  -e LLM_API_KEY=ollama \
  quote-finder quotes.json --mode llama

# Linux: use --network host instead
# docker run --network host -e LLM_BASE_URL=http://localhost:11434/v1 -e LLM_MODEL=llama3 -e LLM_API_KEY=ollama quote-finder quotes.json --mode llama

# Embed mode via Ollama
docker run \
  -e LLM_BASE_URL=http://host.docker.internal:11434/v1 \
  -e LLM_MODEL=nomic-embed-text \
  -e LLM_API_KEY=ollama \
  quote-finder quotes.json --mode embed --query "I just got rejected and feel like giving up"
```

### Expected output

```
Top 3 quotes for: "I need motivation to keep going when things are tough"

1. [0.95] "Just keep swimming." - Dory (Finding Nemo)
2. [0.88] "Get busy living, or get busy dying." - Andy Dufresne (The Shawshank Redemption)
3. [0.82] "The only way out is through." - John Ottway (The Grey)
```

_Scores vary by model and run. Use `--top-n` to control how many results are shown (default: 3)._

## Stretch Goal

`--query` flag is implemented. Usage:

```bash
go run . quotes.json --query "I just got rejected and feel like giving up"
```

**Interesting decision:** I chose to override the query at the CLI layer (replace `input.Query` if `cfg.Query != ""`), not to rank against both queries. The override approach is simpler and matches the expected UX — the CLI flag is an escape hatch for the user, not a secondary input to the model.

## What I'd Do Next: REST Service + Scale

The `Ranker` interface is already service-ready. Here's the roadmap:

**Phase 1 — Lift into a REST server**

Add `server.go` with a single handler wrapping `ranker.Rank()`:

```
POST /rank
Body:     { "query": "...", "quotes": [...] }
Response: { "top3": [...] }
```

`net/http` + goroutines absorb thousands of concurrent connections. The existing `Ranker` interface, `Quote` types, and `Config` load verbatim — zero changes to core logic. Ship behind a load balancer (nginx, AWS ALB) and scale horizontally; each pod is stateless.

**Phase 2 — Cache layer (Redis)**

LLM calls are the expensive operation (~1–3s, ~$0.001 each). Cache by `sha256(query + sorted_quote_ids)`:

```
Request → check Redis key
  HIT:  return cached rankings in <1ms
  MISS: call LLM → cache with TTL (24h) → return
```

Mental wellness apps have high query clustering — users' emotional states follow common patterns ("feeling hopeless", "need motivation"). Cache hit rates will be high, and repeat queries will feel instantaneous.

**Phase 3 — DB-backed quote sets + embeddings**

The `Loader` interface in `loader.go` is the exact swap point for this phase. Today `NewLoader()` returns a `jsonFileLoader` that reads from disk; replacing it with a `dbLoader` (accepting a `*sql.DB` and resolving a quote-set ID) requires no changes to `main.go` or any business logic.

Replace the flat `quotes.json` with Postgres + `pgvector`:

```sql
quote_sets (id, name, created_at)
quotes     (id, set_id, text, movie, character, embedding vector(1536))
```

Pre-compute embeddings at insert time. At query time: vector similarity search for top-N candidates → LLM re-ranks for emotional nuance. This decouples fast retrieval (DB-side) from precise ranking (LLM-side), cutting LLM calls by 80%+ on large quote sets. Seed from open-source datasets for meaningful scale:

- **Cornell Movie Dialogs Corpus** — 220,000+ conversational exchanges from 617 films
- **Kaggle Movie Quotes** — curated collections with metadata (character, genre, year)
- **HuggingFace `movie_quotes`** — ready-to-load datasets with embeddings precomputed for some splits

**Phase 4 — Async + queue for burst traffic**

For spiky load, add a job queue (Redis Streams or SQS):

```
Client → POST /rank  → enqueue → return job_id (202 Accepted)
Client → GET  /rank/{id} → poll result
```

Worker pool pulls from the queue, calls the LLM, writes to Redis. Autoscale workers on queue depth. The LLM provider's rate limit becomes the only ceiling.

| Layer        | Bottleneck                    | Mitigation                         |
| ------------ | ----------------------------- | ---------------------------------- |
| HTTP         | Goroutine memory              | Horizontal pod scaling (stateless) |
| LLM calls    | Provider rate limit + latency | Redis cache + async queue          |
| Quote lookup | DB reads                      | pgvector index + embedding cache   |
| Cold queries | LLM latency (1–3s)            | Pre-warm common queries at startup |

**Phase 5 — User accounts + personalized preference store**

Replace the local `~/.quote-finder/feedback.json` (from the engagement feature above) with a `user_preferences` Postgres table:

```sql
users            (id, email, created_at)
user_preferences (user_id, query, chosen_text, chosen_movie, chosen_character, at)
```

The REST service (Phase 1) authenticates the user, loads their preference history, and injects a personalization hint into the LLM system prompt before each ranking call. Over time, aggregated preference data across all users feeds a fine-tuning pipeline for PolarSky's own models — turning user engagement directly into a proprietary dataset that differentiates PolarSky's emotional relevance from generic LLMs.

| Layer                             | What it enables                                    |
| --------------------------------- | -------------------------------------------------- |
| Local file (CLI, Phase 0)         | Zero-friction MVP — no login, no server            |
| Postgres per-user (REST, Phase 5) | Preferences persist across devices and sessions    |
| Aggregated corpus                 | Fine-tuning signal for PolarSky's inference models |

**Phase 6 — Observability: logging, metrics, and alerting**

A production inference service is only as reliable as its visibility into what's happening. Three layers:

**Structured logging** — replace any `fmt.Fprintf` output with structured JSON logs (using Go's `log/slog` from stdlib, or `zap` for higher throughput). Each ranking request gets a trace ID logged alongside the mode, model, query hash, latency, and whether the result was a cache hit. This makes debugging a slow or wrong ranking tractable without touching the code.

```json
{
  "level": "info",
  "trace_id": "abc123",
  "mode": "llama",
  "model": "llama3",
  "latency_ms": 820,
  "cache_hit": false,
  "top_score": 0.94
}
```

**Metrics** — emit counters and histograms to Prometheus (or push to Datadog/CloudWatch):

| Metric                               | Type      | What it catches                                      |
| ------------------------------------ | --------- | ---------------------------------------------------- |
| `rank_requests_total{mode, status}`  | Counter   | Traffic volume and error rate by mode                |
| `rank_latency_seconds{mode}`         | Histogram | P50/P95/P99 latency per backend                      |
| `llm_errors_total{mode, error_type}` | Counter   | Provider outages, rate limits, hallucinations        |
| `cache_hit_ratio`                    | Gauge     | Redis effectiveness; drops signal cache invalidation |
| `embedding_dim_mismatch_total`       | Counter   | Data quality issues in the embed pipeline            |

**Alerting** — wire Prometheus alerts (or equivalent) to PagerDuty/Slack:

- **Error rate > 1% over 5 min** → page on-call (LLM provider down or bad deploy)
- **P95 latency > 5s** → warn (model overloaded or cold-start spike)
- **Cache hit ratio < 20%** → warn (cache eviction or TTL too short)
- **Zero requests for 10 min during business hours** → warn (silent failure in upstream)

The `Ranker` interface makes this clean to instrument — wrap any backend in a `MetricsRanker` decorator that records latency and error counts before delegating to the real implementation, with no changes to business logic:

```go
type metricsRanker struct{ inner Ranker }

func (m *metricsRanker) Rank(ctx context.Context, query string, quotes []Quote) ([]RankedQuote, error) {
    start := time.Now()
    results, err := m.inner.Rank(ctx, query, quotes)
    recordLatency(time.Since(start), err)
    return results, err
}
```

