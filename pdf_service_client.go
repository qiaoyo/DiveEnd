package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// PDFServiceClient 用于与 PDF 解析服务通信
type PDFServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPDFServiceClient 创建新的 PDF 服务客户端
func NewPDFServiceClient(baseURL string) *PDFServiceClient {
	return &PDFServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PDFParseResponse PDF 解析响应
type PDFParseResponse struct {
	Success bool   `json:"success"`
	Text    string `json:"text"`
	Message string `json:"message"`
}

// ExtractResponse 提取响应
type ExtractResponse struct {
	Success bool                   `json:"success"`
	Title   string                 `json:"title"`
	Authors string                 `json:"authors"`
	Abstract string                 `json:"abstract"`
	Sections []map[string]any       `json:"sections"`
	Message string                 `json:"message"`
}

// ParsePDF 解析 PDF 文件并提取文本
func (c *PDFServiceClient) ParsePDF(filePath string) (*PDFParseResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/parse/upload", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PDF service returned status %d", resp.StatusCode)
	}

	var result PDFParseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ExtractContent 使用 LLM 提取结构化内容
func (c *PDFServiceClient) ExtractContent(text string) (*ExtractResponse, error) {
	reqBody := map[string]string{
		"text": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/extract/", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extract service returned status %d", resp.StatusCode)
	}

	var result ExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
