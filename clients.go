package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type llmService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error)
	AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error)
}

type paperSearchService interface {
	Search(query string, limit int) ([]SearchPaper, error)
	EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error)
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

type ScreeningAIRequest struct {
	SessionTitle string
	Papers       []ScreeningPaper
	PathHistory  []PathHistoryItem
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

func (c *LLMClient) AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.providerLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.providerLabel())
	}

	payload := map[string]any{
		"sessionTitle": request.SessionTitle,
		"pathHistory":  request.PathHistory,
		"papers":       request.Papers,
	}
	contextJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`
You are helping with a paper screening workflow for an academic reading tool.

You will receive:
- the session title
- the previous screening choices
- a candidate paper set with IDs, titles, abstracts, authors and extracted structure

Your task:
- Propose the next screening branch that best narrows the paper set.
- Prefer meaningful academic dimensions such as topic, method, task, benchmark, data domain, or paper type.
- Return 2-6 options.
- Every option must reference only paper IDs from the provided papers.
- Options should partition the current paper set as cleanly as possible.
- Use concise Chinese copy for the message and option labels.

Return valid JSON only with this exact shape:
{
  "message": "下一轮给用户看的问题",
  "dimension": "筛选维度名称",
  "allowMultiSelect": true,
  "allowSkip": false,
  "options": [
    {
      "key": "short-key",
      "label": "选项名称",
      "paperIds": ["paper-id-1"],
      "count": 1
    }
  ]
}

Context:
%s
`, string(contextJSON))

	response, err := c.chat([]llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Message          string                    `json:"message"`
		Dimension        string                    `json:"dimension"`
		AllowMultiSelect bool                      `json:"allowMultiSelect"`
		AllowSkip        bool                      `json:"allowSkip"`
		Options          []ScreeningDecisionOption `json:"options"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse screening analysis response: %w", err)
	}

	node := &ScreeningDecisionNode{
		ID:               fmt.Sprintf("screen-node-%d", time.Now().UnixNano()),
		NodeType:         "branch",
		Message:          strings.TrimSpace(parsed.Message),
		Dimension:        strings.TrimSpace(parsed.Dimension),
		Options:          parsed.Options,
		AllowMultiSelect: parsed.AllowMultiSelect,
		AllowSkip:        parsed.AllowSkip,
	}

	if node.Message == "" {
		node.Message = "请继续缩小筛选范围。"
	}
	if node.Dimension == "" {
		node.Dimension = "研究维度"
	}

	return node, nil
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

const (
	searchHTTPTimeout    = 12 * time.Second
	searchOverallTimeout = 70 * time.Second
	searchRetryDuration  = 1 * time.Minute
	searchRetryInterval  = 1 * time.Second
	searchAttemptTimeout = 1 * time.Second
	searchRetryMax       = int(searchRetryDuration / searchRetryInterval)
)

func NewSearchClient(config AppConfig) *SearchClient {
	searchConfig := normalizeSearchAPIConfig(config.Search)
	return &SearchClient{
		semanticScholarAPIKey: searchConfig.SemanticScholarAPIKey,
		httpClient:            &http.Client{Timeout: searchHTTPTimeout},
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
	if limit > 200 {
		limit = 200 // 最大限制200条
	}

	var combined []SearchPaper
	var errs []string

	// 轻微超采样，避免单源返回偏少导致结果过窄。
	searchLimit := limit
	if limit < 40 {
		searchLimit = 40
	}

	// 并行调用多个搜索源
	type sourceResult struct {
		papers []SearchPaper
		err    error
		name   string
	}

	results := make(chan sourceResult, 3)

	// Semantic Scholar (免费版,无需API key)
	go func() {
		papers, err := s.searchSemanticScholar(query, searchLimit)
		results <- sourceResult{papers: papers, err: err, name: "Semantic Scholar"}
	}()

	// arXiv
	go func() {
		papers, err := s.searchArXiv(query, searchLimit)
		results <- sourceResult{papers: papers, err: err, name: "arXiv"}
	}()

	// arxiv-sanity-lite
	go func() {
		papers, err := s.searchArxivSanityLite(query, searchLimit)
		results <- sourceResult{papers: papers, err: err, name: "arxiv-sanity-lite"}
	}()

	// 收集结果并设置总时限，避免某个源长时间阻塞 DeepStart 创建流程。
	deadline := time.NewTimer(searchOverallTimeout)
	defer deadline.Stop()

	sourceStatus := map[string]string{
		"Semantic Scholar":  "PENDING",
		"arXiv":             "PENDING",
		"arxiv-sanity-lite": "PENDING",
	}

	received := 0
	for received < 3 {
		select {
		case result := <-results:
			received++
			if result.err == nil && len(result.papers) > 0 {
				combined = append(combined, result.papers...)
				sourceStatus[result.name] = fmt.Sprintf("SUCCESS(count=%d)", len(result.papers))
				log.Printf("[Search] source=%s status=%s", result.name, sourceStatus[result.name])
				continue
			}
			if result.err != nil {
				sourceStatus[result.name] = fmt.Sprintf("FAILED(error=%v)", result.err)
				log.Printf("[Search] source=%s status=%s", result.name, sourceStatus[result.name])
				errs = append(errs, fmt.Sprintf("%s: %v", result.name, result.err))
				continue
			}
			sourceStatus[result.name] = "SUCCESS(count=0)"
			log.Printf("[Search] source=%s status=%s", result.name, sourceStatus[result.name])
		case <-deadline.C:
			errs = append(errs, fmt.Sprintf("search timed out after %s", searchOverallTimeout))
			for name, status := range sourceStatus {
				if status == "PENDING" {
					sourceStatus[name] = "FAILED(error=overall timeout)"
					log.Printf("[Search] source=%s status=%s", name, sourceStatus[name])
				}
			}
			received = 3
		}
	}

	log.Printf(
		"[Search] summary: Semantic Scholar=%s | arXiv=%s | arxiv-sanity-lite=%s",
		sourceStatus["Semantic Scholar"],
		sourceStatus["arXiv"],
		sourceStatus["arxiv-sanity-lite"],
	)

	// 去重（按标题）
	paperMap := make(map[string]SearchPaper)
	for _, paper := range combined {
		key := strings.ToLower(strings.TrimSpace(paper.Title))
		if key == "" {
			continue
		}
		// 保留有URL的版本，或者更新的版本
		if existing, ok := paperMap[key]; !ok || (strings.TrimSpace(existing.URL) == "" && strings.TrimSpace(paper.URL) != "") {
			paperMap[key] = paper
		}
	}

	papers := make([]SearchPaper, 0, len(paperMap))
	for _, paper := range paperMap {
		papers = append(papers, paper)
	}

	// 只有在完全没有结果时才返回错误
	if len(papers) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	// 如果有部分失败但至少有一个成功，在日志中记录但不返回错误
	if len(errs) > 0 && len(papers) > 0 {
		// 可以考虑在返回结果中包含警告信息
		// 但这里我们选择静默处理，让用户至少能看到部分结果
	}

	// 按年份排序（新的在前）
	sort.Slice(papers, func(i, j int) bool {
		return papers[i].Year > papers[j].Year
	})

	if len(papers) > limit {
		papers = papers[:limit]
	}

	return papers, nil
}

func (s *SearchClient) retrySourceSearch(sourceName string, fn func() ([]SearchPaper, error)) ([]SearchPaper, error) {
	startedAt := time.Now()
	var lastErr error

	for attempt := 1; attempt <= searchRetryMax; attempt++ {
		papers, err := fn()
		if err == nil {
			log.Printf("[Search][%s] attempt %d/%d succeeded with %d papers", sourceName, attempt, searchRetryMax, len(papers))
			return papers, nil
		}

		lastErr = err
		log.Printf("[Search][%s] attempt %d/%d failed: %v", sourceName, attempt, searchRetryMax, err)
		if attempt < searchRetryMax {
			time.Sleep(searchRetryInterval)
		}
	}

	return nil, fmt.Errorf("%s failed after %d retries in %v: %v", sourceName, searchRetryMax, time.Since(startedAt), lastErr)
}

// 新增：arxiv-sanity-lite 搜索
func (s *SearchClient) searchArxivSanityLite(query string, limit int) ([]SearchPaper, error) {
	// arxiv-sanity-lite API endpoint
	apiURL := fmt.Sprintf(
		"http://arxiv-sanity-lite.com/search?q=%s&size=%d",
		url.QueryEscape(query),
		limit,
	)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DiveEnd/1.0")

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
		return nil, fmt.Errorf("arxiv-sanity-lite request failed: %s", strings.TrimSpace(string(body)))
	}

	// 解析arxiv-sanity-lite响应格式
	var result struct {
		Papers []struct {
			ID       string   `json:"id"`
			Title    string   `json:"title"`
			Authors  []string `json:"authors"`
			Abstract string   `json:"abstract"`
			Year     int      `json:"year"`
			Category string   `json:"category"`
			Tags     []string `json:"tags"`
		} `json:"papers"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	papers := make([]SearchPaper, 0, len(result.Papers))
	for _, item := range result.Papers {
		authors := strings.Join(item.Authors, ", ")
		arxivURL := fmt.Sprintf("https://arxiv.org/abs/%s", strings.TrimSpace(item.ID))

		papers = append(papers, SearchPaper{
			ID:       item.ID,
			Title:    strings.TrimSpace(item.Title),
			Authors:  authors,
			Abstract: strings.TrimSpace(item.Abstract),
			Year:     item.Year,
			Journal:  "arXiv",
			URL:      arxivURL,
			Category: item.Category,
			Tags:     item.Tags,
			Source:   "arxiv_sanity",
		})
	}

	return papers, nil
}

func (s *SearchClient) searchSemanticScholar(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf(
		"https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&fields=title,authors,abstract,year,venue,journal,openAccessPdf",
		url.QueryEscape(query),
		limit,
	)

	return s.retrySourceSearch("Semantic Scholar", func() ([]SearchPaper, error) {
		ctx, cancel := context.WithTimeout(context.Background(), searchAttemptTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 400 {
			if resp.StatusCode == http.StatusTooManyRequests {
				return nil, fmt.Errorf("rate limited (429)")
			}
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
				Source:   "semantic_scholar",
			})
		}

		return papers, nil
	})
}

func (s *SearchClient) searchArXiv(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf(
		"https://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d",
		url.QueryEscape(query),
		limit,
	)

	return s.retrySourceSearch("arXiv", func() ([]SearchPaper, error) {
		ctx, cancel := context.WithTimeout(context.Background(), searchAttemptTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("request failed: %s", strings.TrimSpace(string(body)))
		}

		return parseArXivXML(body)
	})
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
			Source:   "arxiv",
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

// EnhancedSearch 实现增强搜索接口
func (s *SearchClient) EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error) {
	// 使用现有的 Search 方法获取结果
	papers, err := s.Search(query, limit+offset+50) // 多获取一些以支持分页
	if err != nil {
		return nil, err
	}

	// 年份过滤
	var filtered []SearchPaper
	if yearStart > 0 || yearEnd > 0 {
		filtered = make([]SearchPaper, 0)
		for _, paper := range papers {
			if yearStart > 0 && paper.Year < yearStart {
				continue
			}
			if yearEnd > 0 && paper.Year > yearEnd {
				continue
			}
			filtered = append(filtered, paper)
		}
	} else {
		filtered = papers
	}

	// 排序
	switch sortBy {
	case "year_desc":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Year > filtered[j].Year })
	case "year_asc":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Year < filtered[j].Year })
	default:
		// 默认按年份降序
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Year > filtered[j].Year })
	}

	// 分页处理
	total := len(filtered)
	hasMore := offset+limit < total

	start := offset
	if start >= total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	result := &EnhancedSearchResult{
		Query:     query,
		Limit:     limit,
		Offset:    offset,
		YearStart: yearStart,
		YearEnd:   yearEnd,
		SortBy:    sortBy,
		Total:     total,
		HasMore:   hasMore,
		Sources: []SearchSourceStatus{
			{Name: "Semantic Scholar", Success: true, Count: len(filtered)},
			{Name: "arXiv", Success: true, Count: len(filtered)},
			{Name: "arxiv-sanity-lite", Success: true, Count: len(filtered)},
		},
	}

	if start < end {
		result.Papers = filtered[start:end]
	} else {
		result.Papers = []SearchPaper{}
	}

	return result, nil
}
