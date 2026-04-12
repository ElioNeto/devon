// Package index provides semantic indexing and search capabilities using TF-IDF and BM25.
//
// The index uses a term frequency-inverse document frequency scoring algorithm
// to find relevant files based on search queries. It's implemented in pure Go
// with no external dependencies beyond the SQLite database.
package index

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Token represents a single token from text.
type Token struct {
	Text     string // normalized token text
	Offset   int    // original character offset
	Position int    // token position in document
}

// Document represents a document in the index.
type Document struct {
	ID       string    // unique document ID (file path)
	Path     string    // file path relative to workspace
	Tokens   []Token   // tokens in the document
	Length   int       // number of tokens
	WordLen  float64   // effective length (after stopping)
	Modified int64     // last modified timestamp (Unix epoch)
}

// DocumentWithScore is a document with its BM25 relevance score.
type DocumentWithScore struct {
	Document
	Score         float64
	MatchedTokens int
}

// Index represents a searchable TF-IDF index.
type Index struct {
	mu         sync.RWMutex
	documents  map[string]*Document      // id -> doc
	docIDByPath map[string]string        // path -> doc ID
	termFreqs  map[string]map[string]int // term -> docID -> freq
	docFreqs   map[string]int            // term -> number of docs containing term
	totalDocs  int
	corpusLen  float64 // sum of all doc lengths
}

// NewIndex creates a new empty index.
func NewIndex() *Index {
	return &Index{
		documents:   make(map[string]*Document),
		docIDByPath: make(map[string]string),
		termFreqs:   make(map[string]map[string]int),
		docFreqs:    make(map[string]int),
	}
}

// Tokenizer splits text into tokens.
type Tokenizer struct {
	minLength       int
	stopwords       map[string]bool
	caseSensitive   bool
}

// Stopwords for common languages (English + Portuguese).
var commonStopwords = map[string]bool{
	// English
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
	"are": true, "were": true, "been": true, "be": true, "have": true, "has": true,
	"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	"it": true, "its": true, "this": true, "that": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true, "we": true, "they": true,
	"what": true, "which": true, "who": true, "where": true, "when": true, "why": true, "how": true,
	// Portuguese
	"o": true, "a": true, "os": true, "as": true, "de": true, "do": true, "da": true,
	"dos": true, "das": true, "em": true, "no": true, "na": true, "nos": true, "nas": true,
	"para": true, "por": true, "com": true, "sem": true, "sobre": true, "entre": true,
	"mais": true, "menos": true, "mas": true, "ou": true, "que": true, "quem": true,
	"quando": true, "onde": true, "como": true, "porque": true,
}

// NewTokenizer creates a new tokenizer with default settings.
func NewTokenizer() *Tokenizer {
	return &Tokenizer{
		minLength:     2,
		stopwords:     commonStopwords,
		caseSensitive: false,
	}
}

// WithMinLength sets minimum token length.
func (t *Tokenizer) WithMinLength(n int) *Tokenizer {
	t.minLength = n
	return t
}

// WithStopwords sets custom stopwords.
func (t *Tokenizer) WithStopwords(sw map[string]bool) *Tokenizer {
	t.stopwords = sw
	return t
}

// Tokenize splits text into tokens.
func (t *Tokenizer) Tokenize(text string) []Token {
	var tokens []Token
	var (
		start = 0
		pos   = 0
	)

	for i, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if i > start {
				word := text[start:i]
				if t.shouldKeep(word) {
					tokens = append(tokens, Token{
						Text:     t.normalize(word),
						Offset:   start,
						Position: pos,
					})
					pos++
				}
			}
			start = i + 1
			continue
		}
		// Handle end of string
		if i == len(text)-1 && !isWordChar(r) {
			if i > start {
				word := text[start:]
				if t.shouldKeep(word) {
					tokens = append(tokens, Token{
						Text:     t.normalize(word),
						Offset:   start,
						Position: pos,
					})
				}
			}
		}
	}

	// Handle last token if string ends with word char
	if start < len(text) {
		word := text[start:]
		if t.shouldKeep(word) {
			tokens = append(tokens, Token{
				Text:     t.normalize(word),
				Offset:   start,
				Position: pos,
			})
		}
	}

	return tokens
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (t *Tokenizer) shouldKeep(word string) bool {
	if len(word) < t.minLength {
		return false
	}
	normalized := t.normalize(word)
	_, isStopword := t.stopwords[normalized]
	return !isStopword
}

func (t *Tokenizer) normalize(word string) string {
	if !t.caseSensitive {
		return strings.ToLower(word)
	}
	return word
}

// BM25Calculator computes BM25 relevance scores.
type BM25Calculator struct {
	K1 float64 // term frequency saturation parameter
	B  float64 // length normalization parameter
}

// NewBM25Calculator creates a BM25 calculator with standard parameters.
func NewBM25Calculator(k1, b float64) *BM25Calculator {
	return &BM25Calculator{K1: k1, B: b}
}

// ScoreTerm computes the BM25 score for a single term in a document.
func (b *BM25Calculator) ScoreTerm(tf, docFreq, totalDocs, avgDocLen, docLen float64) float64 {
	if docFreq == 0 || totalDocs == 0 {
		return 0
	}

	// IDF component (with smoothing)
	idf := math.Log(float64(totalDocs)/docFreq)

	// TF component with saturation
	tfScore := (tf * (b.K1 + 1)) / (tf + b.K1*(1-b+b*docLen/avgDocLen))

	return tfScore * idf
}

// Index adds a document to the index.
func (i *Index) Index(doc *Document) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Remove old version if exists
	if oldID, exists := i.docIDByPath[doc.Path]; exists {
		i.removeDocument(oldID)
	}

	// Calculate word length (excluding stop words)
	var wordLen float64
	for _, tok := range doc.Tokens {
		if !i.isStopword(tok.Text) {
			wordLen++
		}
	}
	doc.WordLen = wordLen
	doc.Length = len(doc.Tokens)

	// Add to index
	docID := doc.Path // Use path as unique ID
	i.documents[docID] = doc
	i.docIDByPath[doc.Path] = docID

	// Update term frequencies
	if _, exists := i.termFreqs[docID]; !exists {
		i.termFreqs[docID] = make(map[string]int)
	}

	for _, tok := range doc.Tokens {
		// Skip stopwords in index
		if i.isStopword(tok.Text) {
			continue
		}

		i.termFreqs[docID][tok.Text]++

		// Update document frequency
		if _, exists := i.docFreqs[tok.Text]; !exists {
			i.docFreqs[tok.Text] = 0
		}
		i.docFreqs[tok.Text]++
	}

	i.totalDocs++
	i.corpusLen += wordLen
}

// RemoveDocument removes a document from the index.
func (i *Index) RemoveDocument(docID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.removeDocument(docID)
}

// removeDocument removes a document from the index (internal, lock assumed).
func (i *Index) removeDocument(docID string) {
	doc, exists := i.documents[docID]
	if !exists {
		return
	}

	// Remove from term frequencies
	for term := range i.termFreqs[docID] {
		i.docFreqs[term]--
		if i.docFreqs[term] == 0 {
			delete(i.docFreqs, term)
		}
	}

	delete(i.termFreqs, docID)
	delete(i.documents, docID)
	delete(i.docIDByPath, doc.Path)
	i.totalDocs--
	i.corpusLen -= doc.WordLen
}

// isStopword checks if a term is a stopword.
func (i *Index) isStopword(term string) bool {
	_, ok := commonStopwords[term]
	return ok
}

// Search queries the index and returns top-K documents by BM25 score.
func (i *Index) Search(query string, topK int) []DocumentWithScore {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Tokenize query
	tokenizer := NewTokenizer()
	queryTokens := tokenizer.Tokenize(query)

	// Calculate average document length
	var avgDocLen float64
	if i.totalDocs > 0 {
		avgDocLen = i.corpusLen / float64(i.totalDocs)
	}
	if avgDocLen == 0 {
		avgDocLen = 1 // Avoid division by zero
	}

	// BM25 calculator with standard parameters
	bm25 := NewBM25Calculator(1.2, 0.75)

	// Score documents
	scores := make(map[string]float64)
	matches := make(map[string]int)

	for _, queryToken := range queryTokens {
		term := queryToken.Text

		// Document frequency for IDF
		docFreq, exists := i.docFreqs[term]
		if !exists {
			continue
		}

		// Score each document containing this term
		for docID, tf := range i.termFreqs[docID] {
			score := bm25.ScoreTerm(
				float64(tf),        // term frequency in doc
				float64(docFreq),   // document frequency
				float64(i.totalDocs), // total documents
				avgDocLen,          // average document length
				i.documents[docID].WordLen, // actual document length
			)
			scores[docID] += score
			matches[docID]++
		}
	}

	// Sort by score
	result := make([]DocumentWithScore, 0, len(scores))
	for docID, score := range scores {
		doc, exists := i.documents[docID]
		if !exists {
			continue
		}
		result = append(result, DocumentWithScore{
			Document:      *doc,
			Score:         score,
			MatchedTokens: matches[docID],
		})
	}

	// Sort descending by score
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	// Return top-K
	if len(result) > topK {
		result = result[:topK]
	}

	return result
}

// GetDocument returns a document by path, or nil if not found.
func (i *Index) GetDocument(path string) *Document {
	i.mu.RLock()
	defer i.mu.RUnlock()

	docID, exists := i.docIDByPath[path]
	if !exists {
		return nil
	}

	return i.documents[docID]
}

// GetAllDocuments returns all documents in the index.
func (i *Index) GetAllDocuments() []*Document {
	i.mu.RLock()
	defer i.mu.RUnlock()

	docs := make([]*Document, 0, len(i.documents))
	for _, doc := range i.documents {
		docs = append(docs, doc)
	}
	return docs
}

// TotalDocs returns the number of documents in the index.
func (i *Index) TotalDocs() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.totalDocs
}

// TermCount returns the number of unique terms in the index.
func (i *Index) TermCount() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.docFreqs)
}

// GetTerms returns all unique terms in the index.
func (i *Index) GetTerms() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	terms := make([]string, 0, len(i.docFreqs))
	for term := range i.docFreqs {
		terms = append(terms, term)
	}
	return terms
}

// BuildRegex creates a regex pattern from terms.
func BuildRegex(terms []string) string {
	if len(terms) == 0 {
		return ""
	}

	// Create alternation pattern with word boundaries
	patterns := make([]string, len(terms))
	for i, term := range terms {
		// Escape special regex characters
		escaped := term
		specialChars := []string{`.`, `^`, `$`, `*`, `+`, `?`, `(`, `)`, `[`, `]`, `{`, `}`, `|`, `\`}
		for _, ch := range specialChars {
			escaped = strings.ReplaceAll(escaped, ch, `\"+ch)
		}
		patterns[i] = `\b` + escaped + `\b`
	}

	return strings.Join(patterns, "|")
}
