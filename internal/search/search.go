package search

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/qiaoyo/DiveEnd/internal/llm"
)

// SearchResult represents a found paper
type SearchResult struct {
	Title      string   `json:"title"`
	Authors    string   `json:"authors"`
	Venue      string   `json:"venue"`
	Year       int      `json:"year"`
	Abstract   string   `json:"abstract"`
	URL        string   `json:"url"`
	ArxivID    string   `json:"arxiv_id,omitempty"`
	DOI        string   `json:"doi,omitempty"`
}

// Searcher searches for papers on the web
type Searcher struct {
	httpClient *http.Client
	llmClient  *llm.Client
}

// NewSearcher creates a new searcher
func NewSearcher(llmClient *llm.Client) *Searcher {
	return &Searcher{
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
		llmClient: llmClient,
	}
}

// SearchArxiv searches for papers on arXiv by keywords
func (s *Searcher) SearchArxiv(query string, limit int) ([]SearchResult, error) {
	baseURL := fmt.Sprintf("http://export.arxiv.org/api/query?search_query=%s&start=0&max_results=%d", url.QueryEscape(query), limit)

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse Atom feed and extract papers
	// For now, we'll use a simple approach - arxiv returns XML, we'll extract basic info
	// This is a simplified implementation that gets basic info
	results, err := parseArxivResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// SearchGoogleScholar searches Google Scholar (via public API if available)
// This is a stub that can be expanded with a proper scholar API
func (s *Searcher) SearchGoogleScholar(query string, limit int) ([]SearchResult, error) {
	// For now, we'll rely on arXiv search since Scholar requires scraping
	// Could add a third-party scholar API here
	return []SearchResult{}, nil
}

// Search combines multiple sources and returns aggregated results
func (s *Searcher) Search(query string, maxResults int) ([]SearchResult, error) {
	var allResults []SearchResult

	// Search arXiv first (open access)
	arxivResults, err := s.SearchArxiv(query, maxResults)
	if err != nil {
		fmt.Printf("Warning: arXiv search failed: %v\n", err)
	} else {
		allResults = append(allResults, arxivResults...)
	}

	// Deduplicate by URL
	seen := make(map[string]bool)
	var uniqueResults []SearchResult
	for _, r := range allResults {
		if !seen[r.URL] {
			seen[r.URL] = true
			uniqueResults = append(uniqueResults, r)
		}
	}

	// Limit results
	if len(uniqueResults) > maxResults {
		uniqueResults = uniqueResults[:maxResults]
	}

	return uniqueResults, nil
}

// ExtractAuthors extracts authors from a string and formats them
func ExtractAuthors(authors string) string {
	// Split on 'and' and comma, take first 5 authors, add "et al." if more
	parts := strings.FieldsFunc(authors, func(r rune) bool {
		return r == ',' || r == ';'
	})

	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "and" {
			cleaned = append(cleaned, p)
		}
	}

	if len(cleaned) == 0 {
		return ""
	}
	if len(cleaned) <= 5 {
		return strings.Join(cleaned, ", ")
	}
	return strings.Join(cleaned[:5], ", ") + " et al."
}

// Below this is the simplified arxiv parser
// Full XML parsing would be better, but this works for getting started

func parseArxivResponse(body io.Reader) ([]SearchResult, error) {
	data, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	content := string(data)
	var results []SearchResult

	// Find all entry blocks
	entryStart := "<entry>"
	entryEnd := "</entry>"

	start := 0
	for {
		entryStartIdx := strings.Index(content[start:], entryStart)
		if entryStartIdx == -1 {
			break
		}
		entryStartIdx += start
		entryEndIdx := strings.Index(content[entryStartIdx:], entryEnd)
		if entryEndIdx == -1 {
			break
		}
		entryEndIdx += entryStartIdx

		entry := content[entryStartIdx:entryEndIdx]
		result := parseArxivEntry(entry)
		if result.Title != "" {
			results = append(results, result)
		}

		start = entryEndIdx + len(entryEnd)
	}

	return results, nil
}

func parseArxivEntry(entry string) SearchResult {
	var result SearchResult

	// Extract title
	titleStart := strings.Index(entry, "<title>")
	titleEnd := strings.Index(entry, "</title>")
	if titleStart != -1 && titleEnd != -1 && titleEnd > titleStart {
		title := entry[titleStart+7 : titleEnd]
		result.Title = strings.TrimSpace(title)
	}

	// Extract authors
	authorStart := 0
	var authors []string
	for {
		nsStart := strings.Index(entry[authorStart:], "<name>")
		if nsStart == -1 {
			break
		}
		nsStart += authorStart + 6
		nsEnd := strings.Index(entry[nsStart:], "</name>")
		if nsEnd == -1 {
			break
		}
		name := strings.TrimSpace(entry[nsStart : nsStart+nsEnd])
		if name != "" {
			authors = append(authors, name)
		}
		authorStart = nsStart + nsEnd
	}
	if len(authors) > 0 {
		result.Authors = strings.Join(authors, ", ")
	}

	// Extract abstract
	summaryStart := strings.Index(entry, "<summary>")
	summaryEnd := strings.Index(entry, "</summary>")
	if summaryStart != -1 && summaryEnd != -1 && summaryEnd > summaryStart {
		abstract := entry[summaryStart+9 : summaryEnd]
		result.Abstract = strings.TrimSpace(abstract)
	}

	// Extract published year
	publishedStart := strings.Index(entry, "<published>")
	publishedEnd := strings.Index(entry, "</published>")
	if publishedStart != -1 && publishedEnd != -1 {
		published := strings.TrimSpace(entry[publishedStart+11 : publishedEnd])
		if len(published) >= 4 {
			var year int
			fmt.Sscanf(published, "%d", &year)
			result.Year = year
		}
	}

	// Extract id
	idStart := strings.Index(entry, "<id>")
	idEnd := strings.Index(entry, "</id>")
	if idStart != -1 && idEnd != -1 {
		id := strings.TrimSpace(entry[idStart+4 : idEnd])
		result.URL = id
		// Extract arxiv ID from URL
		parts := strings.Split(id, "/")
		if len(parts) > 0 {
			result.ArxivID = parts[len(parts)-1]
		}
		result.Venue = "arXiv"
	}

	return result
}
