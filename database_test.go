package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewDBMigratesLegacySchemaAndCreatesDefaultFolder(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "diveend.db")

	legacyDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = legacyDB.Close() })

	_, err = legacyDB.Exec(`
		CREATE TABLE papers (
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
		t.Fatalf("creating legacy papers table: %v", err)
	}
	_, err = legacyDB.Exec(`
		INSERT INTO papers (id, title, authors, abstract, year, journal, category, tags, added_at, updated_at)
		VALUES ('legacy-paper', 'Legacy Paper', 'Legacy Author', 'Legacy abstract', 2024, 'arXiv', 'legacy', '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("inserting legacy paper: %v", err)
	}
	_ = legacyDB.Close()

	db, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	columns := map[string]bool{}
	rows, err := db.conn.Query(`PRAGMA table_info(papers)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info() error = %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		columns[name] = true
	}

	if !columns["url"] || !columns["folder_id"] || !columns["source_paper_id"] || !columns["download_status"] || !columns["download_error"] {
		t.Fatalf("expected migrated papers table to include url/folder/source/download columns, got %v", columns)
	}

	folders, err := db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}
	if len(folders) != 1 || folders[0].Name != defaultFolderName {
		t.Fatalf("expected a single %s folder, got %+v", defaultFolderName, folders)
	}

	papers, err := db.GetPapers(folders[0].ID)
	if err != nil {
		t.Fatalf("GetPapers() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected migrated paper to be assigned to default folder, got %d papers", len(papers))
	}
	if papers[0].FolderID != folders[0].ID {
		t.Fatalf("expected migrated paper folder ID %q, got %q", folders[0].ID, papers[0].FolderID)
	}
	if papers[0].SourcePaperID != papers[0].ID {
		t.Fatalf("expected migrated paper source_paper_id to backfill as id, got %q", papers[0].SourcePaperID)
	}
	if papers[0].DownloadStatus != "queued" {
		t.Fatalf("expected migrated paper download status queued, got %q", papers[0].DownloadStatus)
	}
}

func TestPaperAndTranslationPersistence(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	folder, err := db.CreateFolder("Agents")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	paper := &Paper{
		ID:             "paper-1",
		SourcePaperID:  "source-paper-1",
		Title:          "Paper 1",
		Authors:        "Author",
		Abstract:       "Abstract",
		Year:           2025,
		Journal:        "arXiv",
		URL:            "https://example.com/paper-1",
		FolderID:       folder.ID,
		Category:       "agents",
		Tags:           []string{"llm", "agent"},
		DownloadStatus: "queued",
		AddedAt:        time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.UpsertPaper(paper); err != nil {
		t.Fatalf("UpsertPaper() error = %v", err)
	}

	papers, err := db.GetPapers(folder.ID)
	if err != nil {
		t.Fatalf("GetPapers() error = %v", err)
	}
	if len(papers) != 1 {
		t.Fatalf("expected one paper, got %d", len(papers))
	}
	if len(papers[0].Tags) != 2 {
		t.Fatalf("expected tags to round-trip, got %+v", papers[0].Tags)
	}
	if papers[0].SourcePaperID != "source-paper-1" {
		t.Fatalf("expected sourcePaperID to round-trip, got %q", papers[0].SourcePaperID)
	}
	if papers[0].DownloadStatus != "queued" {
		t.Fatalf("expected download status to round-trip, got %q", papers[0].DownloadStatus)
	}

	record := &TranslationRecord{
		PaperID:        paper.ID,
		Section:        "Abstract",
		OriginalText:   "Hello",
		TranslatedText: "你好",
		Summary:        "摘要",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.SaveTranslation(record); err != nil {
		t.Fatalf("SaveTranslation() error = %v", err)
	}

	translations, err := db.GetTranslations(paper.ID)
	if err != nil {
		t.Fatalf("GetTranslations() error = %v", err)
	}
	if len(translations) != 1 {
		t.Fatalf("expected one translation, got %d", len(translations))
	}
	if translations[0].Summary != "摘要" {
		t.Fatalf("expected summary to round-trip, got %q", translations[0].Summary)
	}
}

func TestGetPaperByFolderAndSourceAndStats(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	folder, err := db.CreateFolder("Benchmarks")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}

	now := time.Now()
	if err := db.UpsertPaper(&Paper{
		ID:             "paper-a",
		SourcePaperID:  "source-a",
		Title:          "Paper A",
		FolderID:       folder.ID,
		DownloadStatus: "downloading",
		AddedAt:        now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("UpsertPaper(paper-a) error = %v", err)
	}
	if err := db.UpsertPaper(&Paper{
		ID:             "paper-b",
		SourcePaperID:  "source-b",
		Title:          "Paper B",
		FolderID:       folder.ID,
		DownloadStatus: "failed",
		DownloadError:  "timeout",
		AddedAt:        now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("UpsertPaper(paper-b) error = %v", err)
	}

	paper, err := db.GetPaperByFolderAndSource(folder.ID, "source-a")
	if err != nil {
		t.Fatalf("GetPaperByFolderAndSource() error = %v", err)
	}
	if paper.ID != "paper-a" {
		t.Fatalf("expected paper-a, got %q", paper.ID)
	}

	if err := db.UpdatePaperDownloadState("paper-a", "downloaded", "/tmp/paper-a.pdf", ""); err != nil {
		t.Fatalf("UpdatePaperDownloadState() error = %v", err)
	}

	stats, err := db.GetFolderPaperStats(folder.ID)
	if err != nil {
		t.Fatalf("GetFolderPaperStats() error = %v", err)
	}
	if stats.Total != 2 || stats.Downloaded != 1 || stats.Failed != 1 {
		t.Fatalf("unexpected stats %+v", stats)
	}
}

func TestDeepStartSessionPersistence(t *testing.T) {
	dataDir := t.TempDir()

	db, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}

	folders, err := db.GetFolders()
	if err != nil {
		t.Fatalf("GetFolders() error = %v", err)
	}

	detail := &DeepStartSessionDetail{
		Summary: DeepStartSessionSummary{
			ID:             "session-1",
			Title:          "LLM Agents",
			RootPrompt:     "llm agents",
			CurrentQuery:   "llm agents benchmark",
			TargetFolderID: folders[0].ID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		CurrentResults: []SearchPaper{
			{ID: "paper-1", Title: "Agents Survey", Year: 2024},
		},
		CurrentAnalysis: &DeepStartAnalysis{
			Overview: "测试 overview",
			Directions: []DeepStartDirection{
				{
					ID:       "mainline",
					Name:     "主线",
					Summary:  "代表作",
					Why:      "适合先看",
					PaperIDs: []string{"paper-1"},
				},
			},
			PaperNotes: []DeepStartPaperNote{
				{PaperID: "paper-1", Tier: "core", Reason: "入口材料", DirectionIDs: []string{"mainline"}},
			},
			FollowUpQuestions:   []string{"先看 survey"},
			SuggestedQueries:    []string{"llm agents survey"},
			RecommendedPaperIDs: []string{"paper-1"},
		},
		SelectedPaperIDs: []string{"paper-1"},
	}
	if err := db.UpsertDeepStartSession(detail); err != nil {
		t.Fatalf("UpsertDeepStartSession() error = %v", err)
	}
	if err := db.SaveDeepStartMessage(&DeepStartMessage{
		SessionID: detail.Summary.ID,
		Role:      "user",
		Content:   "llm agents",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveDeepStartMessage(user) error = %v", err)
	}
	if err := db.SaveDeepStartMessage(&DeepStartMessage{
		SessionID: detail.Summary.ID,
		Role:      "assistant",
		Content:   "这是 assistant 回复",
		CreatedAt: time.Now().Add(time.Second),
	}); err != nil {
		t.Fatalf("SaveDeepStartMessage(assistant) error = %v", err)
	}
	if err := db.SaveDeepStartSearchRound(detail.Summary.ID, detail.Summary.CurrentQuery, detail.CurrentResults, detail.CurrentAnalysis); err != nil {
		t.Fatalf("SaveDeepStartSearchRound() error = %v", err)
	}
	_ = db.Close()

	reopened, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("NewDB(reopen) error = %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	sessions, err := reopened.ListDeepStartSessions()
	if err != nil {
		t.Fatalf("ListDeepStartSessions() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one session, got %d", len(sessions))
	}

	loaded, err := reopened.GetDeepStartSession(detail.Summary.ID)
	if err != nil {
		t.Fatalf("GetDeepStartSession() error = %v", err)
	}
	if loaded.CurrentAnalysis == nil || loaded.CurrentAnalysis.Overview != "测试 overview" {
		t.Fatalf("expected analysis to round-trip, got %+v", loaded.CurrentAnalysis)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("expected two deepstart messages, got %d", len(loaded.Messages))
	}
	if len(loaded.SelectedPaperIDs) != 1 || loaded.SelectedPaperIDs[0] != "paper-1" {
		t.Fatalf("expected selected IDs to persist, got %+v", loaded.SelectedPaperIDs)
	}

	var roundCount int
	if err := reopened.conn.QueryRow(`SELECT COUNT(*) FROM deepstart_search_rounds WHERE session_id = ?`, detail.Summary.ID).Scan(&roundCount); err != nil {
		t.Fatalf("counting search rounds: %v", err)
	}
	if roundCount != 1 {
		t.Fatalf("expected one search round, got %d", roundCount)
	}
}

func TestDeepStartEnrichmentCacheRoundTrip(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	entry := &DeepStartEnrichmentCache{
		CacheKey:          "test-key|2025",
		Institutions:      []string{"CMU", "OpenAI"},
		Keywords:          []string{"embodied", "agent"},
		SourceLabel:       "arXiv",
		PublicationVenue:  "NeurIPS",
		PublicationYear:   2025,
		CitationCount:     123,
		OpenAlexAttempted: true,
		CrossrefAttempted: true,
		ErrorMessage:      "",
		UpdatedAt:         time.Now(),
	}
	if err := db.UpsertDeepStartEnrichmentCache(entry); err != nil {
		t.Fatalf("UpsertDeepStartEnrichmentCache() error = %v", err)
	}

	loaded, err := db.GetDeepStartEnrichmentCache(entry.CacheKey)
	if err != nil {
		t.Fatalf("GetDeepStartEnrichmentCache() error = %v", err)
	}
	if len(loaded.Institutions) != 2 {
		t.Fatalf("expected two institutions, got %+v", loaded.Institutions)
	}
	if !loaded.OpenAlexAttempted || !loaded.CrossrefAttempted {
		t.Fatalf("expected provider attempt flags to persist, got %+v", loaded)
	}
	if loaded.SourceLabel != "arXiv" {
		t.Fatalf("expected source label to round-trip, got %q", loaded.SourceLabel)
	}
	if loaded.PublicationVenue != "NeurIPS" || loaded.PublicationYear != 2025 || loaded.CitationCount != 123 {
		t.Fatalf("expected publication metadata to round-trip, got %+v", loaded)
	}
}
