package main

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// localRanker ranks quotes using BM25 — no API key required.
// BM25 handles term saturation and document length normalization, making it
// superior to plain TF-IDF for short texts. Fully offline.
type localRanker struct{}

func NewLocalRanker() Ranker {
	return &localRanker{}
}

func (r *localRanker) Rank(_ context.Context, query string, quotes []Quote) ([]RankedQuote, error) {
	queryTokens := tokenize(query)
	corpus := make([][]string, len(quotes))
	for i, q := range quotes {
		corpus[i] = tokenize(q.Text)
	}

	avgLen := avgDocLen(corpus)
	rankings := make([]RankedQuote, len(quotes))
	var maxScore float64
	for i, q := range quotes {
		score := bm25Score(queryTokens, corpus[i], corpus, avgLen)
		rankings[i] = RankedQuote{
			Text:      q.Text,
			Movie:     q.Movie,
			Character: q.Character,
			Score:     score,
		}
		if score > maxScore {
			maxScore = score
		}
	}

	// Normalize to [0, 1].
	if maxScore > 0 {
		for i := range rankings {
			rankings[i].Score /= maxScore
		}
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})

	return rankings, nil
}

// bm25Score computes the BM25 relevance score for doc against queryTokens.
// corpus is used to compute IDF and avgLen.
func bm25Score(queryTokens, doc []string, corpus [][]string, avgLen float64) float64 {
	if len(queryTokens) == 0 || avgLen == 0 {
		return 0
	}
	tf := termFreq(doc)
	N := float64(len(corpus))
	docLen := float64(len(doc))
	var score float64
	for _, term := range queryTokens {
		df := docFreq(term, corpus)
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)
		rawTF := tf[term]
		numerator := rawTF * (bm25K1 + 1)
		denominator := rawTF + bm25K1*(1-bm25B+bm25B*docLen/avgLen)
		score += idf * numerator / denominator
	}
	return score
}

// avgDocLen returns the mean token count per document across the corpus.
func avgDocLen(corpus [][]string) float64 {
	if len(corpus) == 0 {
		return 0
	}
	var total float64
	for _, doc := range corpus {
		total += float64(len(doc))
	}
	return total / float64(len(corpus))
}

// docFreq returns the number of documents in corpus that contain term.
func docFreq(term string, corpus [][]string) float64 {
	var count float64
	for _, doc := range corpus {
		for _, t := range doc {
			if t == term {
				count++
				break
			}
		}
	}
	return count
}

// tokenize lowercases s, splits on non-alphanumeric characters, and removes stopwords.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			if tok := buf.String(); !stopwords[tok] {
				tokens = append(tokens, tok)
			}
			buf.Reset()
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// termFreq builds a word → count map from a token slice.
func termFreq(tokens []string) map[string]float64 {
	tf := make(map[string]float64, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "it": true,
	"in": true, "of": true, "and": true, "to": true, "i": true,
	"my": true, "me": true, "you": true, "we": true, "they": true,
	"this": true, "that": true, "at": true, "be": true, "do": true,
	"has": true, "have": true, "had": true, "was": true, "for": true,
	"on": true, "are": true, "with": true, "as": true, "by": true,
	"or": true, "not": true, "but": true, "so": true, "if": true,
	"up": true, "out": true, "when": true, "what": true, "he": true,
	"she": true, "his": true, "her": true, "its": true, "just": true,
}
