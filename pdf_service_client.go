package main

import (
	"bytes"
	"context"
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

var pdfServiceUploadMaxBytes int64 = 50 * 1024 * 1024
var pdfServiceExtractionMaxMarkdownBytes int64 = 2 * 1024 * 1024

func NewPDFServiceClient(baseURL string) *PDFServiceClient {
	return &PDFServiceClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (c *PDFServiceClient) Status(ctx context.Context) *PDFServiceStatus {
	status := &PDFServiceStatus{
		Enabled:   c != nil && strings.TrimSpace(c.baseURL) != "",
		CheckedAt: time.Now(),
		Checks:    map[string]string{},
	}
	if c == nil || strings.TrimSpace(c.baseURL) == "" {
		status.Message = "PDF 服务未配置"
		return status
	}
	status.URL = c.baseURL
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health/ready", nil)
	if err != nil {
		status.Message = redactErrorText(err)
		return status
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		status.Message = redactErrorText(err)
		return status
	}
	defer resp.Body.Close()

	var payload struct {
		Ready  bool                   `json:"ready"`
		Checks map[string]interface{} `json:"checks"`
	}
	if err := decodePDFServiceJSON(resp.Body, &payload); err != nil {
		status.Message = fmt.Sprintf("PDF 服务响应不可解析: %v", err)
		return status
	}
	status.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 300
	status.Ready = payload.Ready
	for key, value := range payload.Checks {
		status.Checks[key] = fmt.Sprint(value)
	}
	if !status.Healthy {
		status.Message = fmt.Sprintf("PDF 服务 HTTP %d", resp.StatusCode)
	} else if !status.Ready {
		status.Message = "PDF 服务未 ready"
	} else {
		status.Message = "PDF 服务 ready"
	}
	return status
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
	Usage    PDFTokenUsage   `json:"usage"`
}

type PDFTokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

type PDFExtractionLLMConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	APIKey      string  `json:"api_key,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Timeout     int     `json:"timeout,omitempty"`
}

func (c *PDFServiceClient) ParsePDF(filePath string) (*PDFParseResponse, error) {
	return c.ParsePDFWithContext(context.Background(), filePath)
}

func (c *PDFServiceClient) ParsePDFWithContext(ctx context.Context, filePath string) (*PDFParseResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	body, contentType, uploadErrCh, err := streamingPDFUploadBody(filePath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/parse/upload", body)
	if err != nil {
		discardPDFUploadStream(body, uploadErrCh)
		return nil, fmt.Errorf("failed to create parse request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = -1

	resp, err := c.httpClient.Do(req)
	if err != nil {
		discardPDFUploadStream(body, uploadErrCh)
		return nil, classifyPDFServiceCallError(c.baseURL, "parse", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		serviceErr := decodeServiceError("parse", resp)
		discardPDFUploadStream(body, uploadErrCh)
		return nil, serviceErr
	}

	var result PDFParseResponse
	if err := decodePDFServiceJSON(resp.Body, &result); err != nil {
		discardPDFUploadStream(body, uploadErrCh)
		return nil, fmt.Errorf("failed to decode parse response: %w", err)
	}
	if err := finishPDFUploadStream(body, uploadErrCh); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("pdf parse failed: %s", redactSensitiveText(result.Error))
	}

	return &result, nil
}

func streamingPDFUploadBody(filePath string) (io.ReadCloser, string, <-chan error, error) {
	file, err := openPDFServiceUploadFile(filePath)
	if err != nil {
		return nil, "", nil, err
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	errCh := make(chan error, 1)

	go func() {
		var streamErr error
		defer file.Close()
		defer func() {
			if closeErr := multipartWriter.Close(); streamErr == nil && closeErr != nil {
				streamErr = closeErr
			}
			if streamErr != nil {
				_ = writer.CloseWithError(streamErr)
			} else {
				_ = writer.Close()
			}
			errCh <- streamErr
		}()

		part, err := multipartWriter.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			streamErr = fmt.Errorf("failed to create upload body: %w", err)
			return
		}
		written, err := io.Copy(part, io.LimitReader(file, pdfServiceUploadMaxBytes+1))
		if err != nil {
			streamErr = fmt.Errorf("failed to stream PDF file: %w", err)
			return
		}
		if written > pdfServiceUploadMaxBytes {
			streamErr = fmt.Errorf("PDF file exceeds upload limit: %d bytes > %d bytes", written, pdfServiceUploadMaxBytes)
			return
		}
		if err := multipartWriter.WriteField("extract_sections", "true"); err != nil {
			streamErr = fmt.Errorf("failed to set extract_sections: %w", err)
			return
		}
	}()

	return reader, multipartWriter.FormDataContentType(), errCh, nil
}

func openPDFServiceUploadFile(filePath string) (*os.File, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("PDF file path cannot be empty")
	}
	if !strings.EqualFold(filepath.Ext(filePath), ".pdf") {
		return nil, fmt.Errorf("PDF service upload source is not a PDF")
	}

	linkInfo, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat PDF file: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("PDF service upload source is a symbolic link")
	}
	if !linkInfo.Mode().IsRegular() || linkInfo.Size() == 0 {
		return nil, fmt.Errorf("PDF service upload source is empty or invalid")
	}
	if linkInfo.Size() > pdfServiceUploadMaxBytes {
		return nil, fmt.Errorf("PDF file exceeds upload limit: %d bytes > %d bytes", linkInfo.Size(), pdfServiceUploadMaxBytes)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to stat opened PDF file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		_ = file.Close()
		return nil, fmt.Errorf("PDF service upload source is empty or invalid")
	}
	if !os.SameFile(linkInfo, info) {
		_ = file.Close()
		return nil, fmt.Errorf("PDF service upload source changed while opening")
	}
	if info.Size() > pdfServiceUploadMaxBytes {
		_ = file.Close()
		return nil, fmt.Errorf("PDF file exceeds upload limit: %d bytes > %d bytes", info.Size(), pdfServiceUploadMaxBytes)
	}

	header := make([]byte, 5)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		_ = file.Close()
		return nil, fmt.Errorf("failed to read PDF header: %w", err)
	}
	if n < len(header) || string(header) != "%PDF-" {
		_ = file.Close()
		return nil, fmt.Errorf("PDF service upload source is not a valid PDF")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to reset PDF stream: %w", err)
	}
	return file, nil
}

func finishPDFUploadStream(body io.Closer, errCh <-chan error) error {
	_ = body.Close()
	if err := <-errCh; err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return err
	}
	return nil
}

func discardPDFUploadStream(body io.Closer, errCh <-chan error) {
	_ = body.Close()
	<-errCh
}

func (c *PDFServiceClient) ExtractContent(markdown string, llmConfig PDFExtractionLLMConfig) (*PDFExtractResponse, error) {
	return c.ExtractContentWithContext(context.Background(), markdown, llmConfig)
}

func (c *PDFServiceClient) ExtractContentWithContext(ctx context.Context, markdown string, llmConfig PDFExtractionLLMConfig) (*PDFExtractResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	llmConfig = normalizePDFExtractionLLMConfig(llmConfig)
	if err := validatePDFExtractionLLMConfig(llmConfig); err != nil {
		return nil, err
	}
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return nil, fmt.Errorf("pdf extraction markdown is empty")
	}
	if int64(len(markdown)) > pdfServiceExtractionMaxMarkdownBytes {
		return nil, fmt.Errorf("pdf extraction markdown exceeds limit: %d bytes > %d bytes", len(markdown), pdfServiceExtractionMaxMarkdownBytes)
	}

	requestBody := map[string]any{
		"markdown":        markdown,
		"extraction_type": "all",
		"provider":        llmConfig.Provider,
		"model":           llmConfig.Model,
		"api_key":         llmConfig.APIKey,
		"base_url":        llmConfig.BaseURL,
		"max_tokens":      llmConfig.MaxTokens,
		"temperature":     llmConfig.Temperature,
		"timeout":         llmConfig.Timeout,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal extraction request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/extract/", bytes.NewReader(jsonBody))
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
	if err := decodePDFServiceJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode extraction response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("pdf extraction failed: %s", redactSensitiveText(result.Error))
	}
	if result.Data == nil {
		return nil, fmt.Errorf("pdf extraction failed: empty data payload")
	}

	return &result, nil
}

func validatePDFExtractionLLMConfig(config PDFExtractionLLMConfig) error {
	if strings.TrimSpace(config.APIKey) == "" {
		return fmt.Errorf("pdf extraction LLM API key is not configured for provider %q; configure Weak LLM API Key in Settings > Credentials, or configure the legacy strong LLM key used as fallback", config.Provider)
	}
	if strings.TrimSpace(config.Model) == "" {
		return fmt.Errorf("pdf extraction LLM model is not configured for provider %q", config.Provider)
	}
	return nil
}

func normalizePDFExtractionLLMConfig(config PDFExtractionLLMConfig) PDFExtractionLLMConfig {
	config.Provider = strings.TrimSpace(strings.ToLower(config.Provider))
	if config.Provider != "anthropic" {
		config.Provider = "openai"
	}
	config.Model = strings.TrimSpace(config.Model)
	if config.Model == "" {
		if config.Provider == "anthropic" {
			config.Model = "claude-sonnet-4-20250514"
		} else {
			config.Model = "gpt-4o-mini"
		}
	}
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.MaxTokens <= 0 {
		config.MaxTokens = 4096
	}
	if config.Timeout <= 0 {
		config.Timeout = 120
	}
	return config
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
			"failed to call PDF %s service: 无法连接 %s。请先启动 PDF 服务：cd services/pdf_service && pip install -r requirements.txt && uvicorn app.main:app --reload --host 127.0.0.1 --port 50051",
			operation,
			redactSensitiveText(baseURL),
		)
	}

	return fmt.Errorf("failed to call PDF %s service: %w", operation, err)
}

func decodeServiceError(operation string, resp *http.Response) error {
	payload, readErr := readServiceErrorHTTPBody(resp.Body)
	if readErr != nil {
		return fmt.Errorf("%s service returned %d: %v", operation, resp.StatusCode, readErr)
	}

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
			return fmt.Errorf("%s service returned %d: %s", operation, resp.StatusCode, redactSensitiveText(message))
		}
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		trimmed = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("%s service returned %d: %s", operation, resp.StatusCode, redactSensitiveText(trimmed))
}
