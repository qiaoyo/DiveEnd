package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)
var arxivVersionSuffixPattern = regexp.MustCompile(`(?i)v\d+$`)

func (s *SearchClient) searchOpenAlex(
	ctx context.Context,
	query string,
	limit int,
	onAttempt func(string, int, bool, bool, int, error),
) ([]SearchPaper, error) {
	candidates := buildSearchQueryCandidates(query)
	return s.retrySourceSearch(ctx, searchSourceOpenAlex, func(attempt int) ([]SearchPaper, error) {
		variant := searchQueryVariant(candidates, attempt)
		pageSize := minInt(100, limit)
		params := url.Values{
			"search":   {variant},
			"per-page": {strconv.Itoa(pageSize)},
			"select": {
				"id,display_name,publication_year,cited_by_count,doi,ids,authorships," +
					"primary_location,best_oa_location,abstract_inverted_index,topics,type",
			},
		}
		if s.openAlexAPIKey != "" {
			params.Set("api_key", s.openAlexAPIKey)
		}

		var payload struct {
			Results []struct {
				ID                    string            `json:"id"`
				DisplayName           string            `json:"display_name"`
				PublicationYear       int               `json:"publication_year"`
				CitedByCount          int               `json:"cited_by_count"`
				DOI                   string            `json:"doi"`
				IDs                   map[string]string `json:"ids"`
				AbstractInvertedIndex map[string][]int  `json:"abstract_inverted_index"`
				Type                  string            `json:"type"`
				Authorships           []struct {
					Author struct {
						DisplayName string `json:"display_name"`
					} `json:"author"`
					Institutions []struct {
						DisplayName string `json:"display_name"`
					} `json:"institutions"`
				} `json:"authorships"`
				PrimaryLocation *struct {
					LandingPageURL string `json:"landing_page_url"`
					PDFURL         string `json:"pdf_url"`
					Source         *struct {
						DisplayName string `json:"display_name"`
					} `json:"source"`
				} `json:"primary_location"`
				BestOALocation *struct {
					LandingPageURL string `json:"landing_page_url"`
					PDFURL         string `json:"pdf_url"`
				} `json:"best_oa_location"`
				Topics []struct {
					DisplayName string `json:"display_name"`
				} `json:"topics"`
			} `json:"results"`
		}
		if err := s.getSearchJSON(ctx, "https://api.openalex.org/works?"+params.Encode(), &payload); err != nil {
			return nil, err
		}

		papers := make([]SearchPaper, 0, len(payload.Results))
		for _, item := range payload.Results {
			title := cleanSearchText(item.DisplayName)
			if title == "" {
				continue
			}
			authors := make([]string, 0, len(item.Authorships))
			institutions := make([]string, 0)
			for _, authorship := range item.Authorships {
				authors = appendNonEmptyUnique(authors, authorship.Author.DisplayName)
				for _, institution := range authorship.Institutions {
					institutions = appendNonEmptyUnique(institutions, institution.DisplayName)
				}
			}
			externalIDs := openAlexExternalIDs(item.IDs, item.ID, item.DOI)
			landingURL := ""
			oaPDFURL := ""
			venue := ""
			if item.PrimaryLocation != nil {
				landingURL = strings.TrimSpace(item.PrimaryLocation.LandingPageURL)
				oaPDFURL = strings.TrimSpace(item.PrimaryLocation.PDFURL)
				if item.PrimaryLocation.Source != nil {
					venue = strings.TrimSpace(item.PrimaryLocation.Source.DisplayName)
				}
			}
			if item.BestOALocation != nil {
				if landingURL == "" {
					landingURL = strings.TrimSpace(item.BestOALocation.LandingPageURL)
				}
				if strings.TrimSpace(item.BestOALocation.PDFURL) != "" {
					oaPDFURL = strings.TrimSpace(item.BestOALocation.PDFURL)
				}
			}
			if landingURL == "" {
				landingURL = strings.TrimSpace(item.ID)
			}
			if arxivID := extractArxivID(landingURL); arxivID != "" {
				externalIDs["ArXiv"] = arxivID
			}
			keywords := make([]string, 0, minInt(6, len(item.Topics)))
			for _, topic := range item.Topics {
				keywords = appendNonEmptyUnique(keywords, topic.DisplayName)
				if len(keywords) >= 6 {
					break
				}
			}
			papers = append(papers, SearchPaper{
				ID:               strings.TrimPrefix(strings.TrimSpace(item.ID), "https://openalex.org/"),
				Title:            title,
				Authors:          strings.Join(authors, ", "),
				Abstract:         reconstructOpenAlexAbstract(item.AbstractInvertedIndex),
				Year:             item.PublicationYear,
				Journal:          venue,
				PublicationVenue: venue,
				PublicationYear:  item.PublicationYear,
				CitationCount:    item.CitedByCount,
				URL:              landingURL,
				Category:         strings.TrimSpace(item.Type),
				Tags:             keywords,
				Source:           "openalex",
				Sources:          []string{"openalex"},
				ExternalIDs:      externalIDs,
				PDFCandidates:    buildInitialPDFCandidates(oaPDFURL, externalIDs),
				Institutions:     institutions,
				Keywords:         keywords,
				SourceLabel:      searchSourceOpenAlex,
			})
		}
		if len(papers) == 0 {
			return nil, &emptyResultsVariantError{variant: variant, candidateCount: len(candidates)}
		}
		return papers, nil
	}, onAttempt)
}

func (s *SearchClient) searchOpenReview(
	ctx context.Context,
	query string,
	limit int,
	onAttempt func(string, int, bool, bool, int, error),
) ([]SearchPaper, error) {
	candidates := buildSearchQueryCandidates(query)
	return s.retrySourceSearch(ctx, searchSourceReview, func(attempt int) ([]SearchPaper, error) {
		variant := searchQueryVariant(candidates, attempt)
		params := url.Values{
			"term":    {variant},
			"type":    {"terms"},
			"content": {"all"},
			"source":  {"forum"},
			"limit":   {strconv.Itoa(minInt(limit, 100))},
		}
		var payload struct {
			Notes []struct {
				ID      string                     `json:"id"`
				Forum   string                     `json:"forum"`
				Content map[string]json.RawMessage `json:"content"`
			} `json:"notes"`
		}
		if err := s.getSearchJSON(ctx, "https://api2.openreview.net/notes/search?"+params.Encode(), &payload); err != nil {
			return nil, err
		}

		papers := make([]SearchPaper, 0, len(payload.Notes))
		for _, note := range payload.Notes {
			title := cleanSearchText(openReviewString(note.Content["title"]))
			if title == "" {
				continue
			}
			forumID := strings.TrimSpace(note.Forum)
			if forumID == "" {
				forumID = strings.TrimSpace(note.ID)
			}
			authors := openReviewStrings(note.Content["authors"])
			keywords := openReviewStrings(note.Content["keywords"])
			venue := cleanSearchText(openReviewString(note.Content["venue"]))
			if venue == "" {
				venue = cleanSearchText(openReviewString(note.Content["venueid"]))
			}
			year := extractPublicationYear(venue)
			externalIDs := map[string]string{"OpenReview": forumID}
			if doi := normalizeDOI(openReviewString(note.Content["doi"])); doi != "" {
				externalIDs["DOI"] = doi
			}
			if arxivID := extractArxivID(openReviewString(note.Content["arxiv"])); arxivID != "" {
				externalIDs["ArXiv"] = arxivID
			}
			papers = append(papers, SearchPaper{
				ID:               forumID,
				Title:            title,
				Authors:          strings.Join(authors, ", "),
				Abstract:         cleanSearchText(openReviewString(note.Content["abstract"])),
				Year:             year,
				Journal:          venue,
				PublicationVenue: venue,
				PublicationYear:  year,
				URL:              "https://openreview.net/forum?id=" + url.QueryEscape(forumID),
				Category:         venue,
				Tags:             appendNonEmptyUnique(keywords, venue),
				Source:           "openreview",
				Sources:          []string{"openreview"},
				ExternalIDs:      externalIDs,
				PDFCandidates:    []string{"https://openreview.net/pdf?id=" + url.QueryEscape(forumID)},
				Keywords:         keywords,
				SourceLabel:      searchSourceReview,
			})
		}
		if len(papers) == 0 {
			return nil, &emptyResultsVariantError{variant: variant, candidateCount: len(candidates)}
		}
		return papers, nil
	}, onAttempt)
}

func (s *SearchClient) searchDBLP(
	ctx context.Context,
	query string,
	limit int,
	onAttempt func(string, int, bool, bool, int, error),
) ([]SearchPaper, error) {
	candidates := buildSearchQueryCandidates(query)
	return s.retrySourceSearch(ctx, searchSourceDBLP, func(attempt int) ([]SearchPaper, error) {
		variant := searchQueryVariant(candidates, attempt)
		params := url.Values{
			"q":      {variant},
			"h":      {strconv.Itoa(minInt(limit, 100))},
			"format": {"json"},
		}
		var payload struct {
			Result struct {
				Hits struct {
					Hit []struct {
						Info struct {
							Authors json.RawMessage `json:"authors"`
							Title   string          `json:"title"`
							Venue   string          `json:"venue"`
							Year    any             `json:"year"`
							DOI     string          `json:"doi"`
							URL     string          `json:"url"`
							EE      json.RawMessage `json:"ee"`
							Type    string          `json:"type"`
						} `json:"info"`
					} `json:"hit"`
				} `json:"hits"`
			} `json:"result"`
		}
		if err := s.getSearchJSON(ctx, "https://dblp.org/search/publ/api?"+params.Encode(), &payload); err != nil {
			return nil, err
		}

		papers := make([]SearchPaper, 0, len(payload.Result.Hits.Hit))
		for _, hit := range payload.Result.Hits.Hit {
			title := cleanSearchText(hit.Info.Title)
			if title == "" {
				continue
			}
			doi := normalizeDOI(hit.Info.DOI)
			externalIDs := map[string]string{}
			if doi != "" {
				externalIDs["DOI"] = doi
			}
			landingURL := strings.TrimSpace(hit.Info.URL)
			ee := firstJSONString(hit.Info.EE)
			if landingURL == "" {
				landingURL = ee
			}
			papers = append(papers, SearchPaper{
				ID:               strings.TrimSpace(hit.Info.URL),
				Title:            title,
				Authors:          strings.Join(dblpAuthors(hit.Info.Authors), ", "),
				Year:             anyPublicationYear(hit.Info.Year),
				Journal:          cleanSearchText(hit.Info.Venue),
				PublicationVenue: cleanSearchText(hit.Info.Venue),
				PublicationYear:  anyPublicationYear(hit.Info.Year),
				URL:              landingURL,
				Category:         cleanSearchText(hit.Info.Type),
				Source:           "dblp",
				Sources:          []string{"dblp"},
				ExternalIDs:      externalIDs,
				PDFCandidates:    buildInitialPDFCandidates(ee, externalIDs),
				SourceLabel:      searchSourceDBLP,
			})
		}
		if len(papers) == 0 {
			return nil, &emptyResultsVariantError{variant: variant, candidateCount: len(candidates)}
		}
		return papers, nil
	}, onAttempt)
}

func (s *SearchClient) getSearchJSON(ctx context.Context, requestURL string, target any) error {
	reqCtx, cancel := context.WithTimeout(ctx, s.attemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DiveEnd/0.1 academic-search")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	body, readErr := readExternalHTTPBody(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode >= 400 {
		return &searchProviderHTTPError{
			statusCode: resp.StatusCode,
			detail:     redactSensitiveText(string(body)),
			retryAfter: searchRetryAfter(resp.Header),
		}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode search response: %w", err)
	}
	return nil
}

func searchRetryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		if wait := time.Until(retryAt); wait > 0 {
			return wait
		}
	}
	return 0
}

func reconstructOpenAlexAbstract(index map[string][]int) string {
	maxPosition := -1
	for _, positions := range index {
		for _, position := range positions {
			if position > maxPosition {
				maxPosition = position
			}
		}
	}
	if maxPosition < 0 || maxPosition > 10000 {
		return ""
	}
	words := make([]string, maxPosition+1)
	for word, positions := range index {
		for _, position := range positions {
			if position >= 0 && position < len(words) {
				words[position] = word
			}
		}
	}
	return cleanSearchText(strings.Join(words, " "))
}

func openAlexExternalIDs(ids map[string]string, openAlexID, doi string) map[string]string {
	result := map[string]string{}
	for key, value := range ids {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "doi":
			if normalized := normalizeDOI(value); normalized != "" {
				result["DOI"] = normalized
			}
		case "openalex":
			result["OpenAlex"] = strings.TrimPrefix(strings.TrimSpace(value), "https://openalex.org/")
		case "pmid":
			result["PubMed"] = strings.TrimPrefix(strings.TrimSpace(value), "https://pubmed.ncbi.nlm.nih.gov/")
		case "pmcid":
			result["PubMedCentral"] = strings.TrimPrefix(strings.TrimSpace(value), "https://www.ncbi.nlm.nih.gov/pmc/articles/")
		}
	}
	if result["OpenAlex"] == "" {
		result["OpenAlex"] = strings.TrimPrefix(strings.TrimSpace(openAlexID), "https://openalex.org/")
	}
	if result["DOI"] == "" {
		result["DOI"] = normalizeDOI(doi)
	}
	for key, value := range result {
		if strings.TrimSpace(value) == "" {
			delete(result, key)
		}
	}
	return result
}

func openReviewString(raw json.RawMessage) string {
	var direct string
	if json.Unmarshal(raw, &direct) == nil {
		return strings.TrimSpace(direct)
	}
	var wrapped struct {
		Value any `json:"value"`
	}
	if json.Unmarshal(raw, &wrapped) != nil {
		return ""
	}
	switch value := wrapped.Value.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

func openReviewStrings(raw json.RawMessage) []string {
	var direct []string
	if json.Unmarshal(raw, &direct) == nil {
		return compactStrings(direct, 0)
	}
	var wrapped struct {
		Value json.RawMessage `json:"value"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && len(wrapped.Value) > 0 {
		if json.Unmarshal(wrapped.Value, &direct) == nil {
			return compactStrings(direct, 0)
		}
		var single string
		if json.Unmarshal(wrapped.Value, &single) == nil {
			return compactStrings(strings.Split(single, ","), 0)
		}
	}
	return nil
}

func dblpAuthors(raw json.RawMessage) []string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	result := make([]string, 0)
	var walk func(any)
	walk = func(item any) {
		switch typed := item.(type) {
		case map[string]any:
			if text, ok := typed["text"].(string); ok {
				result = appendNonEmptyUnique(result, cleanSearchText(text))
				return
			}
			for _, nested := range typed {
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		case string:
			result = appendNonEmptyUnique(result, cleanSearchText(typed))
		}
	}
	walk(value)
	return result
}

func firstJSONString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value)
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil && len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

func anyPublicationYear(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		return extractPublicationYear(typed)
	default:
		return 0
	}
}

func extractPublicationYear(value string) int {
	for _, field := range strings.Fields(value) {
		field = strings.Trim(field, "(),.[]")
		if len(field) != 4 {
			continue
		}
		year, err := strconv.Atoi(field)
		if err == nil && year >= 1000 && year <= 3000 {
			return year
		}
	}
	return 0
}

func cleanSearchText(value string) string {
	value = html.UnescapeString(value)
	value = htmlTagPattern.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}

func appendNonEmptyUnique(values []string, candidates ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(candidates))
	for _, value := range values {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, candidate)
	}
	return values
}

func normalizeDOI(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "https://doi.org/")
	value = strings.TrimPrefix(value, "http://doi.org/")
	value = strings.TrimPrefix(value, "doi:")
	return strings.TrimSpace(value)
}

func normalizeArxivID(value string) string {
	return arxivVersionSuffixPattern.ReplaceAllString(extractArxivID(value), "")
}

func textMatchesSearchTerm(text, term string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	term = strings.ToLower(strings.TrimSpace(term))
	if text == "" || term == "" {
		return false
	}
	asciiOnly := true
	for _, r := range term {
		if r > 127 {
			asciiOnly = false
			break
		}
	}
	if !asciiOnly {
		return strings.Contains(text, term)
	}
	for _, token := range strings.Fields(normalizeSearchDelimiters(text)) {
		if token == term {
			return true
		}
	}
	return false
}

func filterSearchPapersByQueries(papers []SearchPaper, queries []string) []SearchPaper {
	queryTokens := make([]string, 0, 16)
	phrases := make([]string, 0, len(queries))
	for _, query := range queries {
		normalized := strings.ToLower(strings.TrimSpace(query))
		if normalized == "" {
			continue
		}
		phrases = append(phrases, normalized)
		queryTokens = append(queryTokens, extractDeepStartMessageTokens(normalized)...)
	}
	queryTokens = uniqueStrings(queryTokens)
	if len(queryTokens) == 0 {
		return papers
	}
	minimumMatches := 1
	if len(queryTokens) >= 3 {
		minimumMatches = 2
	}

	filtered := make([]SearchPaper, 0, len(papers))
	for _, paper := range papers {
		title := strings.ToLower(strings.TrimSpace(paper.Title))
		abstract := strings.ToLower(strings.TrimSpace(paper.Abstract))
		directPhrase := false
		for _, phrase := range phrases {
			if strings.Contains(title, phrase) || strings.Contains(abstract, phrase) {
				directPhrase = true
				break
			}
		}
		matches := 0
		for _, token := range queryTokens {
			if textMatchesSearchTerm(title, token) || textMatchesSearchTerm(abstract, token) {
				matches++
			}
		}
		if directPhrase || matches >= minimumMatches {
			filtered = append(filtered, paper)
		}
	}

	// A provider may match a synonym that is absent from sparse metadata.
	// Preserve its ranked response if local filtering would erase everything.
	if len(filtered) == 0 {
		return papers
	}
	return filtered
}

func dedupeSearchPaperKeys(paper SearchPaper) []string {
	keys := make([]string, 0, 6)
	if doi := normalizeDOI(externalIDValue(paper.ExternalIDs, "DOI")); doi != "" {
		keys = append(keys, "doi|"+doi)
	}
	if arxivID := normalizeArxivID(externalIDValue(paper.ExternalIDs, "ArXiv")); arxivID != "" {
		keys = append(keys, "arxiv|"+strings.ToLower(arxivID))
	} else if paper.Source == "arxiv" {
		if arxivID := normalizeArxivID(paper.ID); arxivID != "" {
			keys = append(keys, "arxiv|"+strings.ToLower(arxivID))
		}
	}
	if forumID := strings.TrimSpace(externalIDValue(paper.ExternalIDs, "OpenReview")); forumID != "" {
		keys = append(keys, "openreview|"+strings.ToLower(forumID))
	}
	title := normalizedDedupeToken(paper.Title)
	if title != "" {
		year := paper.Year
		if year == 0 {
			year = paper.PublicationYear
		}
		firstAuthor := normalizedDedupeToken(strings.Split(paper.Authors, ",")[0])
		keys = append(keys, fmt.Sprintf("title_year_author|%s|%d|%s", title, year, firstAuthor))
		keys = append(keys, fmt.Sprintf("title_year|%s|%d", title, year))
		if len(strings.Fields(title)) >= 5 || len([]rune(title)) >= 40 {
			keys = append(keys, "title_exact|"+title)
		}
	}
	if len(keys) == 0 {
		if value := normalizedDedupeToken(paper.URL); value != "" {
			keys = append(keys, "url|"+value)
		} else if value := normalizedDedupeToken(paper.ID); value != "" {
			keys = append(keys, "id|"+value)
		}
	}
	return uniqueStrings(keys)
}

func mergeDuplicateSearchPapers(papers []SearchPaper) []SearchPaper {
	groups := make([]SearchPaper, 0, len(papers))
	active := make([]bool, 0, len(papers))
	aliases := make(map[string]int, len(papers)*2)
	for _, paper := range papers {
		keys := dedupeSearchPaperKeys(paper)
		matched := make(map[int]struct{})
		for _, key := range keys {
			if index, ok := aliases[key]; ok && index < len(active) && active[index] {
				matched[index] = struct{}{}
			}
		}
		if len(matched) == 0 {
			index := len(groups)
			groups = append(groups, normalizeSearchPaperProvenance(paper))
			active = append(active, true)
			for _, key := range keys {
				aliases[key] = index
			}
			continue
		}

		indices := make([]int, 0, len(matched))
		for index := range matched {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		target := indices[0]
		groups[target] = mergeSearchPaperMetadata(groups[target], paper)
		for _, index := range indices[1:] {
			groups[target] = mergeSearchPaperMetadata(groups[target], groups[index])
			active[index] = false
		}
		for _, key := range dedupeSearchPaperKeys(groups[target]) {
			aliases[key] = target
		}
	}

	merged := make([]SearchPaper, 0, len(groups))
	for index, paper := range groups {
		if active[index] {
			merged = append(merged, paper)
		}
	}
	return merged
}

func normalizeSearchPaperProvenance(paper SearchPaper) SearchPaper {
	if len(paper.Sources) == 0 && strings.TrimSpace(paper.Source) != "" {
		paper.Sources = []string{paper.Source}
	}
	paper.Sources = uniqueStrings(paper.Sources)
	if strings.TrimSpace(paper.SourceLabel) == "" {
		paper.SourceLabel = strings.Join(paper.Sources, " + ")
	}
	return paper
}

func mergeSearchPaperMetadata(existing, candidate SearchPaper) SearchPaper {
	existing = normalizeSearchPaperProvenance(existing)
	candidate = normalizeSearchPaperProvenance(candidate)
	preferred, other := existing, candidate
	if isSearchPaperPreferred(candidate, existing) {
		preferred, other = candidate, existing
	}
	if preferred.Abstract == "" {
		preferred.Abstract = other.Abstract
	}
	if preferred.Authors == "" {
		preferred.Authors = other.Authors
	}
	if preferred.Year == 0 {
		preferred.Year = other.Year
	}
	if preferred.PublicationYear == 0 {
		preferred.PublicationYear = other.PublicationYear
	}
	if preferred.Journal == "" {
		preferred.Journal = other.Journal
	}
	if preferred.PublicationVenue == "" {
		preferred.PublicationVenue = other.PublicationVenue
	}
	if preferred.URL == "" {
		preferred.URL = other.URL
	}
	if preferred.Category == "" {
		preferred.Category = other.Category
	}
	if other.CitationCount > preferred.CitationCount {
		preferred.CitationCount = other.CitationCount
	}
	if preferred.ExternalIDs == nil {
		preferred.ExternalIDs = map[string]string{}
	}
	for key, value := range other.ExternalIDs {
		if strings.TrimSpace(preferred.ExternalIDs[key]) == "" {
			preferred.ExternalIDs[key] = value
		}
	}
	preferred.Sources = uniqueStrings(append(preferred.Sources, other.Sources...))
	preferred.PDFCandidates = uniqueStrings(append(preferred.PDFCandidates, other.PDFCandidates...))
	preferred.Institutions = uniqueStrings(append(preferred.Institutions, other.Institutions...))
	preferred.Keywords = uniqueStrings(append(preferred.Keywords, other.Keywords...))
	preferred.Tags = uniqueStrings(append(preferred.Tags, other.Tags...))
	labels := make([]string, 0, len(preferred.Sources))
	for _, source := range searchSourceOrder {
		for _, sourceID := range preferred.Sources {
			if strings.EqualFold(strings.ReplaceAll(source, " ", "_"), sourceID) ||
				(source == searchSourceReview && sourceID == "openreview") ||
				(source == searchSourceOpenAlex && sourceID == "openalex") ||
				(source == searchSourceArxiv && sourceID == "arxiv") ||
				(source == searchSourceDBLP && sourceID == "dblp") ||
				(source == searchSourceSemantic && sourceID == "semantic_scholar") {
				labels = appendNonEmptyUnique(labels, source)
			}
		}
	}
	preferred.SourceLabel = strings.Join(labels, " + ")
	return preferred
}
