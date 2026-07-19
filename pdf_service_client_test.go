package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFServiceClientParseAndExtractContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/parse/upload":
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			if got := r.FormValue("extract_sections"); got != "true" {
				t.Fatalf("expected extract_sections=true, got %q", got)
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("FormFile() error = %v", err)
			}
			defer file.Close()
			if header.Filename != "sample.pdf" {
				t.Fatalf("expected filename sample.pdf, got %q", header.Filename)
			}
			if _, err := io.ReadAll(file); err != nil {
				t.Fatalf("ReadAll(file) error = %v", err)
			}
			_ = json.NewEncoder(w).Encode(PDFParseResponse{
				Success:  true,
				Markdown: "# Example\n\n## Abstract",
				Metadata: map[string]any{"title": "Example"},
				Sections: []string{"Example", "  Abstract"},
			})

		case "/extract/":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["extraction_type"] != "all" {
				t.Fatalf("expected extraction_type=all, got %#v", payload["extraction_type"])
			}
			if payload["provider"] != "openai" {
				t.Fatalf("expected provider=openai, got %#v", payload["provider"])
			}
			if payload["model"] != "ark-code-latest" {
				t.Fatalf("expected configured model, got %#v", payload["model"])
			}
			if payload["api_key"] != "weak-key" {
				t.Fatalf("expected configured api key, got %#v", payload["api_key"])
			}
			if payload["base_url"] != "https://ark.cn-beijing.volces.com/api/coding/v3" {
				t.Fatalf("expected configured base_url, got %#v", payload["base_url"])
			}

			_ = json.NewEncoder(w).Encode(PDFExtractResponse{
				Success: true,
				Data: &PDFExtractData{
					Metadata: PDFExtractMetadata{
						Title:    "Example",
						Authors:  []string{"Author One"},
						Abstract: "Abstract text",
					},
					RelevanceTags: []string{"agents"},
				},
				Provider: "openai",
				Model:    "gpt-4o-mini",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient(server.URL)
	parseResult, err := client.ParsePDF(pdfPath)
	if err != nil {
		t.Fatalf("ParsePDF() error = %v", err)
	}
	if parseResult.Markdown == "" || len(parseResult.Sections) != 2 {
		t.Fatalf("unexpected parse result: %+v", parseResult)
	}

	extractResult, err := client.ExtractContent(parseResult.Markdown, PDFExtractionLLMConfig{
		Provider: "openai",
		Model:    "ark-code-latest",
		APIKey:   "weak-key",
		BaseURL:  "https://ark.cn-beijing.volces.com/api/coding/v3",
	})
	if err != nil {
		t.Fatalf("ExtractContent() error = %v", err)
	}
	if extractResult.Data == nil || extractResult.Data.Metadata.Title != "Example" {
		t.Fatalf("unexpected extract result: %+v", extractResult)
	}
}

func TestPDFServiceClientRejectsOversizedExtractionMarkdownBeforeRequest(t *testing.T) {
	previousLimit := pdfServiceExtractionMaxMarkdownBytes
	pdfServiceExtractionMaxMarkdownBytes = 16
	t.Cleanup(func() { pdfServiceExtractionMaxMarkdownBytes = previousLimit })

	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewPDFServiceClient(server.URL)
	_, err := client.ExtractContent(strings.Repeat("x", 17), PDFExtractionLLMConfig{
		Provider: "openai",
		Model:    "test-model",
		APIKey:   "test-key",
	})
	if err == nil || !strings.Contains(err.Error(), "markdown exceeds limit") {
		t.Fatalf("expected oversized markdown error, got %v", err)
	}
	if serverCalled {
		t.Fatal("PDF extraction service should not be called for oversized markdown")
	}
}

func TestPDFServiceClientParsePDFStreamsMultipartBody(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "streamed.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 streamed body"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient("http://pdf-service.local")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.ContentLength != -1 {
			t.Fatalf("expected streaming request with unknown content length, got %d", req.ContentLength)
		}
		if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data; boundary=") {
			t.Fatalf("expected multipart content type, got %q", got)
		}
		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}

		seenFile := false
		seenExtractSections := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}
			payload, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll(part) error = %v", err)
			}
			switch part.FormName() {
			case "file":
				seenFile = true
				if part.FileName() != "streamed.pdf" {
					t.Fatalf("expected streamed.pdf filename, got %q", part.FileName())
				}
				if string(payload) != "%PDF-1.4 streamed body" {
					t.Fatalf("unexpected streamed file payload %q", string(payload))
				}
			case "extract_sections":
				seenExtractSections = true
				if string(payload) != "true" {
					t.Fatalf("expected extract_sections=true, got %q", string(payload))
				}
			default:
				t.Fatalf("unexpected multipart field %q", part.FormName())
			}
		}
		if !seenFile || !seenExtractSections {
			t.Fatalf("missing multipart fields: file=%v extract_sections=%v", seenFile, seenExtractSections)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"success": true,
				"markdown": "# Streamed",
				"metadata": {},
				"sections": ["Streamed"]
			}`)),
			Request: req,
		}, nil
	})}

	parseResult, err := client.ParsePDF(pdfPath)
	if err != nil {
		t.Fatalf("ParsePDF() error = %v", err)
	}
	if parseResult.Markdown != "# Streamed" {
		t.Fatalf("unexpected parse result: %+v", parseResult)
	}
}

func TestPDFServiceClientRejectsUnsafeUploadFilesBeforeRequest(t *testing.T) {
	previousLimit := pdfServiceUploadMaxBytes
	pdfServiceUploadMaxBytes = int64(len("%PDF-1.4\n"))
	t.Cleanup(func() { pdfServiceUploadMaxBytes = previousLimit })

	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := NewPDFServiceClient(server.URL)

	tempDir := t.TempDir()
	notPDFPath := filepath.Join(tempDir, "paper.txt")
	if err := os.WriteFile(notPDFPath, []byte("%PDF-1.4\n"), 0600); err != nil {
		t.Fatalf("WriteFile not pdf error = %v", err)
	}
	fakePDFPath := filepath.Join(tempDir, "fake.pdf")
	if err := os.WriteFile(fakePDFPath, []byte("not a pdf"), 0600); err != nil {
		t.Fatalf("WriteFile fake pdf error = %v", err)
	}
	oversizedPDFPath := filepath.Join(tempDir, "oversized.pdf")
	if err := os.WriteFile(oversizedPDFPath, []byte("%PDF-1.4\noversized"), 0600); err != nil {
		t.Fatalf("WriteFile oversized pdf error = %v", err)
	}
	dirPDFPath := filepath.Join(tempDir, "dir.pdf")
	if err := os.Mkdir(dirPDFPath, 0700); err != nil {
		t.Fatalf("Mkdir dir pdf error = %v", err)
	}
	symlinkTarget := filepath.Join(tempDir, "target.pdf")
	if err := os.WriteFile(symlinkTarget, []byte("%PDF-1.4\n"), 0600); err != nil {
		t.Fatalf("WriteFile symlink target error = %v", err)
	}
	symlinkPDFPath := filepath.Join(tempDir, "linked.pdf")
	if err := os.Symlink(symlinkTarget, symlinkPDFPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	cases := []struct {
		name        string
		path        string
		wantMessage string
	}{
		{name: "wrong extension", path: notPDFPath, wantMessage: "not a PDF"},
		{name: "fake pdf", path: fakePDFPath, wantMessage: "not a valid PDF"},
		{name: "oversized pdf", path: oversizedPDFPath, wantMessage: "exceeds upload limit"},
		{name: "directory", path: dirPDFPath, wantMessage: "empty or invalid"},
		{name: "symlink", path: symlinkPDFPath, wantMessage: "symbolic link"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverCalled = false
			if _, err := client.ParsePDF(tc.path); err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q error, got %v", tc.wantMessage, err)
			}
			if serverCalled {
				t.Fatal("PDF service should not be called for unsafe local upload file")
			}
		})
	}
}

func TestPDFServiceClientHonorsCanceledContext(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.ParsePDFWithContext(ctx, pdfPath); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ParsePDFWithContext() context.Canceled, got %v", err)
	}

	if _, err := client.ExtractContentWithContext(ctx, "# Example", PDFExtractionLLMConfig{Provider: "openai", Model: "gpt-4o-mini", APIKey: "key"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected ExtractContentWithContext() context.Canceled, got %v", err)
	}

	select {
	case <-called:
		t.Fatal("server should not be called after context cancellation")
	default:
	}
}

func TestPDFServiceClientExtractContentRejectsBadPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(PDFExtractResponse{
			Success: true,
			Data:    nil,
		})
	}))
	defer server.Close()

	client := NewPDFServiceClient(server.URL)
	_, err := client.ExtractContent("# Example", PDFExtractionLLMConfig{Provider: "openai", Model: "gpt-4o-mini", APIKey: "key"})
	if err == nil {
		t.Fatal("expected ExtractContent() to fail on empty data payload")
	}
}

func TestPDFServiceClientExtractContentRequiresConfiguredAPIKey(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewPDFServiceClient(server.URL)
	_, err := client.ExtractContent("# Example", PDFExtractionLLMConfig{Provider: "openai", Model: "gpt-4o-mini"})
	if err == nil {
		t.Fatal("expected ExtractContent() to fail without an API key")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected actionable API key error, got %v", err)
	}
	if called {
		t.Fatal("server should not be called when the extraction LLM API key is missing")
	}
}

func TestPDFServiceClientLimitsServiceErrorBody(t *testing.T) {
	previousLimit := serviceErrorBodyLimitBytes
	serviceErrorBodyLimitBytes = 16
	t.Cleanup(func() { serviceErrorBodyLimitBytes = previousLimit })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, strings.Repeat("x", 17), http.StatusInternalServerError)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient(server.URL)
	_, err := client.ParsePDF(pdfPath)
	if err == nil {
		t.Fatal("expected ParsePDF() to fail on oversized service error body")
	}
	if !strings.Contains(err.Error(), "response body exceeds 16 byte limit") {
		t.Fatalf("expected response body limit error, got %v", err)
	}
}

func TestPDFServiceClientRedactsServiceErrorBodies(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"service failed api_key=sk-pdf-http-secret token=pdf-http-token"}`))
		}))
		defer server.Close()

		client := NewPDFServiceClient(server.URL)
		_, err := client.ParsePDF(pdfPath)
		if err == nil {
			t.Fatal("expected ParsePDF() to fail")
		}
		message := err.Error()
		for _, leaked := range []string{"sk-pdf-http-secret", "pdf-http-token"} {
			if strings.Contains(message, leaked) {
				t.Fatalf("expected %q to be redacted from %q", leaked, message)
			}
		}
		if !strings.Contains(message, redactedValue) {
			t.Fatalf("expected redacted marker in %q", message)
		}
	})

	t.Run("success false", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(PDFParseResponse{
				Success: false,
				Error:   "parser failed access_token=parse-token client_secret=parse-client-secret",
			})
		}))
		defer server.Close()

		client := NewPDFServiceClient(server.URL)
		_, err := client.ParsePDF(pdfPath)
		if err == nil {
			t.Fatal("expected ParsePDF() to fail")
		}
		message := err.Error()
		for _, leaked := range []string{"parse-token", "parse-client-secret"} {
			if strings.Contains(message, leaked) {
				t.Fatalf("expected %q to be redacted from %q", leaked, message)
			}
		}
		if !strings.Contains(message, redactedValue) {
			t.Fatalf("expected redacted marker in %q", message)
		}
	})
}

func TestPDFServiceClientLimitsSuccessfulResponseBody(t *testing.T) {
	previousLimit := pdfServiceResponseBodyLimitBytes
	pdfServiceResponseBodyLimitBytes = 16
	t.Cleanup(func() { pdfServiceResponseBodyLimitBytes = previousLimit })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"markdown":"` + strings.Repeat("x", 17) + `","metadata":{},"sections":[]}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient(server.URL)
	_, err := client.ParsePDF(pdfPath)
	if err == nil {
		t.Fatal("expected ParsePDF() to fail on oversized successful response body")
	}
	if !strings.Contains(err.Error(), "response body exceeds 16 byte limit") {
		t.Fatalf("expected response body limit error, got %v", err)
	}
}

func TestParsePDFSendsMultipartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}

		foundFile := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}

			if part.FormName() == "file" {
				foundFile = true
			}
		}
		if !foundFile {
			t.Fatal("expected multipart file part to be present")
		}

		_ = json.NewEncoder(w).Encode(PDFParseResponse{Success: true, Markdown: "ok", Metadata: map[string]any{}, Sections: []string{}})
	}))
	defer server.Close()

	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient(server.URL)
	if _, err := client.ParsePDF(pdfPath); err != nil {
		t.Fatalf("ParsePDF() error = %v", err)
	}
}

func TestParsePDFFriendlyErrorWhenServiceUnavailable(t *testing.T) {
	tempDir := t.TempDir()
	pdfPath := filepath.Join(tempDir, "sample.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 mock"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewPDFServiceClient("http://127.0.0.1:1")
	_, err := client.ParsePDF(pdfPath)
	if err == nil {
		t.Fatal("expected ParsePDF() to fail when service is unavailable")
	}

	message := err.Error()
	if !strings.Contains(message, "services/pdf_service") {
		t.Fatalf("expected start hint in error message, got %q", message)
	}
	if !strings.Contains(message, "uvicorn app.main:app") {
		t.Fatalf("expected uvicorn start command in error message, got %q", message)
	}
	if !strings.Contains(message, "--host 127.0.0.1") {
		t.Fatalf("expected loopback host start command in error message, got %q", message)
	}
	if strings.Contains(message, "--host 0.0.0.0") {
		t.Fatalf("error message should not suggest public bind host: %q", message)
	}
}
