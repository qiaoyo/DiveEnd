package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PDFServiceClient talks to the FastAPI PDF parsing service.
type PDFServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPDFServiceClient(baseURL string) *PDFServiceClient {
	return &PDFServiceClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

type PDFParseResponse struct {
	Success  bool           `json:"success"`
	Markdown string         `json:"markdown"`
	Metadata map[string]any `json:"metadata"`
	Sections []string       `json:"sections"`
	Error    string         `json:"error"`
}

type PDFExtractMetadata struct {
	Title        string   `json:"title"`
	Authors      []string `json:"authors"`
	Affiliations []string `json:"affiliations"`
	Abstract     string   `json:"abstract"`
	Problem      string   `json:"problem"`
	Method       string   `json:"method"`
	GitHubURL    string   `json:"github_url"`
	ArxivURL     string   `json:"arxiv_url"`
	Keywords     []string `json:"keywords"`
}

type PDFExtractMetric struct {
	MetricName    string `json:"metric_name"`
	DatasetOrTask string `json:"dataset_or_task"`
	OursValue     string `json:"ours_value"`
	Unit          string `json:"unit"`
}

type PDFExtractBaseline struct {
	MetricName string `json:"metric_name"`
	MethodName string `json:"method_name"`
	Value      string `json:"value"`
}

type PDFExtractData struct {
	Metadata      PDFExtractMetadata   `json:"metadata"`
	Metrics       []PDFExtractMetric   `json:"metrics"`
	Baselines     []PDFExtractBaseline `json:"baselines"`
	RelevanceTags []string             `json:"relevance_tags"`
}

type PDFExtractResponse struct {
	Success  bool            `json:"success"`
	Data     *PDFExtractData `json:"data"`
	Error    string          `json:"error"`
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
}

func (c *PDFServiceClient) ParsePDF(filePath string) (*PDFParseResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create upload body: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy PDF file: %w", err)
	}
	if err := writer.WriteField("extract_sections", "true"); err != nil {
		return nil, fmt.Errorf("failed to set extract_sections: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize upload body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/parse/upload", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create parse request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyPDFServiceCallError(c.baseURL, "parse", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeServiceError("parse", resp)
	}

	var result PDFParseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode parse response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("pdf parse failed: %s", strings.TrimSpace(result.Error))
	}

	return &result, nil
}

func (c *PDFServiceClient) ExtractContent(markdown, provider string) (*PDFExtractResponse, error) {
	if strings.TrimSpace(provider) == "" {
		provider = "openai"
	}

	requestBody := map[string]any{
		"markdown":        markdown,
		"extraction_type": "all",
		"provider":        provider,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal extraction request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/extract/", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create extraction request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyPDFServiceCallError(c.baseURL, "extract", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeServiceError("extract", resp)
	}

	var result PDFExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode extraction response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("pdf extraction failed: %s", strings.TrimSpace(result.Error))
	}
	if result.Data == nil {
		return nil, fmt.Errorf("pdf extraction failed: empty data payload")
	}

	return &result, nil
}

func classifyPDFServiceCallError(baseURL, operation string, err error) error {
	if err == nil {
		return nil
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	var netErr net.Error
	if strings.Contains(message, "connection refused") ||
		strings.Contains(message, "no such host") ||
		strings.Contains(message, "network is unreachable") ||
		strings.Contains(message, "connection reset by peer") ||
		(errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf(
			"failed to call PDF %s service: 无法连接 %s。请先启动 PDF 服务：cd services/pdf_service && pip install -r requirements.txt && uvicorn app.main:app --reload --host 0.0.0.0 --port 50051",
			operation,
			baseURL,
		)
	}

	return fmt.Errorf("failed to call PDF %s service: %w", operation, err)
}

func decodeServiceError(operation string, resp *http.Response) error {
	payload, _ := io.ReadAll(resp.Body)

	var structured struct {
		Error      string `json:"error"`
		Detail     string `json:"detail"`
		StatusCode int    `json:"status_code"`
	}
	if err := json.Unmarshal(payload, &structured); err == nil {
		message := strings.TrimSpace(structured.Error)
		if message == "" {
			message = strings.TrimSpace(structured.Detail)
		}
		if message != "" {
			return fmt.Errorf("%s service returned %d: %s", operation, resp.StatusCode, message)
		}
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		trimmed = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("%s service returned %d: %s", operation, resp.StatusCode, trimmed)
}
