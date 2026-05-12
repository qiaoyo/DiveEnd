package main

import (
	"encoding/json"
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
}
