package index

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Searcher provides search functionality over an existing Index.
// It wraps the Index methods with nil-guard and offers additional
// search strategies like regex matching and prefix search.
type Searcher struct {
	index *Index
}

// NewSearcher creates a new Searcher backed by the given Index.
// The index must be non-nil for meaningful results (nil-safe methods return zero values).
func NewSearcher(index *Index) *Searcher {
	return &Searcher{index: index}
}

// Search performs a BM25 semantic search over the index.
// It tokenizes the query and returns documents sorted by descending relevance score.
// If the index is nil, it returns nil.
func (s *Searcher) Search(query string, topK int) []DocumentWithScore {
	if s.index == nil {
		return nil
	}
	return s.index.Search(query, topK)
}

// SearchByPath returns a single Document identified by its file path,
// or nil if the path is not indexed.
func (s *Searcher) SearchByPath(path string) *Document {
	if s.index == nil {
		return nil
	}
	return s.index.GetDocument(path)
}

// SearchRegex searches for indexed documents whose tokens match a given regex pattern.
// It scores documents by TF-IDF of matching terms. If topK is 0, all matches are returned.
// Returns nil if the index is nil or the pattern is invalid.
func (s *Searcher) SearchRegex(pattern string, topK int) []DocumentWithScore {
	if s.index == nil {
		return nil
	}

	pattern = "^" + pattern + "$"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	results := make([]DocumentWithScore, 0)
	docs := s.index.GetAllDocuments()

	for _, doc := range docs {
		score := float64(0)
		matchedTokens := 0

		for _, term := range s.index.GetTerms() {
			if re.MatchString(term) {
				docFreq := s.index.docFreqs[term]
				tf := s.index.termFreqs[doc.Path][term]
				if tf > 0 {
					ratio := float64(s.index.totalDocs) / float64(docFreq)
					if ratio > 0 {
						score += float64(tf) * math.Log(ratio)
					}
					matchedTokens++
				}
			}
		}

		if matchedTokens > 0 {
			results = append(results, DocumentWithScore{
				Document:      *doc,
				Score:         score / float64(matchedTokens),
				MatchedTokens: matchedTokens,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// SearchPrefix searches for documents containing terms that start with the given prefix.
// It performs individual BM25 searches for each matching term and deduplicates results.
func (s *Searcher) SearchPrefix(prefix string, topK int) []DocumentWithScore {
	if s.index == nil {
		return nil
	}

	results := make([]DocumentWithScore, 0)
	terms := s.index.GetTerms()

	for _, term := range terms {
		if strings.HasPrefix(term, prefix) {
			query := term
			matches := s.Search(query, 100) // Get all matches for this term

			for _, m := range matches {
				exists := false
				for _, r := range results {
					if r.Path == m.Path {
						exists = true
						break
					}
				}
				if !exists {
					results = append(results, m)
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}
