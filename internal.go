package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ==================== Database ====================

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(dataPath string) (*DB, error) {
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataPath, "diveend.db")
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS papers (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			authors TEXT,
			abstract TEXT,
			year INTEGER,
			journal TEXT,
			pdf_path TEXT,
			category TEXT,
			tags TEXT,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT
		)
	`)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// Paper represents an academic paper
type Paper struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Authors   string    `json:"authors"`
	Abstract  string    `json:"abstract"`
	Year      int       `json:"year"`
	Journal   string    `json:"journal"`
	PDFPath   string    `json:"pdf_path,omitempty"`
	Category  string    `json:"category"`
	Tags      string    `json:"tags"`
	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (db *DB) AddPaper(paper *Paper) error {
	_, err := db.conn.Exec(`
		INSERT INTO papers (id, title, authors, abstract, year, journal, pdf_path, category, tags, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, paper.ID, paper.Title, paper.Authors, paper.Abstract, paper.Year, paper.Journal,
		paper.PDFPath, paper.Category, paper.Tags, paper.AddedAt, paper.UpdatedAt)
	return err
}

func (db *DB) GetPapers() ([]Paper, error) {
	rows, err := db.conn.Query("SELECT id, title, authors, abstract, year, journal, pdf_path, category, tags, added_at, updated_at FROM papers ORDER BY added_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		var p Paper
		if err := rows.Scan(&p.ID, &p.Title, &p.Authors, &p.Abstract, &p.Year, &p.Journal,
			&p.PDFPath, &p.Category, &p.Tags, &p.AddedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		papers = append(papers, p)
	}
	return papers, nil
}

func (db *DB) DeletePaper(id string) error {
	_, err := db.conn.Exec("DELETE FROM papers WHERE id = ?", id)
	return err
}

func (db *DB) GetConfig(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetConfig(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// ==================== LLM ====================

// LLMClient is an LLM client
type LLMClient struct {
	apiKey     string
	model      string
	provider   string
	baseURL    string
	httpClient *http.Client
}

// NewLLMClient creates a new LLM client
func NewLLMClient(apiKey, model, provider string) *LLMClient {
	var baseURL string
	if provider == "openai" {
		baseURL = "https://api.openai.com/v1/chat/completions"
	} else {
		baseURL = "https://api.anthropic.com/v1/messages"
	}

	return &LLMClient{
		apiKey: apiKey,
		model:  model,
		provider: provider,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// LLMMessage represents a chat message
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat sends a chat request and returns the response
func (c *LLMClient) Chat(messages []LLMMessage) (string, error) {
	switch c.provider {
	case "openai":
		return c.chatOpenAI(messages)
	case "anthropic":
		return c.chatAnthropic(messages)
	default:
		return "", fmt.Errorf("unknown provider: %s", c.provider)
	}
}

type openAIRequest struct {
	Model    string      `json:"model"`
	Messages []LLMMessage `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *LLMClient) chatOpenAI(messages []LLMMessage) (string, error) {
	reqBody := openAIRequest{
		Model:    c.model,
		Messages: messages,
	}

	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", c.baseURL, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("openai error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}

	return result.Choices[0].Message.Content, nil
}

type anthropicRequest struct {
	Model     string      `json:"model"`
	Messages  []LLMMessage `json:"messages"`
	MaxTokens int         `json:"max_tokens"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *LLMClient) chatAnthropic(messages []LLMMessage) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 16000,
	}

	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", c.baseURL, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result anthropicResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %s", string(body))
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("anthropic error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from anthropic")
	}

	var fullText string
	for _, content := range result.Content {
		if content.Type == "text" {
			fullText += content.Text
		}
	}

	return fullText, nil
}

// Translate translates text using LLM
func (c *LLMClient) Translate(text, targetLang string) (string, error) {
	prompt := fmt.Sprintf("Translate the following text to %s:\n\n%s", targetLang, text)
	messages := []LLMMessage{{Role: "user", Content: prompt}}
	return c.Chat(messages)
}

// ==================== Search ====================

// SearchClient handles paper search
type SearchClient struct {
	semanticScholarAPIKey string
	httpClient           *http.Client
}

// NewSearchClient creates a new search client
func NewSearchClient(apiKey string) *SearchClient {
	return &SearchClient{
		semanticScholarAPIKey: apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchPaper represents search result paper
type SearchPaper struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Abstract string `json:"abstract"`
	Year     int    `json:"year"`
	Journal  string `json:"journal"`
	Category string `json:"category"`
}

// Search searches for papers
func (s *SearchClient) Search(query string, limit int) ([]SearchPaper, error) {
	ssPapers, _ := s.searchSemanticScholar(query, limit)
	arXivPapers, _ := s.searchArXiv(query, limit)

	paperMap := make(map[string]SearchPaper)
	for _, p := range ssPapers {
		key := strings.ToLower(strings.TrimSpace(p.Title))
		paperMap[key] = p
	}
	for _, p := range arXivPapers {
		key := strings.ToLower(strings.TrimSpace(p.Title))
		if _, exists := paperMap[key]; !exists {
			paperMap[key] = p
		}
	}

	papers := make([]SearchPaper, 0, len(paperMap))
	for _, p := range paperMap {
		papers = append(papers, p)
	}

	if len(papers) > limit {
		papers = papers[:limit]
	}

	return papers, nil
}

func (s *SearchClient) searchSemanticScholar(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=%d&fields=title,authors,abstract,year,journal", url.QueryEscape(query), limit)

	req, _ := http.NewRequest("GET", apiURL, nil)
	if s.semanticScholarAPIKey != "" {
		req.Header.Set("x-api-key", s.semanticScholarAPIKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			PaperID  string `json:"paperId"`
			Title    string `json:"title"`
			Authors  []struct {
				Name string `json:"name"`
			} `json:"authors"`
			Abstract string `json:"abstract"`
			Year     int    `json:"year"`
			Journal  string `json:"journal"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	papers := make([]SearchPaper, 0, len(result.Data))
	for _, item := range result.Data {
		authors := ""
		for i, a := range item.Authors {
			if i > 0 {
				authors += ", "
			}
			authors += a.Name
		}

		papers = append(papers, SearchPaper{
			ID:       item.PaperID,
			Title:    item.Title,
			Authors:  authors,
			Abstract: item.Abstract,
			Year:     item.Year,
			Journal:  item.Journal,
		})
	}

	return papers, nil
}

func (s *SearchClient) searchArXiv(query string, limit int) ([]SearchPaper, error) {
	apiURL := fmt.Sprintf("http://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d", url.QueryEscape(query), limit)

	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return parseArXivXML(string(body))
}

func parseArXivXML(xml string) ([]SearchPaper, error) {
	var papers []SearchPaper
	entries := strings.Split(xml, "<entry>")

	for i, entry := range entries {
		if i == 0 {
			continue
		}

		extractField := func(tag string) string {
			start := strings.Index(entry, "<"+tag+">")
			if start == -1 {
				return ""
			}
			start += len(tag) + 2
			end := strings.Index(entry, "</"+tag+">")
			if end == -1 {
				return ""
			}
			return entry[start:end]
		}

		title := extractField("title")
		summary := extractField("summary")
		authors := extractField("author")
		published := extractField("published")

		year := 2024
		if len(published) >= 4 {
			fmt.Sscanf(published[:4], "%d", &year)
		}

		id := extractField("id")
		if idx := strings.LastIndex(id, "/"); idx != -1 {
			id = id[idx+1:]
		}

		if title != "" {
			papers = append(papers, SearchPaper{
				ID:       id,
				Title:    strings.ReplaceAll(title, "\n", " "),
				Authors:  strings.ReplaceAll(authors, "\n", " "),
				Abstract: strings.ReplaceAll(summary, "\n", " "),
				Year:     year,
				Journal:  "arXiv",
			})
		}
	}

	return papers, nil
}