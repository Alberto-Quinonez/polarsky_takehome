.PHONY: init-llama run-llama init-embed run-embed run-local build test test-integration vet tidy

QUERY ?=
TOPN  ?= 3

init-llama:
	ollama pull llama3

run-llama:
	LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=llama3 LLM_API_KEY=ollama go run . quotes.json --mode llama --top-n $(TOPN) $(if $(QUERY),--query "$(QUERY)",)

init-embed:
	ollama pull nomic-embed-text

run-embed:
	LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=nomic-embed-text LLM_API_KEY=ollama go run . quotes.json --mode embed --top-n $(TOPN) $(if $(QUERY),--query "$(QUERY)",)

run-local:
	go run . quotes.json --mode local --top-n $(TOPN) $(if $(QUERY),--query "$(QUERY)",)

build:
	go build -o quote-finder .

test:
	go test -cover ./...

test-integration:
	go test -tags integration -v ./...

vet:
	go vet ./...

tidy:
	go mod tidy
