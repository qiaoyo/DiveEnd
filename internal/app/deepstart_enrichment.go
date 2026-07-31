package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type deepStartEnricher interface {
	EnrichPapers(
		ctx context.Context,
		query string,
		papers []SearchPaper,
		onProgress func(completed int, total int, etaSeconds int, message string),
	) ([]SearchPaper, error)
}

type DeepStartEnricher struct {
	db              *DB
	httpClient      *http.Client
	openAlexBaseURL string
	crossrefBaseURL string
}

type deepStartPublicationMetadata struct {
	Venue         string
	Year          int
	CitationCount int
	PDFURLs       []string
}

func NewDeepStartEnricher(db *DB) *DeepStartEnricher {
	return &DeepStartEnricher{
		db:              db,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		openAlexBaseURL: "https://api.openalex.org/works",
		crossrefBaseURL: "https://api.crossref.org/works",
	}
}

func (e *DeepStartEnricher) EnrichPapers(
	ctx context.Context,
	query string,
	papers []SearchPaper,
	onProgress func(completed int, total int, etaSeconds int, message string),
) ([]SearchPaper, error) {
	if len(papers) == 0 {
		return []SearchPaper{}, nil
	}

	enriched := make([]SearchPaper, len(papers))
	copy(enriched, papers)

	startedAt := time.Now()
	total := len(enriched)
	for index := range enriched {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		paper, message := e.enrichSinglePaper(ctx, query, enriched[index])
		enriched[index] = paper

		completed := index + 1
		etaSeconds := 0
		if completed > 0 && completed < total {
			avg := time.Since(startedAt) / time.Duration(completed)
			etaSeconds = int((avg * time.Duration(total-completed)).Seconds())
			if etaSeconds < 0 {
				etaSeconds = 0
			}
		}

		if onProgress != nil {
			onProgress(completed, total, etaSeconds, message)
		}
	}

	return enriched, nil
}

func (e *DeepStartEnricher) enrichSinglePaper(ctx context.Context, query string, paper SearchPaper) (SearchPaper, string) {
	paper.SourceLabel = resolveSearchSourceLabel(paper)
	if paper.PublicationYear <= 0 {
		paper.PublicationYear = paper.Year
	}
	if strings.TrimSpace(paper.PublicationVenue) == "" {
		paper.PublicationVenue = strings.TrimSpace(paper.Journal)
	}
	if strings.TrimSpace(paper.Journal) == "" && strings.TrimSpace(paper.PublicationVenue) != "" {
		paper.Journal = strings.TrimSpace(paper.PublicationVenue)
	}
	if shouldSkipExternalEnrichment(paper) {
		paper.Keywords = buildPaperKeywords(paper, query, paper.Keywords)
		paper.EnrichmentNote = ""
		return paper, "本地补全完成"
	}

	cacheKey := deepStartEnrichmentCacheKey(paper)
	if e.db != nil && cacheKey != "" {
		if cached, err := e.db.GetDeepStartEnrichmentCache(cacheKey); err == nil && cached != nil {
			paper.Institutions = normalizeInstitutionList(cached.Institutions)
			paper.Keywords = buildPaperKeywords(paper, query, cached.Keywords)
			if strings.TrimSpace(cached.SourceLabel) != "" {
				paper.SourceLabel = strings.TrimSpace(cached.SourceLabel)
			}
			if strings.TrimSpace(cached.PublicationVenue) != "" {
				paper.PublicationVenue = strings.TrimSpace(cached.PublicationVenue)
				if strings.TrimSpace(paper.Journal) == "" {
					paper.Journal = paper.PublicationVenue
				}
			}
			if cached.PublicationYear > 0 {
				paper.PublicationYear = cached.PublicationYear
				if paper.Year <= 0 {
					paper.Year = cached.PublicationYear
				}
			}
			if cached.CitationCount > 0 {
				paper.CitationCount = cached.CitationCount
			}
			paper.EnrichmentNote = strings.TrimSpace(cached.ErrorMessage)
			return paper, "使用缓存补全"
		}
	}

	institutions := normalizeInstitutionList(paper.Institutions)
	keywords := buildPaperKeywords(paper, query, paper.Keywords)
	openAlexAttempted := false
	crossrefAttempted := false
	var errorParts []string

	publication := deepStartPublicationMetadata{
		Venue:         strings.TrimSpace(paper.PublicationVenue),
		Year:          paper.PublicationYear,
		CitationCount: paper.CitationCount,
	}

	openAlexInstitutions, openAlexKeywords, openAlexPublication, openAlexErr := e.fetchOpenAlexMetadata(ctx, paper)
	openAlexAttempted = true
	if openAlexErr != nil {
		errorParts = append(errorParts, "OpenAlex: "+openAlexErr.Error())
	}
	if len(institutions) == 0 {
		institutions = normalizeInstitutionList(openAlexInstitutions)
	}
	keywords = buildPaperKeywords(paper, query, append(keywords, openAlexKeywords...))
	publication = mergePublicationMetadata(publication, openAlexPublication)

	crossrefInstitutions, crossrefKeywords, crossrefPublication, crossrefErr := e.fetchCrossrefMetadata(ctx, paper)
	crossrefAttempted = true
	if crossrefErr != nil {
		errorParts = append(errorParts, "Crossref: "+crossrefErr.Error())
	}
	if len(institutions) == 0 {
		institutions = normalizeInstitutionList(crossrefInstitutions)
	}
	keywords = buildPaperKeywords(paper, query, append(keywords, crossrefKeywords...))
	publication = mergePublicationMetadata(publication, crossrefPublication)

	errorMessage := strings.Join(errorParts, "; ")
	if len(institutions) == 0 {
		errorMessage = strings.TrimSpace(strings.Trim(errorMessage+"; 机构未提供", "; "))
	}

	paper.Institutions = institutions
	paper.Keywords = keywords
	if len(publication.PDFURLs) > 0 {
		paper.PDFCandidates = uniqueStrings(append(paper.PDFCandidates, publication.PDFURLs...))
	}
	paper.PublicationVenue = strings.TrimSpace(publication.Venue)
	paper.PublicationYear = publication.Year
	if paper.PublicationYear > 0 {
		paper.Year = paper.PublicationYear
	}
	if strings.TrimSpace(paper.PublicationVenue) != "" {
		paper.Journal = strings.TrimSpace(paper.PublicationVenue)
	}
	if publication.CitationCount > 0 {
		paper.CitationCount = publication.CitationCount
	}
	paper.SourceLabel = resolveSearchSourceLabel(paper)
	paper.EnrichmentNote = errorMessage

	if e.db != nil && cacheKey != "" {
		cacheEntry := &DeepStartEnrichmentCache{
			CacheKey:          cacheKey,
			Institutions:      institutions,
			Keywords:          keywords,
			SourceLabel:       paper.SourceLabel,
			PublicationVenue:  paper.PublicationVenue,
			PublicationYear:   paper.PublicationYear,
			CitationCount:     paper.CitationCount,
			OpenAlexAttempted: openAlexAttempted,
			CrossrefAttempted: crossrefAttempted,
			ErrorMessage:      paper.EnrichmentNote,
			UpdatedAt:         time.Now(),
		}
		_ = e.db.UpsertDeepStartEnrichmentCache(cacheEntry)
	}

	if len(institutions) == 0 {
		return paper, "机构未提供"
	}
	return paper, "机构补全完成"
}

func (e *DeepStartEnricher) fetchOpenAlexMetadata(ctx context.Context, paper SearchPaper) ([]string, []string, deepStartPublicationMetadata, error) {
	query := strings.TrimSpace(paper.Title)
	if query == "" {
		return nil, nil, deepStartPublicationMetadata{}, fmt.Errorf("missing title")
	}

	values := url.Values{}
	values.Set("search", query)
	values.Set("per-page", "5")
	values.Set("mailto", "diveend@local.invalid")
	endpoint := e.openAlexBaseURL + "?" + values.Encode()

	body, err := e.getJSON(ctx, endpoint, "")
	if err != nil {
		return nil, nil, deepStartPublicationMetadata{}, err
	}

	var result struct {
		Results []struct {
			Title           string `json:"title"`
			PublicationYear int    `json:"publication_year"`
			CitedByCount    int    `json:"cited_by_count"`
			HostVenue       *struct {
				DisplayName string `json:"display_name"`
			} `json:"host_venue"`
			PrimaryLocation *struct {
				Source *struct {
					DisplayName string `json:"display_name"`
				} `json:"source"`
			} `json:"primary_location"`
			Authorships []struct {
				Institutions []struct {
					DisplayName string `json:"display_name"`
				} `json:"institutions"`
			} `json:"authorships"`
			Concepts []struct {
				DisplayName string  `json:"display_name"`
				Score       float64 `json:"score"`
			} `json:"concepts"`
			BestOALocation *struct {
				PDFURL         string `json:"pdf_url"`
				LandingPageURL string `json:"landing_page_url"`
			} `json:"best_oa_location"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, deepStartPublicationMetadata{}, err
	}
	if len(result.Results) == 0 {
		return nil, nil, deepStartPublicationMetadata{}, fmt.Errorf("empty results")
	}

	best := chooseOpenAlexResult(result.Results, paper)
	institutions := make([]string, 0, 4)
	for _, authorship := range best.Authorships {
		for _, institution := range authorship.Institutions {
			if name := strings.TrimSpace(institution.DisplayName); name != "" {
				institutions = append(institutions, name)
			}
		}
	}

	keywords := make([]string, 0, 6)
	for _, concept := range best.Concepts {
		if concept.Score < 0.3 {
			continue
		}
		if keyword := strings.TrimSpace(concept.DisplayName); keyword != "" {
			keywords = append(keywords, keyword)
		}
		if len(keywords) >= 6 {
			break
		}
	}

	venue := ""
	if best.PrimaryLocation != nil && best.PrimaryLocation.Source != nil {
		venue = strings.TrimSpace(best.PrimaryLocation.Source.DisplayName)
	}
	if venue == "" && best.HostVenue != nil {
		venue = strings.TrimSpace(best.HostVenue.DisplayName)
	}
	publication := deepStartPublicationMetadata{
		Venue:         venue,
		Year:          best.PublicationYear,
		CitationCount: best.CitedByCount,
	}
	if best.BestOALocation != nil {
		if pdfURL := strings.TrimSpace(best.BestOALocation.PDFURL); pdfURL != "" {
			publication.PDFURLs = append(publication.PDFURLs, pdfURL)
		}
		if landingURL := strings.TrimSpace(best.BestOALocation.LandingPageURL); landingURL != "" {
			publication.PDFURLs = append(publication.PDFURLs, landingURL)
		}
	}
	publication.PDFURLs = uniqueStrings(publication.PDFURLs)

	return normalizeInstitutionList(institutions), uniqueStrings(keywords), publication, nil
}

func chooseOpenAlexResult(results []struct {
	Title           string `json:"title"`
	PublicationYear int    `json:"publication_year"`
	CitedByCount    int    `json:"cited_by_count"`
	HostVenue       *struct {
		DisplayName string `json:"display_name"`
	} `json:"host_venue"`
	PrimaryLocation *struct {
		Source *struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
	} `json:"primary_location"`
	Authorships []struct {
		Institutions []struct {
			DisplayName string `json:"display_name"`
		} `json:"institutions"`
	} `json:"authorships"`
	Concepts []struct {
		DisplayName string  `json:"display_name"`
		Score       float64 `json:"score"`
	} `json:"concepts"`
	BestOALocation *struct {
		PDFURL         string `json:"pdf_url"`
		LandingPageURL string `json:"landing_page_url"`
	} `json:"best_oa_location"`
}, paper SearchPaper) struct {
	Title           string `json:"title"`
	PublicationYear int    `json:"publication_year"`
	CitedByCount    int    `json:"cited_by_count"`
	HostVenue       *struct {
		DisplayName string `json:"display_name"`
	} `json:"host_venue"`
	PrimaryLocation *struct {
		Source *struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
	} `json:"primary_location"`
	Authorships []struct {
		Institutions []struct {
			DisplayName string `json:"display_name"`
		} `json:"institutions"`
	} `json:"authorships"`
	Concepts []struct {
		DisplayName string  `json:"display_name"`
		Score       float64 `json:"score"`
	} `json:"concepts"`
	BestOALocation *struct {
		PDFURL         string `json:"pdf_url"`
		LandingPageURL string `json:"landing_page_url"`
	} `json:"best_oa_location"`
} {
	target := normalizedDedupeToken(paper.Title)
	best := results[0]
	bestScore := openAlexMatchScore(best.Title, best.PublicationYear, target, paper.Year)

	for _, candidate := range results[1:] {
		score := openAlexMatchScore(candidate.Title, candidate.PublicationYear, target, paper.Year)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	return best
}

func openAlexMatchScore(title string, year int, targetTitle string, targetYear int) int {
	score := 0
	titleNorm := normalizedDedupeToken(title)
	if titleNorm == targetTitle {
		score += 100
	} else if strings.Contains(titleNorm, targetTitle) || strings.Contains(targetTitle, titleNorm) {
		score += 60
	}
	if targetYear > 0 && year == targetYear {
		score += 20
	}
	return score
}

func (e *DeepStartEnricher) fetchCrossrefMetadata(ctx context.Context, paper SearchPaper) ([]string, []string, deepStartPublicationMetadata, error) {
	query := strings.TrimSpace(paper.Title)
	if query == "" {
		return nil, nil, deepStartPublicationMetadata{}, fmt.Errorf("missing title")
	}

	values := url.Values{}
	values.Set("query.title", query)
	values.Set("rows", "5")
	endpoint := e.crossrefBaseURL + "?" + values.Encode()

	body, err := e.getJSON(ctx, endpoint, "application/json")
	if err != nil {
		return nil, nil, deepStartPublicationMetadata{}, err
	}

	var result struct {
		Message struct {
			Items []struct {
				Title               []string `json:"title"`
				Subject             []string `json:"subject"`
				ContainerTitle      []string `json:"container-title"`
				IsReferencedByCount int      `json:"is-referenced-by-count"`
				Author              []struct {
					Affiliation []struct {
						Name string `json:"name"`
					} `json:"affiliation"`
				} `json:"author"`
				Issued struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"issued"`
			} `json:"items"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, deepStartPublicationMetadata{}, err
	}
	if len(result.Message.Items) == 0 {
		return nil, nil, deepStartPublicationMetadata{}, fmt.Errorf("empty results")
	}

	best := chooseCrossrefItem(result.Message.Items, paper)
	institutions := make([]string, 0, 4)
	for _, author := range best.Author {
		for _, affiliation := range author.Affiliation {
			if name := strings.TrimSpace(affiliation.Name); name != "" {
				institutions = append(institutions, name)
			}
		}
	}

	keywords := make([]string, 0, len(best.Subject))
	for _, subject := range best.Subject {
		if subject = strings.TrimSpace(subject); subject != "" {
			keywords = append(keywords, subject)
		}
	}

	publicationVenue := ""
	if len(best.ContainerTitle) > 0 {
		publicationVenue = strings.TrimSpace(best.ContainerTitle[0])
	}
	publication := deepStartPublicationMetadata{
		Venue:         publicationVenue,
		Year:          extractCrossrefYear(best.Issued.DateParts),
		CitationCount: best.IsReferencedByCount,
	}

	return normalizeInstitutionList(institutions), uniqueStrings(keywords), publication, nil
}

func chooseCrossrefItem(items []struct {
	Title               []string `json:"title"`
	Subject             []string `json:"subject"`
	ContainerTitle      []string `json:"container-title"`
	IsReferencedByCount int      `json:"is-referenced-by-count"`
	Author              []struct {
		Affiliation []struct {
			Name string `json:"name"`
		} `json:"affiliation"`
	} `json:"author"`
	Issued struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
}, paper SearchPaper) struct {
	Title               []string `json:"title"`
	Subject             []string `json:"subject"`
	ContainerTitle      []string `json:"container-title"`
	IsReferencedByCount int      `json:"is-referenced-by-count"`
	Author              []struct {
		Affiliation []struct {
			Name string `json:"name"`
		} `json:"affiliation"`
	} `json:"author"`
	Issued struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
} {
	target := normalizedDedupeToken(paper.Title)
	best := items[0]
	bestScore := crossrefMatchScore(best.Title, extractCrossrefYear(best.Issued.DateParts), target, paper.Year)

	for _, candidate := range items[1:] {
		score := crossrefMatchScore(candidate.Title, extractCrossrefYear(candidate.Issued.DateParts), target, paper.Year)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	return best
}

func crossrefMatchScore(titles []string, year int, targetTitle string, targetYear int) int {
	title := ""
	if len(titles) > 0 {
		title = titles[0]
	}
	titleNorm := normalizedDedupeToken(title)
	score := 0
	if titleNorm == targetTitle {
		score += 100
	} else if strings.Contains(titleNorm, targetTitle) || strings.Contains(targetTitle, titleNorm) {
		score += 60
	}
	if targetYear > 0 && year == targetYear {
		score += 20
	}
	return score
}

func extractCrossrefYear(dateParts [][]int) int {
	if len(dateParts) == 0 || len(dateParts[0]) == 0 {
		return 0
	}
	return dateParts[0][0]
}

func (e *DeepStartEnricher) getJSON(ctx context.Context, endpoint string, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %s", redactURLQueryValuesInText(err.Error()))
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("User-Agent", "DiveEnd/1.0 (+https://github.com/diveend)")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metadata request failed: %s", redactURLQueryValuesInText(err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return readExternalHTTPBody(resp.Body)
}

func mergePublicationMetadata(base deepStartPublicationMetadata, candidate deepStartPublicationMetadata) deepStartPublicationMetadata {
	if strings.TrimSpace(base.Venue) == "" && strings.TrimSpace(candidate.Venue) != "" {
		base.Venue = strings.TrimSpace(candidate.Venue)
	}
	if base.Year <= 0 && candidate.Year > 0 {
		base.Year = candidate.Year
	}
	if candidate.CitationCount > base.CitationCount {
		base.CitationCount = candidate.CitationCount
	}
	if len(candidate.PDFURLs) > 0 {
		base.PDFURLs = uniqueStrings(append(base.PDFURLs, candidate.PDFURLs...))
	}
	return base
}

func buildPaperKeywords(paper SearchPaper, query string, candidates []string) []string {
	keywords := make([]string, 0, len(candidates)+len(paper.Tags))
	keywords = append(keywords, candidates...)
	keywords = append(keywords, paper.Tags...)
	keywords = append(keywords, paper.Keywords...)

	if len(keywords) == 0 {
		keywords = append(keywords, extractKeywordsFromText(strings.Join([]string{paper.Title, paper.Abstract}, " "))...)
	}
	return uniqueStrings(trimKeywordList(keywords, query, 10))
}

func trimKeywordList(values []string, query string, limit int) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if shouldSkipKeyword(value, query) {
			continue
		}
		clean = append(clean, value)
		if limit > 0 && len(clean) >= limit {
			break
		}
	}
	return clean
}

func shouldSkipKeyword(value, query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return true
	}
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	if normalizedQuery != "" {
		if normalized == normalizedQuery {
			return true
		}
		if len([]rune(normalizedQuery)) > 12 && strings.Contains(normalized, normalizedQuery) {
			return true
		}
	}
	if len([]rune(normalized)) > 64 {
		return true
	}
	if strings.ContainsAny(normalized, "。！？!?,，；;：:") && len([]rune(normalized)) > 12 {
		return true
	}
	if strings.Count(normalized, " ") >= 4 {
		return true
	}

	intentHints := []string{
		"我想", "希望", "梳理", "这两年", "领域", "论文", "帮我", "我要",
		"i want", "help me", "sort out", "papers in the field", "over the past two years",
	}
	for _, hint := range intentHints {
		if strings.Contains(normalized, hint) && len([]rune(normalized)) > 8 {
			return true
		}
	}
	return false
}

func extractKeywordsFromText(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	keywords := make([]string, 0, 12)
	for _, token := range words {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, stop := searchEnglishStopWords[token]; stop {
			continue
		}
		if len([]rune(token)) < 3 {
			continue
		}
		keywords = append(keywords, token)
		if len(keywords) >= 12 {
			break
		}
	}
	return uniqueStrings(keywords)
}

func resolveSearchSourceLabel(paper SearchPaper) string {
	if label := strings.TrimSpace(paper.SourceLabel); label != "" {
		return label
	}

	switch strings.TrimSpace(strings.ToLower(paper.Source)) {
	case "semantic_scholar":
		return "Semantic Scholar"
	case "arxiv":
		return "arXiv"
	case "arxiv_sanity":
		return "arXiv Sanity Lite"
	default:
		if strings.TrimSpace(paper.Journal) != "" {
			return strings.TrimSpace(paper.Journal)
		}
		return "Unknown Source"
	}
}

func normalizeInstitutionList(values []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		clean = append(clean, value)
	}
	return uniqueStrings(clean)
}

func deepStartEnrichmentCacheKey(paper SearchPaper) string {
	title := normalizedDedupeToken(paper.Title)
	if title == "" {
		return ""
	}
	if paper.Year > 0 {
		return fmt.Sprintf("%s|%d", title, paper.Year)
	}
	return title
}

func shouldSkipExternalEnrichment(paper SearchPaper) bool {
	source := strings.TrimSpace(strings.ToLower(paper.Source))
	switch source {
	case "semantic_scholar", "arxiv", "arxiv_sanity":
		return false
	default:
		return true
	}
}
