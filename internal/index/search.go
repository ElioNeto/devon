package index

import (
	"math"
	"regexp"
	"strings"
)

// Searcher provides search functionality over indexed documents.
type Searcher struct {
	index *Index
}

// NewSearcher creates a new searcher for the given index.
func NewSearcher(index *Index) *Searcher {
	return &Searcher{index: index}
}

// Search performs a semantic search over the index.
// It returns documents sorted by BM25 relevance score.
func (s *Searcher) Search(query string, topK int) []DocumentWithScore {
	if s.index == nil {
		return nil
	}
	return s.index.Search(query, topK)
}

// SearchByPath returns a document by its path.
func (s *Searcher) SearchByPath(path string) *Document {
	if s.index == nil {
		return nil
	}
	return s.index.GetDocument(path)
}

// SearchRegex searches for documents containing any of the terms in a regex pattern.
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
					score += float64(tf) * mathLog(float64(s.index.totalDocs)/float64(docFreq))
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

	sortResults(results)

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// SearchPrefix searches for documents containing terms that start with a prefix.
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

	sortResults(results)

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results
}

// sortResults sorts results by score descending.
func sortResults(results []DocumentWithScore) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// mathLog computes log(x+1) safely.
func mathLog(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return mathLogUnsafe(x)
}

func mathLogUnsafe(x float64) float64 {
	return math.Log(x + 1)
}
