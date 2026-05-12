package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type screeningLLMStub struct{}

func (screeningLLMStub) TranslateSection(section, originalText string) (string, string, error) {
	return "译文", "摘要", nil
}

func (screeningLLMStub) AnalyzeDeepStart(request DeepStartAIRequest) (*DeepStartAIResponse, error) {
	return &DeepStartAIResponse{}, nil
}

func (screeningLLMStub) AnalyzeScreening(request ScreeningAIRequest) (*ScreeningDecisionNode, error) {
	mid := len(request.Papers) / 2
	leftIDs := make([]string, 0, mid)
	rightIDs := make([]string, 0, len(request.Papers)-mid)

	for index, paper := range request.Papers {
		if index < mid {
			leftIDs = append(leftIDs, paper.ID)
		} else {
			rightIDs = append(rightIDs, paper.ID)
		}
	}

	return &ScreeningDecisionNode{
		ID:        "screen-node-stub",
		NodeType:  "branch",
		Message:   "按研究子方向筛选",
		Dimension: "研究子方向",
		Options: []ScreeningDecisionOption{
			{Key: "left", Label: "左分支", PaperIDs: leftIDs, Count: len(leftIDs)},
			{Key: "right", Label: "右分支", PaperIDs: rightIDs, Count: len(rightIDs)},
		},
		AllowMultiSelect: false,
		AllowSkip:        false,
	}, nil
}

func TestAppScreeningFlowImportsSelectedPapers(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/parse/upload":
			_ = json.NewEncoder(w).Encode(PDFParseResponse{
				Success:  true,
				Markdown: "# Example\n\n## Abstract\nA paper",
				Metadata: map[string]any{"title": "Example"},
				Sections: []string{"Example", "  Abstract"},
			})
		case "/extract/":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload["api_key"] != "weak-test-key" {
				t.Fatalf("expected weak llm api key to be forwarded, got %#v", payload["api_key"])
			}
			_ = json.NewEncoder(w).Encode(PDFExtractResponse{
				Success: true,
				Data: &PDFExtractData{
					Metadata: PDFExtractMetadata{
						Title:    "Example",
						Authors:  []string{"Author One"},
						Abstract: "A paper",
					},
					RelevanceTags: []string{"agents", "screening"},
				},
				Provider: "openai",
				Model:    "gpt-4o-mini",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app.pdfService = NewPDFServiceClient(server.URL)
	app.config.WeakLLM = defaultOpenAICompatibleLLMConfig()
	app.config.WeakLLM.APIKey = "weak-test-key"
	app.config.WeakLLM.Model = "weak-model"
	app.llm = screeningLLMStub{}

	var filePaths []string
	for index := 0; index < 6; index++ {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("paper-%d.pdf", index))
		if err := os.WriteFile(path, []byte("%PDF-1.4 mock"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		filePaths = append(filePaths, path)
	}

	session, err := app.CreateScreeningSession("Robotics")
	if err != nil {
		t.Fatalf("CreateScreeningSession() error = %v", err)
	}

	detail, err := app.UploadScreeningFiles(session.ID, filePaths)
	if err != nil {
		t.Fatalf("UploadScreeningFiles() error = %v", err)
	}
	if len(detail.Papers) != 6 {
		t.Fatalf("expected 6 uploaded papers, got %d", len(detail.Papers))
	}

	progress, err := app.ExtractPaperContent(session.ID)
	if err != nil {
		t.Fatalf("ExtractPaperContent() error = %v", err)
	}
	if progress.Status != "completed" || progress.Completed != 6 {
		t.Fatalf("unexpected extraction progress: %+v", progress)
	}

	node, err := app.AnalyzePapers(session.ID)
	if err != nil {
		t.Fatalf("AnalyzePapers() error = %v", err)
	}
	if node.NodeType != "branch" || len(node.Options) != 2 {
		t.Fatalf("unexpected screening node: %+v", node)
	}

	nextNode, err := app.ApplyScreeningChoice(session.ID, []string{node.Options[0].Key})
	if err != nil {
		t.Fatalf("ApplyScreeningChoice() error = %v", err)
	}
	if nextNode.NodeType != "complete" {
		t.Fatalf("expected completion node, got %+v", nextNode)
	}

	imported, err := app.CompleteScreening(session.ID, "")
	if err != nil {
		t.Fatalf("CompleteScreening() error = %v", err)
	}
	if len(imported) != 3 {
		t.Fatalf("expected 3 imported papers, got %d", len(imported))
	}
	if len(imported[0].Tags) != 2 || imported[0].Category != "agents" {
		t.Fatalf("expected extracted tags to be imported, got %+v", imported[0])
	}

	papers, err := app.db.GetPapers(imported[0].FolderID)
	if err != nil {
		t.Fatalf("GetPapers() error = %v", err)
	}
	if len(papers) != 3 {
		t.Fatalf("expected 3 papers in library folder, got %d", len(papers))
	}
}

func TestDeleteScreeningSessionCascadesPapers(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	session := &ScreeningSession{
		ID:                  "session-1",
		Title:               "Cascade",
		Status:              "upload",
		SelectedOptionsJSON: "[]",
		PathHistoryJSON:     "[]",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := db.UpsertScreeningSession(session); err != nil {
		t.Fatalf("UpsertScreeningSession() error = %v", err)
	}
	if err := db.UpsertScreeningPaper(&ScreeningPaper{
		ID:        "paper-1",
		SessionID: session.ID,
		FileName:  "paper.pdf",
		FilePath:  "/tmp/paper.pdf",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertScreeningPaper() error = %v", err)
	}

	if err := db.DeleteScreeningSession(session.ID); err != nil {
		t.Fatalf("DeleteScreeningSession() error = %v", err)
	}

	papers, err := db.GetScreeningPapers(session.ID)
	if err != nil {
		t.Fatalf("GetScreeningPapers() error = %v", err)
	}
	if len(papers) != 0 {
		t.Fatalf("expected screening papers to cascade delete, got %d", len(papers))
	}
}

func TestSyncStatusAggregatesStoredRecords(t *testing.T) {
	app := NewApp()
	config := defaultAppConfig()
	config.DataPath = t.TempDir()
	config.BaiduCloud.Enabled = false
	if err := app.applyConfig(config, true); err != nil {
		t.Fatalf("applyConfig() error = %v", err)
	}
	t.Cleanup(func() { _ = app.db.Close() })

	if err := app.db.SaveSyncRecord(&SyncRecord{
		Type:        "upload",
		FileName:    "paper-1.pdf",
		Status:      "success",
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		CompletedAt: time.Now().Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveSyncRecord(success) error = %v", err)
	}
	if err := app.db.SaveSyncRecord(&SyncRecord{
		Type:         "upload",
		FileName:     "paper-2.pdf",
		Status:       "failed",
		ErrorMessage: "network down",
		CreatedAt:    time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SaveSyncRecord(failed) error = %v", err)
	}
	if err := app.db.SaveSyncConflict(&SyncConflict{
		FileName:   "notes.md",
		LocalPath:  "/tmp/local",
		LocalTime:  time.Now().Add(-time.Hour),
		RemotePath: "/remote/notes.md",
		RemoteTime: time.Now(),
		CreatedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("SaveSyncConflict() error = %v", err)
	}

	status, err := app.GetSyncStatus()
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v", err)
	}
	if status.TotalSynced != 1 || status.TotalFailed != 1 {
		t.Fatalf("unexpected sync counters: %+v", status)
	}
	if status.Conflicts != 1 {
		t.Fatalf("expected 1 unresolved conflict, got %+v", status)
	}

	records, err := app.GetSyncRecords(10)
	if err != nil {
		t.Fatalf("GetSyncRecords() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 sync records, got %d", len(records))
	}
	if !strings.Contains(records[0].FileName, "paper") {
		t.Fatalf("unexpected sync record order: %+v", records)
	}
}
