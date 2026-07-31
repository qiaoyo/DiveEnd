package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type llmService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error)
	AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error)
}

type weakLLMService interface {
	TranslateSection(section, originalText string) (translated string, summary string, err error)
	ExtractPaperProfile(markdown string) (*PaperProfileExtraction, error)
}

type paperSearchService interface {
	Search(query string, limit int) ([]SearchPaper, error)
	SearchWithContext(ctx context.Context, query string, limit int) ([]SearchPaper, error)
	EnhancedSearch(query string, limit int, offset int, yearStart int, yearEnd int, sortBy string) (*EnhancedSearchResult, error)
	LastSearchStats() SearchRetrievalStats
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
	tokenBudget            *dailyTokenBudget
}

type contextLLMService interface {
	TranslateSectionWithContext(ctx context.Context, section, originalText string) (translated string, summary string, err error)
	AnalyzeDeepStartWithContext(ctx context.Context, request DeepStartAIRequest) (*DeepStartAIResponse, error)
	AnalyzeScreeningWithContext(ctx context.Context, request ScreeningAIRequest) (*ScreeningDecisionNode, error)
}

type contextWeakLLMService interface {
	TranslateSectionWithContext(ctx context.Context, section, originalText string) (translated string, summary string, err error)
	ExtractPaperProfileWithContext(ctx context.Context, markdown string) (*PaperProfileExtraction, error)
}

type contextQueryRewriter interface {
	RewriteSearchQueriesWithContext(ctx context.Context, query string) ([]string, error)
}

type deepReadAssistant interface {
	AnswerDeepReadWithContext(
		ctx context.Context,
		paperTitle string,
		mode string,
		question string,
		sectionContext string,
	) (*DeepReadAIResponse, error)
}

func NewLLMClient(config AppConfig) *LLMClient {
	return NewStrongLLMClient(config)
}

func NewStrongLLMClient(config AppConfig) *LLMClient {
	llmConfig := normalizeLLMConfig(config.LLM)
	return newLLMClientFromConfig(llmConfig)
}

func NewWeakLLMClient(config AppConfig) *LLMClient {
	llmConfig := normalizeLLMConfig(config.WeakLLM)
	return newLLMClientFromConfig(llmConfig)
}

func newLLMClientFromConfig(llmConfig LLMConfig) *LLMClient {
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

func (c *LLMClient) ExtractPaperProfile(markdown string) (*PaperProfileExtraction, error) {
	return c.ExtractPaperProfileWithContext(context.Background(), markdown)
}

func (c *LLMClient) ExtractPaperProfileWithContext(ctx context.Context, markdown string) (*PaperProfileExtraction, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.safeProviderLabel())
	}

	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return nil, fmt.Errorf("markdown cannot be empty")
	}
	if len(markdown) > 24000 {
		markdown = markdown[:24000]
	}

	prompt := fmt.Sprintf(`
You are an academic paper information extractor.

Extract key information from the markdown content below and return valid JSON only:
{
  "title": "paper title",
  "authors": ["author a", "author b"],
  "abstract": "paper abstract",
  "problem": "2-3 sentence problem statement",
  "method": "2-3 sentence method summary",
  "keywords": ["keyword1", "keyword2", "keyword3"],
  "relevanceTags": ["tag1", "tag2", "tag3"],
  "topicLabel": "one short topic label",
  "methodLabel": "one short method label",
  "taskLabel": "one short task label",
  "domainLabel": "one short domain label",
  "classificationConfidence": 0.0
}

Rules:
- Keep keywords and relevanceTags concise.
- Keep each label short and concrete.
- classificationConfidence should be a number between 0 and 1.
- If a field is missing, return empty string or empty array.
- Return JSON only, no markdown wrapper.

Markdown:
%s
`, markdown)

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, err
	}

	var parsed PaperProfileExtraction
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse paper profile extraction response: %w", err)
	}

	parsed.Title = strings.TrimSpace(parsed.Title)
	parsed.Abstract = strings.TrimSpace(parsed.Abstract)
	parsed.Problem = strings.TrimSpace(parsed.Problem)
	parsed.Method = strings.TrimSpace(parsed.Method)
	parsed.Authors = compactStrings(parsed.Authors, 32)
	parsed.Keywords = compactStrings(parsed.Keywords, 16)
	parsed.RelevanceTags = compactStrings(parsed.RelevanceTags, 16)
	parsed.TopicLabel = strings.TrimSpace(parsed.TopicLabel)
	parsed.MethodLabel = strings.TrimSpace(parsed.MethodLabel)
	parsed.TaskLabel = strings.TrimSpace(parsed.TaskLabel)
	parsed.DomainLabel = strings.TrimSpace(parsed.DomainLabel)
	if parsed.ClassificationConfidence < 0 {
		parsed.ClassificationConfidence = 0
	}
	if parsed.ClassificationConfidence > 1 {
		parsed.ClassificationConfidence = 1
	}
	return &parsed, nil
}

func (c *LLMClient) RewriteSearchQueries(query string) ([]string, error) {
	return c.RewriteSearchQueriesWithContext(context.Background(), query)
}

func (c *LLMClient) RewriteSearchQueriesWithContext(ctx context.Context, query string) ([]string, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.safeProviderLabel())
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	prompt := fmt.Sprintf(`
You are a search query rewriting assistant for academic paper retrieval.

Convert the input query into concise English retrieval queries that work well for Semantic Scholar and arXiv.

Return valid JSON only:
{
  "primaryQuery": "english main query",
  "expandedQueries": ["english expansion 1", "english expansion 2"]
}

Rules:
- Keep each query short and keyword-centric.
- Avoid natural-language intent sentences.
- Prefer domain terms, methods, tasks, and benchmark-oriented keywords.
- Return up to 2 expanded queries.

Input query:
%s
`, query)

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, err
	}

	var parsed struct {
		PrimaryQuery    string   `json:"primaryQuery"`
		ExpandedQueries []string `json:"expandedQueries"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse rewritten query response: %w", err)
	}

	queries := make([]string, 0, 3)
	if primary := strings.TrimSpace(parsed.PrimaryQuery); primary != "" {
		queries = append(queries, primary)
	}
	queries = append(queries, compactStrings(parsed.ExpandedQueries, 2)...)
	queries = uniqueStrings(queries)
	if len(queries) == 0 {
		return nil, fmt.Errorf("empty rewritten queries")
	}
	return queries, nil
}

func (c *LLMClient) TranslateSection(section, originalText string) (translated string, summary string, err error) {
	return c.TranslateSectionWithContext(context.Background(), section, originalText)
}

func (c *LLMClient) TranslateSectionWithContext(ctx context.Context, section, originalText string) (translated string, summary string, err error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return "", "", fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return "", "", fmt.Errorf("missing model for %s", c.safeProviderLabel())
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

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
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

func (c *LLMClient) AnswerDeepReadWithContext(
	ctx context.Context,
	paperTitle string,
	mode string,
	question string,
	sectionContext string,
) (*DeepReadAIResponse, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.safeProviderLabel())
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "summary" && mode != "question" {
		return nil, fmt.Errorf("unsupported DeepRead AI mode")
	}
	question = strings.TrimSpace(question)
	if mode == "question" && question == "" {
		return nil, fmt.Errorf("question cannot be empty")
	}
	sectionContext = strings.TrimSpace(sectionContext)
	if sectionContext == "" {
		return nil, fmt.Errorf("paper context cannot be empty")
	}

	task := "Summarize the paper for a researcher. Explain the research question, method, evidence, conclusion, and limitations."
	if mode == "question" {
		task = "Answer the researcher's question using only the supplied paper context.\nQuestion: " + question
	}
	prompt := fmt.Sprintf(`
You are an academic reading assistant. Use only the supplied paper context.

Paper: %s
Task: %s

Rules:
- Answer in Chinese unless the question clearly requests another language.
- Separate what the paper states from your interpretation.
- If the context is insufficient, say exactly what is missing.
- Cite evidence with section IDs that exist in the context, such as "section-2".
- Keep evidence excerpts short and verbatim. Never invent a section ID or quotation.
- For summary mode, cover the research question, method, key evidence, conclusion, and practical reading order.
- Return valid JSON only, without a markdown wrapper.

Return this exact shape:
{
  "answer": "structured answer",
  "takeaway": "one concise takeaway",
  "evidence": [
    {
      "sectionId": "section-1",
      "sectionTitle": "Abstract",
      "excerpt": "short supporting excerpt"
    }
  ],
  "limitations": ["limitation or missing evidence"]
}

Paper context:
%s
`, strings.TrimSpace(paperTitle), task, sectionContext)

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, err
	}

	var parsed DeepReadAIResponse
	if err := json.Unmarshal([]byte(extractJSONObject(response)), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse DeepRead assistant response: %w", err)
	}
	parsed.Mode = mode
	parsed.Answer = strings.TrimSpace(parsed.Answer)
	parsed.Takeaway = strings.TrimSpace(parsed.Takeaway)
	parsed.Limitations = compactStrings(parsed.Limitations, 8)
	if parsed.Answer == "" {
		return nil, fmt.Errorf("empty DeepRead assistant response")
	}

	parsed.Evidence = groundDeepReadEvidence(parsed.Evidence, sectionContext)
	if len(parsed.Evidence) == 0 {
		parsed.Limitations = compactStrings(
			append(parsed.Limitations, "模型未返回可由当前论文上下文逐字验证的证据摘录。"),
			8,
		)
	}
	return &parsed, nil
}

type deepReadContextSection struct {
	title   string
	content string
}

func groundDeepReadEvidence(evidence []DeepReadEvidence, sectionContext string) []DeepReadEvidence {
	sections := parseDeepReadContextSections(sectionContext)
	clean := make([]DeepReadEvidence, 0, len(evidence))
	for _, item := range evidence {
		item.SectionID = strings.TrimSpace(item.SectionID)
		item.Excerpt = strings.TrimSpace(item.Excerpt)
		section, ok := sections[item.SectionID]
		if !ok || item.Excerpt == "" {
			continue
		}
		excerptRunes := []rune(item.Excerpt)
		if len(excerptRunes) > 240 {
			item.Excerpt = string(excerptRunes[:240])
		}
		if !strings.Contains(normalizeEvidenceText(section.content), normalizeEvidenceText(item.Excerpt)) {
			continue
		}
		item.SectionTitle = section.title
		clean = append(clean, item)
		if len(clean) >= 6 {
			break
		}
	}
	return clean
}

func parseDeepReadContextSections(sectionContext string) map[string]deepReadContextSection {
	sections := map[string]deepReadContextSection{}
	var currentID string
	var currentTitle string
	var content strings.Builder
	flush := func() {
		if currentID == "" {
			return
		}
		sections[currentID] = deepReadContextSection{
			title:   currentTitle,
			content: strings.TrimSpace(content.String()),
		}
		content.Reset()
	}
	for _, line := range strings.Split(sectionContext, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if closeIndex := strings.Index(trimmed, "] "); closeIndex > 1 {
				candidateID := strings.TrimSpace(trimmed[1:closeIndex])
				if strings.HasPrefix(candidateID, "section-") {
					flush()
					currentID = candidateID
					currentTitle = strings.TrimSpace(trimmed[closeIndex+2:])
					continue
				}
			}
		}
		if currentID != "" {
			content.WriteString(line)
			content.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func normalizeEvidenceText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func (c *LLMClient) AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	return c.AnalyzeDeepStartWithContext(context.Background(), request)
}

func (c *LLMClient) AnalyzeDeepStartWithContext(ctx context.Context, request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.safeProviderLabel())
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
- Prefer grouping by topicLabel/methodLabel/taskLabel/domainLabel when these fields are available.
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
  "recommendedPaperIds": ["paper-id-1"],
  "retainedPaperIds": ["paper-id-1"]
}

Context:
%s
`, string(contextJSON))

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
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
		RetainedPaperIDs    []string             `json:"retainedPaperIds"`
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
		RetainedPaperIDs:    parsed.RetainedPaperIDs,
	}, request)

	return &DeepStartAIResponse{
		Title:    normalizeDeepStartTitle(parsed.Title, request.RootPrompt, request.CurrentQuery),
		Analysis: analysis,
	}, nil
}

func (c *LLMClient) AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	return c.AnalyzeScreeningWithContext(context.Background(), request)
}

func (c *LLMClient) AnalyzeScreeningWithContext(ctx context.Context, request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	if c.requiresAPIKey() && strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("missing API key for %s", c.safeProviderLabel())
	}
	if strings.TrimSpace(c.model) == "" {
		return nil, fmt.Errorf("missing model for %s", c.safeProviderLabel())
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

	response, err := c.chatWithContext(ctx, []llmMessage{{Role: "user", Content: prompt}})
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
	return c.chatWithContext(context.Background(), messages)
}

func (c *LLMClient) chatWithContext(ctx context.Context, messages []llmMessage) (string, error) {
	switch c.wireAPI {
	case "responses":
		return c.chatResponsesWithContext(ctx, messages)
	case "chat_completions":
		return c.chatOpenAIWithContext(ctx, messages)
	case "anthropic_messages":
		return c.chatAnthropicWithContext(ctx, messages)
	default:
		return "", fmt.Errorf("unknown wire API for %s: %s", c.safeProviderLabel(), redactSensitiveText(c.wireAPI))
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

func (c *LLMClient) safeProviderLabel() string {
	return redactSensitiveText(c.providerLabel())
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
	return c.chatResponsesWithContext(context.Background(), messages)
}

func (c *LLMClient) chatResponsesWithContext(ctx context.Context, messages []llmMessage) (string, error) {
	reservation, err := c.reserveTokenBudget(messages, 8192)
	if err != nil {
		return "", err
	}
	defer reservation.Finish(0)

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL("/responses"), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%s responses request creation failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	if c.requiresOpenAIAuth {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s responses request failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	defer resp.Body.Close()

	body, err := readExternalHTTPBody(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s responses request failed: %s", c.safeProviderLabel(), redactSensitiveText(string(body)))
	}

	text, err := parseResponsesText(body)
	if err != nil {
		return "", err
	}
	var usage struct {
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(body, &usage)
	actualTokens := usage.Usage.TotalTokens
	if actualTokens <= 0 {
		actualTokens = usage.Usage.InputTokens + usage.Usage.OutputTokens
	}
	if actualTokens <= 0 {
		actualTokens = estimateMessageTokens(messages) + estimateTextTokens(text)
	}
	reservation.Finish(actualTokens)
	return text, nil
}

func (c *LLMClient) chatOpenAI(messages []llmMessage) (string, error) {
	return c.chatOpenAIWithContext(context.Background(), messages)
}

func (c *LLMClient) chatOpenAIWithContext(ctx context.Context, messages []llmMessage) (string, error) {
	reservation, err := c.reserveTokenBudget(messages, 8192)
	if err != nil {
		return "", err
	}
	defer reservation.Finish(0)

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL("/chat/completions"), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%s chat completions request creation failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	if c.requiresOpenAIAuth {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s chat completions request failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	defer resp.Body.Close()

	body, err := readExternalHTTPBody(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s chat completions request failed: %s", c.safeProviderLabel(), redactSensitiveText(string(body)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}

	content := result.Choices[0].Message.Content
	actualTokens := result.Usage.TotalTokens
	if actualTokens <= 0 {
		actualTokens = result.Usage.PromptTokens + result.Usage.CompletionTokens
	}
	if actualTokens <= 0 {
		actualTokens = estimateMessageTokens(messages) + estimateTextTokens(content)
	}
	reservation.Finish(actualTokens)
	return content, nil
}

func (c *LLMClient) chatAnthropic(messages []llmMessage) (string, error) {
	return c.chatAnthropicWithContext(context.Background(), messages)
}

func (c *LLMClient) chatAnthropicWithContext(ctx context.Context, messages []llmMessage) (string, error) {
	reservation, err := c.reserveTokenBudget(messages, 4096)
	if err != nil {
		return "", err
	}
	defer reservation.Finish(0)

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL("/messages"), bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("%s messages request creation failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s messages request failed: %s", c.safeProviderLabel(), redactURLQueryValuesInText(err.Error()))
	}
	defer resp.Body.Close()

	body, err := readExternalHTTPBody(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s messages request failed: %s", c.safeProviderLabel(), redactSensitiveText(string(body)))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
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

	content := text.String()
	actualTokens := result.Usage.InputTokens + result.Usage.OutputTokens
	if actualTokens <= 0 {
		actualTokens = estimateMessageTokens(messages) + estimateTextTokens(content)
	}
	reservation.Finish(actualTokens)
	return content, nil
}

func (c *LLMClient) reserveTokenBudget(messages []llmMessage, maxOutputTokens int64) (*tokenReservation, error) {
	if c.tokenBudget == nil {
		return &tokenReservation{}, nil
	}
	reservation, err := c.tokenBudget.Reserve(messages, maxOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("%s request blocked: %w", c.safeProviderLabel(), err)
	}
	return reservation, nil
}

func estimateTextTokens(text string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	var asciiRunes int64
	var nonASCIIRunes int64
	for _, value := range text {
		if value <= unicode.MaxASCII {
			asciiRunes++
		} else {
			nonASCIIRunes++
		}
	}
	return (asciiRunes+3)/4 + nonASCIIRunes
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
		RetainedPaperIDs:    filterExistingPaperIDs(analysis.RetainedPaperIDs, validPaperIDs),
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

func buildSearchQueryCandidates(query string) []string {
	original := strings.TrimSpace(query)
	if original == "" {
		return []string{""}
	}

	candidates := []string{original}
	lowered := strings.ToLower(original)
	lowered = normalizeSearchDelimiters(lowered)

	tokens := strings.Fields(lowered)
	keywords := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if !isQueryKeywordToken(token) {
			continue
		}
		if _, stop := searchEnglishStopWords[token]; stop {
			continue
		}
		if utf8Len(token) <= 1 {
			continue
		}
		keywords = append(keywords, token)
		if len(keywords) >= 10 {
			break
		}
	}
	if len(keywords) > 0 {
		candidates = append(candidates, strings.Join(keywords, " "))
	}

	// Mixed-language prompts benefit from a provider-neutral ASCII fallback.
	asciiKeywords := extractASCIISearchKeywords(original, 8)
	if len(asciiKeywords) > 0 {
		candidates = append(candidates, strings.Join(asciiKeywords, " "))
	}

	return uniqueStrings(candidates)
}

func isQueryKeywordToken(token string) bool {
	if token == "" {
		return false
	}
	hasASCIIAlphaNum := false
	hasNonASCII := false
	for _, r := range token {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r <= unicode.MaxASCII {
				hasASCIIAlphaNum = true
			} else {
				hasNonASCII = true
			}
		}
	}
	// 过滤掉“model的论文”这类中英混写 token，避免把整句意图词带进检索变体。
	if hasASCIIAlphaNum && hasNonASCII {
		return false
	}
	return true
}

func extractASCIISearchKeywords(value string, limit int) []string {
	if limit <= 0 || strings.TrimSpace(value) == "" {
		return nil
	}

	keywords := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		token := strings.ToLower(strings.TrimSpace(current.String()))
		current.Reset()
		if token == "" || utf8Len(token) <= 1 {
			return
		}
		if _, stop := searchEnglishStopWords[token]; stop {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		keywords = append(keywords, token)
	}

	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
			continue
		}
		flush()
		if len(keywords) >= limit {
			break
		}
	}
	flush()

	if len(keywords) > limit {
		keywords = keywords[:limit]
	}
	return keywords
}

func normalizeExternalIDMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	clean := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		clean[key] = value
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func normalizeExternalIDMapFromAny(values map[string]any) map[string]string {
	if len(values) == 0 {
		return nil
	}

	clean := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}

		var normalized string
		switch typed := value.(type) {
		case string:
			normalized = strings.TrimSpace(typed)
		case json.Number:
			normalized = strings.TrimSpace(typed.String())
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) {
				continue
			}
			if typed == math.Trunc(typed) {
				normalized = strconv.FormatInt(int64(typed), 10)
			} else {
				normalized = strconv.FormatFloat(typed, 'f', -1, 64)
			}
		case float32:
			f := float64(typed)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				continue
			}
			if f == math.Trunc(f) {
				normalized = strconv.FormatInt(int64(f), 10)
			} else {
				normalized = strconv.FormatFloat(f, 'f', -1, 64)
			}
		case int:
			normalized = strconv.Itoa(typed)
		case int64:
			normalized = strconv.FormatInt(typed, 10)
		case int32:
			normalized = strconv.FormatInt(int64(typed), 10)
		case int16:
			normalized = strconv.FormatInt(int64(typed), 10)
		case int8:
			normalized = strconv.FormatInt(int64(typed), 10)
		case uint:
			normalized = strconv.FormatUint(uint64(typed), 10)
		case uint64:
			normalized = strconv.FormatUint(typed, 10)
		case uint32:
			normalized = strconv.FormatUint(uint64(typed), 10)
		case uint16:
			normalized = strconv.FormatUint(uint64(typed), 10)
		case uint8:
			normalized = strconv.FormatUint(uint64(typed), 10)
		case bool:
			normalized = strconv.FormatBool(typed)
		default:
			normalized = strings.TrimSpace(fmt.Sprint(typed))
		}

		if normalized == "" {
			continue
		}
		clean[key] = normalized
	}

	if len(clean) == 0 {
		return nil
	}
	return clean
}

func externalIDValue(values map[string]string, key string) string {
	if len(values) == 0 {
		return ""
	}
	for candidateKey, candidateValue := range values {
		if strings.EqualFold(strings.TrimSpace(candidateKey), strings.TrimSpace(key)) {
			return strings.TrimSpace(candidateValue)
		}
	}
	return ""
}

func buildInitialPDFCandidates(urlValue string, externalIDs map[string]string) []string {
	candidates := make([]string, 0, 4)
	appendCandidate := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		for _, existing := range candidates {
			if existing == raw {
				return
			}
		}
		candidates = append(candidates, raw)
	}

	if trimmed := strings.TrimSpace(urlValue); trimmed != "" {
		appendCandidate(trimmed)
		if strings.Contains(strings.ToLower(trimmed), "arxiv.org/abs/") {
			appendCandidate(arxivAbsToPDFURL(trimmed))
		}
	}

	if arxivID := extractArxivID(externalIDValue(externalIDs, "ArXiv")); arxivID != "" {
		appendCandidate(fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", arxivID))
	}

	if doi := strings.TrimSpace(externalIDValue(externalIDs, "DOI")); doi != "" {
		appendCandidate("https://doi.org/" + strings.TrimPrefix(doi, "doi:"))
	}

	return candidates
}

func searchQueryVariant(candidates []string, attempt int) string {
	if len(candidates) == 0 {
		return ""
	}
	if attempt <= 0 {
		return candidates[0]
	}
	idx := (attempt - 1) % len(candidates)
	return candidates[idx]
}

func normalizeSearchDelimiters(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune(' ')
	}
	return builder.String()
}

func utf8Len(value string) int {
	return len([]rune(value))
}

type SearchClient struct {
	semanticScholarAPIKey string
	openAlexAPIKey        string
	enableSemanticScholar bool
	enableArxiv           bool
	enableOpenAlex        bool
	enableOpenReview      bool
	enableDBLP            bool
	perSourceResultLimit  int
	retryDuration         time.Duration
	retryInterval         time.Duration
	attemptTimeout        time.Duration
	retryMax              int
	overallTimeout        time.Duration
	httpClient            *http.Client
	progressMu            sync.RWMutex
	progressCallbackMu    sync.Mutex
	progressReporter      func(SearchProgressEvent)
	statsMu               sync.RWMutex
	lastSearchStats       SearchRetrievalStats
	lastSearchSources     []SearchSourceProgress
}

type searchContextKey string

const searchContextPerSourceLimitKey searchContextKey = "per_source_limit_override"

const (
	searchHTTPTimeout       = 12 * time.Second
	searchOverallTimeoutPad = 10 * time.Second
)

var searchEnglishStopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "i": {}, "me": {}, "my": {}, "we": {}, "our": {},
	"want": {}, "need": {}, "to": {}, "in": {}, "of": {}, "for": {}, "on": {}, "at": {},
	"with": {}, "from": {}, "and": {}, "or": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "field": {}, "papers": {}, "paper": {}, "sort": {},
	"out": {}, "over": {}, "past": {}, "last": {}, "two": {}, "year": {}, "years": {},
	"recent": {}, "about": {}, "this": {}, "that": {}, "it": {}, "as": {}, "by": {},
}

const (
	searchSourceSemantic = "Semantic Scholar"
	searchSourceArxiv    = "arXiv"
	searchSourceOpenAlex = "OpenAlex"
	searchSourceReview   = "OpenReview"
	searchSourceDBLP     = "DBLP"
)

var searchSourceOrder = []string{
	searchSourceOpenAlex,
	searchSourceArxiv,
	searchSourceReview,
	searchSourceDBLP,
	searchSourceSemantic,
}

type searchSourceDefinition struct {
	name   string
	search func(context.Context, string, int, func(string, int, bool, bool, int, error)) ([]SearchPaper, error)
}

type SearchSourceProgress struct {
	Name        string `json:"name"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"maxAttempts"`
	Status      string `json:"status"` // "pending" | "retrying" | "success" | "failed"
	Success     bool   `json:"success"`
	Done        bool   `json:"done"`
	ResultCount int    `json:"resultCount"`
	Error       string `json:"error,omitempty"`
}

type SearchProgressEvent struct {
	Query            string                 `json:"query"`
	ElapsedSeconds   int                    `json:"elapsedSeconds"`
	TotalSeconds     int                    `json:"totalSeconds"`
	CompletedSources int                    `json:"completedSources"`
	TotalSources     int                    `json:"totalSources"`
	Sources          []SearchSourceProgress `json:"sources"`
	Phase            string                 `json:"phase"` // "searching" | "completed"
	Message          string                 `json:"message,omitempty"`
}

type searchProviderHTTPError struct {
	statusCode int
	detail     string
	retryAfter time.Duration
}

func (e *searchProviderHTTPError) Error() string {
	if strings.TrimSpace(e.detail) == "" {
		return fmt.Sprintf("request failed (%d)", e.statusCode)
	}
	return fmt.Sprintf("request failed (%d): %s", e.statusCode, e.detail)
}

func isRetryableSearchError(err error) bool {
	var httpErr *searchProviderHTTPError
	if !errors.As(err, &httpErr) {
		return true
	}
	return httpErr.statusCode == http.StatusRequestTimeout ||
		httpErr.statusCode == http.StatusTooEarly ||
		httpErr.statusCode == http.StatusTooManyRequests ||
		httpErr.statusCode >= http.StatusInternalServerError
}

func NewSearchClient(config AppConfig) *SearchClient {
	searchConfig := normalizeSearchAPIConfig(config.Search)
	retryDuration := time.Duration(searchConfig.RetryDurationSeconds) * time.Second
	retryInterval := time.Duration(searchConfig.RetryIntervalSeconds) * time.Second
	attemptTimeout := time.Duration(searchConfig.RequestTimeoutSeconds) * time.Second
	if retryInterval <= 0 {
		retryInterval = time.Second
	}
	if attemptTimeout <= 0 {
		attemptTimeout = 5 * time.Second
	}
	if attemptTimeout > searchHTTPTimeout {
		attemptTimeout = searchHTTPTimeout
	}
	if retryDuration < retryInterval {
		retryDuration = retryInterval
	}
	retryMax := int(retryDuration / retryInterval)
	if retryMax < 1 {
		retryMax = 1
	}
	if retryMax > 8 {
		retryMax = 8
	}

	return &SearchClient{
		semanticScholarAPIKey: searchConfig.SemanticScholarAPIKey,
		openAlexAPIKey:        searchConfig.OpenAlexAPIKey,
		enableSemanticScholar: searchConfig.EnableSemanticScholar,
		enableArxiv:           searchConfig.EnableArxiv,
		enableOpenAlex:        searchConfig.EnableOpenAlex,
		enableOpenReview:      searchConfig.EnableOpenReview,
		enableDBLP:            searchConfig.EnableDBLP,
		perSourceResultLimit:  searchConfig.PerSourceResultLimit,
		retryDuration:         retryDuration,
		retryInterval:         retryInterval,
		attemptTimeout:        attemptTimeout,
		retryMax:              retryMax,
		overallTimeout:        retryDuration + searchOverallTimeoutPad,
		httpClient:            &http.Client{Timeout: searchHTTPTimeout},
	}
}

func (s *SearchClient) enabledSources() []searchSourceDefinition {
	sources := make([]searchSourceDefinition, 0, len(searchSourceOrder))
	if s.enableOpenAlex {
		sources = append(sources, searchSourceDefinition{name: searchSourceOpenAlex, search: s.searchOpenAlex})
	}
	if s.enableArxiv {
		sources = append(sources, searchSourceDefinition{name: searchSourceArxiv, search: s.searchArXiv})
	}
	if s.enableOpenReview {
		sources = append(sources, searchSourceDefinition{name: searchSourceReview, search: s.searchOpenReview})
	}
	if s.enableDBLP {
		sources = append(sources, searchSourceDefinition{name: searchSourceDBLP, search: s.searchDBLP})
	}
	if s.enableSemanticScholar {
		sources = append(sources, searchSourceDefinition{name: searchSourceSemantic, search: s.searchSemanticScholar})
	}
	return sources
}

func (s *SearchClient) SetProgressReporter(reporter func(SearchProgressEvent)) {
	s.progressMu.Lock()
	s.progressReporter = reporter
	s.progressMu.Unlock()
}

func (s *SearchClient) LastSearchStats() SearchRetrievalStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.lastSearchStats
}

func (s *SearchClient) Search(query string, limit int) ([]SearchPaper, error) {
	return s.SearchWithContext(context.Background(), query, limit)
}

func (s *SearchClient) SearchWithPerSourceLimit(ctx context.Context, query string, limit int, perSourceLimit int) ([]SearchPaper, error) {
	if perSourceLimit <= 0 {
		return s.SearchWithContext(ctx, query, limit)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, searchContextPerSourceLimitKey, perSourceLimit)
	return s.SearchWithContext(ctx, query, limit)
}

func (s *SearchClient) SearchWithContext(ctx context.Context, query string, limit int) ([]SearchPaper, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
	s.updateLastSearchStats(SearchRetrievalStats{Query: query})
	s.updateLastSearchSources(nil)
	enabledSources := s.enabledSources()
	if len(enabledSources) == 0 {
		return nil, fmt.Errorf("no academic search source enabled in config/app.yaml")
	}

	var combined []SearchPaper
	var errs []string

	// 每源抓取上限与会话总上限解耦：DeepStart 可以总量 200，但单源默认抓取 100。
	searchLimit := s.perSourceResultLimit
	if override, ok := ctx.Value(searchContextPerSourceLimitKey).(int); ok && override > 0 {
		searchLimit = override
	}
	if searchLimit <= 0 {
		searchLimit = 100
	}
	if searchLimit > 100 {
		searchLimit = 100
	}

	startedAt := time.Now()

	sourceStates := map[string]SearchSourceProgress{}
	for _, source := range enabledSources {
		sourceStates[source.name] = SearchSourceProgress{
			Name:        source.name,
			MaxAttempts: s.retryMax,
			Status:      "pending",
		}
	}
	var sourceStateMu sync.Mutex

	updateProgress := func(sourceName string, attempt int, success bool, done bool, count int, err error) {
		sourceStateMu.Lock()
		state := sourceStates[sourceName]
		state.Attempt = attempt
		state.MaxAttempts = s.retryMax
		state.Success = success
		state.Done = done
		state.ResultCount = count
		if success {
			state.Status = "success"
			state.Error = ""
		} else {
			if done {
				state.Status = "failed"
			} else {
				state.Status = "retrying"
			}
			if err != nil {
				state.Error = redactSearchErrorText(err)
			}
		}
		sourceStates[sourceName] = state
		snapshot := cloneSearchSourceStates(sourceStates)
		sourceStateMu.Unlock()

		s.emitSearchProgress(SearchProgressEvent{
			Query:          query,
			ElapsedSeconds: elapsedSearchSeconds(startedAt, s.retryDuration),
			TotalSeconds:   int(s.retryDuration / time.Second),
			TotalSources:   len(snapshot),
			Sources:        snapshot,
			Phase:          "searching",
		})
	}

	s.emitSearchProgress(SearchProgressEvent{
		Query:          query,
		ElapsedSeconds: 0,
		TotalSeconds:   int(s.retryDuration / time.Second),
		TotalSources:   len(sourceStates),
		Sources:        cloneSearchSourceStates(sourceStates),
		Phase:          "searching",
	})

	type sourceResult struct {
		papers []SearchPaper
		err    error
		name   string
	}

	overallCtx, overallCancel := context.WithTimeout(ctx, s.overallTimeout)
	defer overallCancel()

	totalSources := len(enabledSources)
	results := make(chan sourceResult, totalSources)
	for _, source := range enabledSources {
		source := source
		go func() {
			papers, err := source.search(overallCtx, query, searchLimit, updateProgress)
			results <- sourceResult{papers: papers, err: err, name: source.name}
		}()
	}

	// 收集结果并设置总时限，避免极端情况下卡死。
	received := 0
	for received < totalSources {
		select {
		case result := <-results:
			received++
			if result.err == nil && len(result.papers) > 0 {
				combined = append(combined, result.papers...)
				continue
			}
			if result.err != nil {
				errs = append(errs, fmt.Sprintf("%s: %s", result.name, redactSearchErrorText(result.err)))
				continue
			}
		case <-overallCtx.Done():
			if errorsIsDeadlineExceeded(overallCtx.Err()) {
				errs = append(errs, fmt.Sprintf("search timed out after %s", s.overallTimeout))
			} else {
				errs = append(errs, "search cancelled")
			}
			received = totalSources
		}
	}

	sourceStateMu.Lock()
	sourceSnapshot := cloneSearchSourceStates(sourceStates)
	sourceStateMu.Unlock()
	s.updateLastSearchSources(sourceSnapshot)

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	s.emitSearchProgress(SearchProgressEvent{
		Query:            query,
		ElapsedSeconds:   elapsedSearchSeconds(startedAt, s.retryDuration),
		TotalSeconds:     int(s.retryDuration / time.Second),
		TotalSources:     len(sourceSnapshot),
		CompletedSources: countCompletedSources(sourceSnapshot),
		Sources:          sourceSnapshot,
		Phase:            "completed",
	})

	summaryParts := make([]string, 0, len(enabledSources))
	for _, source := range enabledSources {
		summaryParts = append(summaryParts, fmt.Sprintf("%s=%s", source.name, formatSourceSummary(sourceSnapshot, source.name)))
	}
	log.Printf("[Search] summary: %s", strings.Join(summaryParts, " | "))

	rawCount := len(combined)
	papers := mergeDuplicateSearchPapers(combined)
	dedupCount := len(papers)

	// 只有在完全没有结果时才返回错误
	if len(papers) == 0 && len(errs) > 0 {
		s.updateLastSearchStats(SearchRetrievalStats{
			Query:      query,
			RawCount:   rawCount,
			DedupCount: dedupCount,
			FinalCount: 0,
		})
		return nil, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	// 如果有部分失败但至少有一个成功，在日志中记录但不返回错误
	if len(errs) > 0 && len(papers) > 0 {
		// 可以考虑在返回结果中包含警告信息
		// 但这里我们选择静默处理，让用户至少能看到部分结果
	}

	papers = filterSearchPapersByQueries(papers, []string{query})
	papers = rankSearchPapersByQueries(papers, []string{query})

	if len(papers) > limit {
		papers = papers[:limit]
	}

	s.updateLastSearchStats(SearchRetrievalStats{
		Query:      query,
		RawCount:   rawCount,
		DedupCount: dedupCount,
		FinalCount: len(papers),
	})

	return papers, nil
}

func (s *SearchClient) updateLastSearchStats(stats SearchRetrievalStats) {
	s.statsMu.Lock()
	s.lastSearchStats = stats
	s.statsMu.Unlock()
}

func (s *SearchClient) updateLastSearchSources(sources []SearchSourceProgress) {
	s.statsMu.Lock()
	s.lastSearchSources = append([]SearchSourceProgress(nil), sources...)
	s.statsMu.Unlock()
}

func (s *SearchClient) lastSourceSnapshot() []SearchSourceProgress {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return append([]SearchSourceProgress(nil), s.lastSearchSources...)
}

func (s *SearchClient) retrySourceSearch(
	ctx context.Context,
	sourceName string,
	fn func(attempt int) ([]SearchPaper, error),
	onAttempt func(sourceName string, attempt int, success bool, done bool, count int, err error),
) ([]SearchPaper, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	startedAt := time.Now()
	var lastErr error

	for attempt := 1; attempt <= s.retryMax; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attemptStarted := time.Now()
		papers, err := fn(attempt)
		if err == nil {
			log.Printf("[Search][%s] attempt %d/%d succeeded with %d papers", sourceName, attempt, s.retryMax, len(papers))
			if onAttempt != nil {
				onAttempt(sourceName, attempt, true, true, len(papers), nil)
			}
			return papers, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		done := attempt == s.retryMax
		if emptyErr, ok := err.(*emptyResultsVariantError); ok {
			candidateCount := emptyErr.candidateCount
			if candidateCount <= 0 {
				candidateCount = 1
			}
			if attempt >= candidateCount {
				done = true
				err = fmt.Errorf("no results after trying %d query variants", candidateCount)
			}
		} else if !isRetryableSearchError(err) {
			done = true
		}

		lastErr = err
		log.Printf("[Search][%s] attempt %d/%d failed: %s", sourceName, attempt, s.retryMax, redactSearchErrorText(err))
		if onAttempt != nil {
			onAttempt(sourceName, attempt, false, done, 0, err)
		}
		if done {
			return nil, fmt.Errorf("%s failed after %d retries in %v: %s", sourceName, attempt, time.Since(startedAt), redactSearchErrorText(lastErr))
		}
		if attempt < s.retryMax {
			waitDuration := searchRetryDelay(s.retryInterval, attempt, err)
			if wait := waitDuration - time.Since(attemptStarted); wait > 0 {
				if err := sleepWithContext(ctx, wait); err != nil {
					return nil, err
				}
			}
		}
	}

	return nil, fmt.Errorf("%s failed after %d retries in %v: %s", sourceName, s.retryMax, time.Since(startedAt), redactSearchErrorText(lastErr))
}

func searchRetryDelay(base time.Duration, attempt int, err error) time.Duration {
	if base <= 0 {
		return 0
	}
	delay := base
	for exponent := 1; exponent < attempt && delay < 5*time.Second; exponent++ {
		delay *= 2
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
	}
	var providerErr *searchProviderHTTPError
	if errors.As(err, &providerErr) && providerErr.retryAfter > delay {
		delay = providerErr.retryAfter
	}
	if delay > 15*time.Second {
		delay = 15 * time.Second
	}
	return delay
}

func redactSearchErrorText(err error) string {
	if err == nil {
		return ""
	}
	return redactURLQueryValuesInText(err.Error())
}

func (s *SearchClient) searchSemanticScholar(
	ctx context.Context,
	query string,
	limit int,
	onAttempt func(sourceName string, attempt int, success bool, done bool, count int, err error),
) ([]SearchPaper, error) {
	candidates := buildSearchQueryCandidates(query)
	return s.retrySourceSearch(ctx, searchSourceSemantic, func(attempt int) ([]SearchPaper, error) {
		variant := searchQueryVariant(candidates, attempt)
		papers := make([]SearchPaper, 0, limit)
		remaining := limit
		offset := 0

		for remaining > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			pageSize := minInt(100, remaining)
			apiURL := fmt.Sprintf(
				"https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&offset=%d&fields=title,authors,abstract,year,venue,journal,openAccessPdf,citationCount,url,externalIds",
				url.QueryEscape(variant),
				pageSize,
				offset,
			)

			reqCtx, cancel := context.WithTimeout(ctx, s.attemptTimeout)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
			if err != nil {
				cancel()
				return nil, err
			}
			if s.semanticScholarAPIKey != "" {
				req.Header.Set("x-api-key", s.semanticScholarAPIKey)
			}
			req.Header.Set("Accept", "application/json")

			resp, err := s.httpClient.Do(req)
			if err != nil {
				cancel()
				return nil, err
			}
			body, readErr := readExternalHTTPBody(resp.Body)
			resp.Body.Close()
			cancel()
			if readErr != nil {
				return nil, readErr
			}

			if resp.StatusCode >= 400 {
				return nil, &searchProviderHTTPError{
					statusCode: resp.StatusCode,
					detail:     redactSensitiveText(string(body)),
					retryAfter: searchRetryAfter(resp.Header),
				}
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
					CitationCount int            `json:"citationCount"`
					URL           string         `json:"url"`
					ExternalIDs   map[string]any `json:"externalIds"`
				} `json:"data"`
			}

			if err := json.Unmarshal(body, &result); err != nil {
				return nil, err
			}
			if len(result.Data) == 0 {
				break
			}

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

				externalIDs := normalizeExternalIDMapFromAny(item.ExternalIDs)
				openAccessURL := ""
				if item.OpenAccessPDF != nil {
					openAccessURL = strings.TrimSpace(item.OpenAccessPDF.URL)
				}
				landingURL := strings.TrimSpace(item.URL)
				urlValue := openAccessURL
				if urlValue == "" {
					urlValue = landingURL
				}
				pdfCandidates := buildInitialPDFCandidates(openAccessURL, externalIDs)

				papers = append(papers, SearchPaper{
					ID:               item.PaperID,
					Title:            item.Title,
					Authors:          strings.Join(authors, ", "),
					Abstract:         item.Abstract,
					Year:             item.Year,
					Journal:          journal,
					PublicationVenue: journal,
					PublicationYear:  item.Year,
					CitationCount:    item.CitationCount,
					URL:              urlValue,
					Tags:             []string{},
					Source:           "semantic_scholar",
					Sources:          []string{"semantic_scholar"},
					ExternalIDs:      externalIDs,
					PDFCandidates:    pdfCandidates,
					SourceLabel:      "Semantic Scholar",
				})
			}

			if len(result.Data) < pageSize {
				break
			}
			remaining = limit - len(papers)
			offset += len(result.Data)

			if remaining > 0 {
				if err := sleepWithContext(ctx, s.retryInterval); err != nil {
					return nil, err
				}
			}
		}

		if len(papers) == 0 {
			return nil, &emptyResultsVariantError{
				variant:        variant,
				candidateCount: len(candidates),
			}
		}
		if len(papers) > limit {
			papers = papers[:limit]
		}
		return papers, nil
	}, onAttempt)
}

func (s *SearchClient) searchArXiv(
	ctx context.Context,
	query string,
	limit int,
	onAttempt func(sourceName string, attempt int, success bool, done bool, count int, err error),
) ([]SearchPaper, error) {
	candidates := buildSearchQueryCandidates(query)
	return s.retrySourceSearch(ctx, searchSourceArxiv, func(attempt int) ([]SearchPaper, error) {
		variant := searchQueryVariant(candidates, attempt)
		apiURL := fmt.Sprintf(
			"https://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d",
			url.QueryEscape(variant),
			limit,
		)

		reqCtx, cancel := context.WithTimeout(ctx, s.attemptTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := readExternalHTTPBody(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			return nil, &searchProviderHTTPError{
				statusCode: resp.StatusCode,
				detail:     redactSensitiveText(string(body)),
				retryAfter: searchRetryAfter(resp.Header),
			}
		}

		papers, err := parseArXivXML(body)
		if err != nil {
			return nil, err
		}
		if len(papers) == 0 {
			return nil, &emptyResultsVariantError{
				variant:        variant,
				candidateCount: len(candidates),
			}
		}
		return papers, nil
	}, onAttempt)
}

func (s *SearchClient) emitSearchProgress(progress SearchProgressEvent) {
	s.progressMu.RLock()
	reporter := s.progressReporter
	s.progressMu.RUnlock()
	if reporter == nil {
		return
	}
	progress.CompletedSources = countCompletedSources(progress.Sources)
	s.progressCallbackMu.Lock()
	defer s.progressCallbackMu.Unlock()
	reporter(progress)
}

func cloneSearchSourceStates(state map[string]SearchSourceProgress) []SearchSourceProgress {
	cloned := make([]SearchSourceProgress, 0, len(state))
	for _, name := range searchSourceOrder {
		if value, ok := state[name]; ok {
			cloned = append(cloned, value)
		}
	}
	return cloned
}

func countCompletedSources(sources []SearchSourceProgress) int {
	completed := 0
	for _, source := range sources {
		if source.Done {
			completed++
		}
	}
	return completed
}

func elapsedSearchSeconds(startedAt time.Time, totalDuration time.Duration) int {
	if startedAt.IsZero() {
		return 0
	}
	elapsed := int(time.Since(startedAt).Seconds())
	if elapsed < 0 {
		return 0
	}
	total := int(totalDuration / time.Second)
	if total <= 0 {
		total = 1
	}
	if elapsed > total {
		return total
	}
	return elapsed
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func errorsIsDeadlineExceeded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func formatSourceSummary(sources []SearchSourceProgress, sourceName string) string {
	for _, source := range sources {
		if source.Name != sourceName {
			continue
		}
		if source.Success {
			return fmt.Sprintf("SUCCESS(count=%d)", source.ResultCount)
		}
		if strings.TrimSpace(source.Error) != "" {
			return fmt.Sprintf("FAILED(error=%s)", source.Error)
		}
		return strings.ToUpper(source.Status)
	}
	return "UNKNOWN"
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
			ID:               shortID,
			Title:            title,
			Authors:          strings.Join(entry.Authors, ", "),
			Abstract:         strings.Join(strings.Fields(entry.Summary), " "),
			Year:             year,
			Journal:          "arXiv",
			PublicationVenue: "arXiv",
			PublicationYear:  year,
			CitationCount:    0,
			URL:              id,
			PDFCandidates:    []string{fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", shortID)},
			Tags:             []string{},
			Source:           "arxiv",
			Sources:          []string{"arxiv"},
			ExternalIDs:      map[string]string{"ArXiv": normalizeArxivID(shortID)},
			SourceLabel:      "arXiv",
		})
	}

	return papers, nil
}

func dedupeSearchPaperKey(paper SearchPaper) string {
	keys := dedupeSearchPaperKeys(paper)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func normalizedDedupeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

type emptyResultsVariantError struct {
	variant        string
	candidateCount int
}

func (e *emptyResultsVariantError) Error() string {
	return fmt.Sprintf("empty results for query variant %q", e.variant)
}

func isSearchPaperPreferred(candidate SearchPaper, existing SearchPaper) bool {
	return searchPaperQualityScore(candidate) > searchPaperQualityScore(existing)
}

func searchPaperQualityScore(paper SearchPaper) int {
	score := 0
	if strings.TrimSpace(paper.URL) != "" {
		score += 20
	}
	if strings.TrimSpace(paper.Abstract) != "" {
		score += 10
	}
	if strings.TrimSpace(paper.Authors) != "" {
		score += 6
	}
	if strings.TrimSpace(paper.Journal) != "" {
		score += 4
	}
	if strings.TrimSpace(paper.ID) != "" {
		score += 3
	}
	score += minInt(len(paper.ExternalIDs)*2, 8)
	score += minInt(len(paper.PDFCandidates)*2, 6)
	score += minInt(len(paper.Sources)*2, 8)
	score += len(strings.TrimSpace(paper.Abstract)) / 80
	score += len(paper.Tags)
	return score
}

func rankSearchPapersByQueries(papers []SearchPaper, queries []string) []SearchPaper {
	tokens := make([]string, 0, 24)
	phrases := make([]string, 0, len(queries))
	primaryTokens := map[string]struct{}{}
	for queryIndex, query := range queries {
		normalized := strings.ToLower(strings.TrimSpace(query))
		if normalized == "" {
			continue
		}
		phrases = append(phrases, normalized)
		queryTokens := extractDeepStartMessageTokens(normalized)
		tokens = append(tokens, queryTokens...)
		if queryIndex == 0 {
			for _, token := range queryTokens {
				primaryTokens[token] = struct{}{}
			}
		}
	}
	tokens = uniqueStrings(tokens)

	score := func(paper SearchPaper) (int, string, []string) {
		title := strings.ToLower(strings.TrimSpace(paper.Title))
		abstract := strings.ToLower(strings.TrimSpace(paper.Abstract))
		value := 0
		titlePhrases := make([]string, 0, 2)
		abstractPhrases := make([]string, 0, 2)
		titleTerms := make([]string, 0, 6)
		abstractTerms := make([]string, 0, 6)
		for index, phrase := range phrases {
			titleWeight := 30
			abstractWeight := 12
			if index == 0 {
				titleWeight = 120
				abstractWeight = 40
			}
			if strings.Contains(title, phrase) {
				value += titleWeight
				titlePhrases = append(titlePhrases, phrase)
			} else if strings.Contains(abstract, phrase) {
				value += abstractWeight
				abstractPhrases = append(abstractPhrases, phrase)
			}
		}
		for _, token := range tokens {
			titleWeight := 18
			abstractWeight := 5
			if _, primary := primaryTokens[token]; primary {
				titleWeight = 36
				abstractWeight = 10
			}
			if textMatchesSearchTerm(title, token) {
				value += titleWeight
				titleTerms = append(titleTerms, token)
			}
			if textMatchesSearchTerm(abstract, token) {
				value += abstractWeight
				abstractTerms = append(abstractTerms, token)
			}
		}
		matchedTerms := uniqueStrings(append(titleTerms, abstractTerms...))
		if len(primaryTokens) > 0 {
			primaryMatches := 0
			for _, token := range matchedTerms {
				if _, ok := primaryTokens[token]; ok {
					primaryMatches++
				}
			}
			value += primaryMatches * 12
			value += primaryMatches * 48 / len(primaryTokens)
		}
		if len(paper.Sources) > 1 {
			value += minInt(len(paper.Sources)-1, 3) * 4
		}
		if paper.CitationCount > 0 {
			value += minInt(int(math.Log10(float64(paper.CitationCount)+1)*3), 12)
		}
		value += minInt(searchPaperQualityScore(paper)/10, 8)
		if len(matchedTerms) > 6 {
			matchedTerms = matchedTerms[:6]
		}
		return value, buildSearchPaperMatchReason(titlePhrases, abstractPhrases, titleTerms, abstractTerms), matchedTerms
	}

	type scoredPaper struct {
		paper SearchPaper
		score int
	}
	scored := make([]scoredPaper, 0, len(papers))
	for _, paper := range papers {
		value, reason, matchedTerms := score(paper)
		paper.MatchReason = reason
		paper.MatchedTerms = matchedTerms
		scored = append(scored, scoredPaper{paper: paper, score: value})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if scored[i].paper.Year != scored[j].paper.Year {
			return scored[i].paper.Year > scored[j].paper.Year
		}
		return strings.ToLower(scored[i].paper.Title) < strings.ToLower(scored[j].paper.Title)
	})
	ranked := make([]SearchPaper, 0, len(scored))
	for _, item := range scored {
		ranked = append(ranked, item.paper)
	}
	return ranked
}

func buildSearchPaperMatchReason(titlePhrases, abstractPhrases, titleTerms, abstractTerms []string) string {
	if len(titlePhrases) > 0 {
		return fmt.Sprintf("标题直接匹配检索短语「%s」。", titlePhrases[0])
	}
	titleTerms = uniqueStrings(titleTerms)
	abstractTerms = uniqueStrings(abstractTerms)
	if len(titleTerms) > 0 {
		return "标题命中关键词：" + strings.Join(compactStrings(titleTerms, 4), "、") + "。"
	}
	if len(abstractPhrases) > 0 {
		return fmt.Sprintf("摘要直接匹配检索短语「%s」。", abstractPhrases[0])
	}
	if len(abstractTerms) > 0 {
		return "摘要命中关键词：" + strings.Join(compactStrings(abstractTerms, 4), "、") + "。"
	}
	return "由论文来源召回，当前按年份与元数据完整度辅助排序。"
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
		// Search already returns the unified relevance order.
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

	sourceSnapshot := s.lastSourceSnapshot()
	sourceStatuses := make([]SearchSourceStatus, 0, len(sourceSnapshot))
	for _, source := range sourceSnapshot {
		sourceStatuses = append(sourceStatuses, SearchSourceStatus{
			Name:    source.Name,
			Success: source.Success,
			Error:   source.Error,
			Count:   source.ResultCount,
		})
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
		Sources:   sourceStatuses,
	}

	if start < end {
		result.Papers = filtered[start:end]
	} else {
		result.Papers = []SearchPaper{}
	}

	return result, nil
}
