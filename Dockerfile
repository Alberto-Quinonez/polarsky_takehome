FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o quote-finder .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/quote-finder .
COPY quotes.json .
ENTRYPOINT ["./quote-finder"]
CMD ["quotes.json"]
