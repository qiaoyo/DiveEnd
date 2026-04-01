package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type llmService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error)
}

type paperSearchService interface {
	Search(query string, limit int) ([]SearchPaper, error)
}

type LLMClient struct {
	apiKey                 string
	model                  string
	providerID             string
	providerName           string
	providerType           string
	baseURL                string
	wireAPI                string
	requiresOpenAIAuth     bool
	disableResponseStorage bool
	reasoningEffort        string
	httpClient             *http.Client
}

func NewLLMClient(config AppConfig) *LLMClient {
	llmConfig := normalizeLLMConfig(config.LLM)

	return &LLMClient{
		apiKey:                 llmConfig.APIKey,
		model:                  llmConfig.Model,
		providerID:             llmConfig.ProviderID,
		providerName:           llmConfig.ProviderName,
		providerType:           llmConfig.ProviderType,
		baseURL:                llmConfig.BaseURL,
		wireAPI:                llmConfig.WireAPI,
		requiresOpenAIAuth:     llmConfig.RequiresOpenAIAuth,
		disableResponseStorage: llmConfig.DisableResponseStorage,
		reasoningEffort:        llmConfig.ReasoningEffort,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepStartAIRequest struct {
	RootPrompt       string
	CurrentQuery     string
	TargetFolderName string
	Messages         []DeepStartMessage
	Results          []SearchPaper
}

type DeepStartAIResponse struct {
	Title    string
	Analysis DeepStartAnalysis
}

func (c *LLMClient) TranslateSection(section, originalText string) (translated string, summary string, err error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return "", "", fmt.Errorf("missing API key for %s", c.providerLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return "", "", fmt.Errorf("missing model for %s", c.providerLabel())
	}

	prompt := fmt.Sprintf(`
Please translate the following academic paper section from English to Chinese.
Then provide a concise Chinese summary with the core findings and reading cues.

Return valid JSON only:
{
  "translation": "full Chinese translation",
  "summary": "short Chinese summary"
}

Section: %s

Original text:
%s
`, section, originalText)

	response, err := c.chat([]llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return "", "", err
	}

	var parsed struct {
		Translation string `json:"translation"`
		Summary     string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return "", "", fmt.Errorf("failed to parse translation response: %w", err)
	}

	if strings.TrimSpace(parsed.Translation) == "" {
		return "", "", fmt.Errorf("empty translation result")
	}

	return parsed.Translation, parsed.Summary, nil
}

func (c *LLMClient) AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.providerLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.providerLabel())
	}

	payload := map[string]any{
		"rootPrompt":       request.RootPrompt,
		"currentQuery":     request.CurrentQuery,
		"targetFolderName": request.TargetFolderName,
		"messages":         request.Messages,
		"results":          request.Results,
	}
	contextJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`
You are helping a user explore an academic field and decide which papers to import into a reading library.

You will receive:
- the user's research goal
- the current retrieval query
- the target folder name
- the recent conversation history
- a list of retrieved papers

Rules:
- Return valid JSON only.
- Do not invent paper IDs. Only use IDs that exist in the provided results.
- Group the papers into meaningful directions or sub-topics.
- Mark recommended papers with tier "core", "important", or "optional".
- If the result set is weak or empty, explain that honestly and propose better follow-up questions and search queries.
- Keep overviews and reasons concise but useful.

Return this exact JSON shape:
{
  "title": "short session title",
  "overview": "short Chinese overview",
  "directions": [
    {
      "id": "direction-id",
      "name": "direction name",
      "summary": "what this direction is about",
      "why": "why it matters",
      "paperIds": ["paper-id-1"]
    }
  ],
  "paperNotes": [
    {
      "paperId": "paper-id-1",
      "tier": "core",
      "reason": "why to read it",
      "directionIds": ["direction-id"]
    }
  ],
  "followUpQuestions": ["question 1"],
  "suggestedQueries": ["new search query"],
  "recommendedPaperIds": ["paper-id-1"]
}

Context:
%s
`, string(contextJSON))

	response, err := c.chat([]llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Title               string               `json:"title"`
		Overview            string               `json:"overview"`
		Directions          []DeepStartDirection `json:"directions"`
		PaperNotes          []DeepStartPaperNote `json:"paperNotes"`
		FollowUpQuestions   []string             `json:"followUpQuestions"`
		SuggestedQueries    []string             `json:"suggestedQueries"`
		RecommendedPaperIDs []string             `json:"recommendedPaperIds"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse DeepStart analysis response: %w", err)
	}

	analysis := normalizeDeepStartAnalysis(DeepStartAnalysis{
		Overview:            parsed.Overview,
		Directions:          parsed.Directions,
		PaperNotes:          parsed.PaperNotes,
		FollowUpQuestions:   parsed.FollowUpQuestions,
		SuggestedQueries:    parsed.SuggestedQueries,
		RecommendedPaperIDs: parsed.RecommendedPaperIDs,
	}, request)

	return &DeepStartAIResponse{
		Title:    normalizeDeepStartTitle(parsed.Title, request.RootPrompt, request.CurrentQuery),
		Analysis: analysis,
	}, nil
}

func (c *LLMClient) requiresAPIKey() bool {
	return c.wireAPI == "anthropic_messages" || c.requiresOpenAIAuth
}

func (c *LLMClient) chat(messages []llmMessage) (string, error) {
	switch c.wireAPI {
	case "responses":
		return c.chatResponses(messages)
	case "chat_completions":
		return c.chatOpenAI(messages)
	case "anthropic_messages":
		return c.chatAnthropic(messages)
	default:
		return "", fmt.Errorf("unknown wire API for %s: %s", c.providerLabel(), c.wireAPI)
	}
}

func (c *LLMClient) providerLabel() string {
	if strings.TrimSpace(c.providerName) != "" {
		return c.providerName
	}
	if strings.TrimSpace(c.providerID) != "" {
		return c.providerID
	}
	return c.providerType
}

func (c *LLMClient) endpointURL(path string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(defaultAnthropicLLMConfig().BaseURL, "/")
	}
	if strings.HasSuffix(baseURL, path) {
		return baseURL
	}
	return baseURL + path
}

func (c *LLMClient) chatResponses(messages []llmMessage) (string, error) {
	reqBody := map[string]any{
		"model": c.model,
		"input": messagesToPrompt(messages),
		"store": !c.disableResponseStorage,
	}
	if effort := strings.TrimSpace(c.reasoningEffort); effort != "" {
		reqBody["reasoning"] = map[string]any{"effort": effort}
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.endpointURL("/responses"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.requiresOpenAIAuth {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s responses request failed: %s", c.providerLabel(), strings.TrimSpace(string(body)))
	}

	return parseResponsesText(body)
}

func (c *LLMClient) chatOpenAI(messages []llmMessage) (string, error) {
	reqBody := struct {
		Model    string       `json:"model"`
		Messages []llmMessage `json:"messages"`
	}{
		Model:    c.model,
		Messages: messages,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.endpointURL("/chat/completions"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.requiresOpenAIAuth {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s chat completions request failed: %s", c.providerLabel(), strings.TrimSpace(string(body)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}

	return result.Choices[0].Message.Content, nil
}

func (c *LLMClient) chatAnthropic(messages []llmMessage) (string, error) {
	reqBody := struct {
		Model     string       `json:"model"`
		Messages  []llmMessage `json:"messages"`
		MaxTokens int          `json:"max_tokens"`
	}{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.endpointURL("/messages"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s messages request failed: %s", c.providerLabel(), strings.TrimSpace(string(body)))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from anthropic")
	}

	var text strings.Builder
	for _, content := range result.Content {
		if content.Type == "text" {
			text.WriteString(content.Text)
		}
	}

	return text.String(), nil
}

func messagesToPrompt(messages []llmMessage) string {
	if len(messages) == 1 {
		return messages[0].Content
	}

	var prompt strings.Builder
	for index, message := range messages {
		if index > 0 {
			prompt.WriteString("\n\n")
		}
		prompt.WriteString(strings.ToUpper(message.Role))
		prompt.WriteString(":\n")
		prompt.WriteString(message.Content)
	}

	return prompt.String()
}

func parseResponsesText(body []byte) (string, error) {
	var result struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if strings.TrimSpace(result.OutputText) != "" {
		return result.OutputText, nil
	}

	var text strings.Builder
	for _, item := range result.Output {
		for _, content := range item.Content {
			if content.Text != "" {
				text.WriteString(content.Text)
			}
		}
	}
	if text.Len() > 0 {
		return text.String(), nil
	}

	var fallback struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &fallback); err == nil && len(fallback.Choices) > 0 {
		return fallback.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no textual response returned")
}

func normalizeDeepStartAnalysis(analysis DeepStartAnalysis, request DeepStartAIRequest) DeepStartAnalysis {
	validPaperIDs := make(map[string]struct{}, len(request.Results))
	for _, paper := range request.Results {
		id := strings.TrimSpace(paper.ID)
		if id != "" {
			validPaperIDs[id] = struct{}{}
		}
	}

	cleanDirections := make([]DeepStartDirection, 0, len(analysis.Directions))
	seenDirectionIDs := make(map[string]struct{})
	for index, direction := range analysis.Directions {
		directionID := strings.TrimSpace(direction.ID)
		if directionID == "" {
			directionID = fmt.Sprintf("direction-%d", index+1)
		}
		if _, exists := seenDirectionIDs[directionID]; exists {
			continue
		}
		seenDirectionIDs[directionID] = struct{}{}

		paperIDs := filterExistingPaperIDs(direction.PaperIDs, validPaperIDs)
		cleanDirections = append(cleanDirections, DeepStartDirection{
			ID:       directionID,
			Name:     strings.TrimSpace(direction.Name),
			Summary:  strings.TrimSpace(direction.Summary),
			Why:      strings.TrimSpace(direction.Why),
			PaperIDs: paperIDs,
		})
	}

	cleanNotes := make([]DeepStartPaperNote, 0, len(analysis.PaperNotes))
	for _, note := range analysis.PaperNotes {
		paperID := strings.TrimSpace(note.PaperID)
		if _, ok := validPaperIDs[paperID]; !ok {
			continue
		}
		tier := strings.ToLower(strings.TrimSpace(note.Tier))
		if tier != "core" && tier != "important" && tier != "optional" {
			tier = "important"
		}

		directionIDs := make([]string, 0, len(note.DirectionIDs))
		for _, directionID := range note.DirectionIDs {
			directionID = strings.TrimSpace(directionID)
			if directionID == "" {
				continue
			}
			if _, exists := seenDirectionIDs[directionID]; exists {
				directionIDs = append(directionIDs, directionID)
			}
		}

		cleanNotes = append(cleanNotes, DeepStartPaperNote{
			PaperID:      paperID,
			Tier:         tier,
			Reason:       strings.TrimSpace(note.Reason),
			DirectionIDs: uniqueStrings(directionIDs),
		})
	}

	return DeepStartAnalysis{
		Overview:            strings.TrimSpace(analysis.Overview),
		Directions:          cleanDirections,
		PaperNotes:          cleanNotes,
		FollowUpQuestions:   compactStrings(analysis.FollowUpQuestions, 4),
		SuggestedQueries:    compactStrings(analysis.SuggestedQueries, 4),
		RecommendedPaperIDs: filterExistingPaperIDs(analysis.RecommendedPaperIDs, validPaperIDs),
	}
}

func normalizeDeepStartTitle(title, rootPrompt, currentQuery string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}

	fallback := strings.TrimSpace(currentQuery)
	if fallback == "" {
		fallback = strings.TrimSpace(rootPrompt)
	}
	fallback = strings.Join(strings.Fields(fallback), " ")
	if len([]rune(fallback)) > 48 {
		return string([]rune(fallback)[:48])
	}
	if fallback == "" {
		return "未命名探索"
	}
	return fallback
}

func filterExistingPaperIDs(ids []string, validPaperIDs map[string]struct{}) []string {
	clean := make([]string, 0, len(ids))
	seen := make(map[string]struct{})
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := validPaperIDs[id]; !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	return clean
}

func compactStrings(values []string, limit int) []string {
	clean := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
		if limit > 0 && len(clean) >= limit {
			break
		}
	}
	return clean
}

func uniqueStrings(values []string) []string {
	return compactStrings(values, 0)
}

type SearchClient struct {
	semanticScholarAPIKey string
	httpClient            *http.Client
}

func NewSearchClient(config AppConfig) *SearchClient {
	searchConfig := normalizeSearchAPIConfig(config.Search)
	return &SearchClient{
		semanticScholarAPIKey: searchConfig.SemanticScholarAPIKey,
		httpClient:            &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *SearchClient) Search(query string, limit int) ([]SearchPaper, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 20
	}

	var combined []SearchPaper
	var errs []string

	if papers, err := s.searchSemanticScholar(query, limit); err == nil {
		combined = append(combined, papers...)
	} else {
		errs = append(errs, "semantic scholar: "+err.Error())
	}

	if papers, err := s.searchArXiv(query, limit); err == nil {
		combined = append(combined, papers...)
	} else {
		errs = append(errs, "arXiv: "+err.Error())
	}

	paperMap := make(map[string]SearchPaper)
	for _, paper := range combined {
		key := strings.ToLower(strings.TrimSpace(paper.Title))
		if key == "" {
			continue
		}
		if existing, ok := paperMap[key]; ok && strings.TrimSpace(existing.URL) != "" {
			continue
		}
		paperMap[key] = paper
	}

	papers := make([]SearchPaper, 0, len(paperMap))
	for _, paper := range paperMap {
		papers = append(papers, paper)
	}

	if len(papers) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf(strings.Join(errs, "; "))
	}
	if len(papers) > limit {
		papers = papers[:limit]
	}

	return papers, nil
}

func (s *SearchClient) searchSemanticScholar(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf(
		"https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&fields=title,authors,abstract,year,venue,journal,openAccessPdf",
		url.QueryEscape(query),
		limit,
	)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	if s.semanticScholarAPIKey != "" {
		req.Header.Set("x-api-key", s.semanticScholarAPIKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed: %s", strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []struct {
			PaperID string `json:"paperId"`
			Title   string `json:"title"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			Abstract string `json:"abstract"`
			Year     int    `json:"year"`
			Venue    string `json:"venue"`
			Journal  *struct {
				Name string `json:"name"`
			} `json:"journal"`
			OpenAccessPDF *struct {
				URL string `json:"url"`
			} `json:"openAccessPdf"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	papers := make([]SearchPaper, 0, len(result.Data))
	for _, item := range result.Data {
		authors := make([]string, 0, len(item.Authors))
		for _, author := range item.Authors {
			if strings.TrimSpace(author.Name) != "" {
				authors = append(authors, author.Name)
			}
		}

		journal := strings.TrimSpace(item.Venue)
		if journal == "" && item.Journal != nil {
			journal = strings.TrimSpace(item.Journal.Name)
		}

		urlValue := ""
		if item.OpenAccessPDF != nil {
			urlValue = strings.TrimSpace(item.OpenAccessPDF.URL)
		}

		papers = append(papers, SearchPaper{
			ID:       item.PaperID,
			Title:    item.Title,
			Authors:  strings.Join(authors, ", "),
			Abstract: item.Abstract,
			Year:     item.Year,
			Journal:  journal,
			URL:      urlValue,
			Tags:     []string{},
		})
	}

	return papers, nil
}

func (s *SearchClient) searchArXiv(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf(
		"https://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d",
		url.QueryEscape(query),
		limit,
	)

	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed: %s", strings.TrimSpace(string(body)))
	}

	return parseArXivXML(body)
}

func parseArXivXML(data []byte) ([]SearchPaper, error) {
	var feed struct {
		Entries []struct {
			ID        string   `xml:"id"`
			Title     string   `xml:"title"`
			Summary   string   `xml:"summary"`
			Published string   `xml:"published"`
			Authors   []string `xml:"author>name"`
		} `xml:"entry"`
	}

	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	papers := make([]SearchPaper, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		title := strings.Join(strings.Fields(entry.Title), " ")
		if strings.TrimSpace(title) == "" {
			continue
		}

		year := 0
		if len(entry.Published) >= 4 {
			fmt.Sscanf(entry.Published[:4], "%d", &year)
		}

		id := strings.TrimSpace(entry.ID)
		shortID := id
		if idx := strings.LastIndex(shortID, "/"); idx >= 0 {
			shortID = shortID[idx+1:]
		}

		papers = append(papers, SearchPaper{
			ID:       shortID,
			Title:    title,
			Authors:  strings.Join(entry.Authors, ", "),
			Abstract: strings.Join(strings.Fields(entry.Summary), " "),
			Year:     year,
			Journal:  "arXiv",
			URL:      id,
			Tags:     []string{},
		})
	}

	return papers, nil
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return raw
	}

	return raw[start : end+1]
}
