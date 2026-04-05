package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func NewDB(dataPath string) (*DB, error) {
	if err := os.MkdirAll(dataPath, 0700); err != nil {
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

func (db *DB) Close() error {
	if db == nil || db.conn == nil {
		return nil
	}
	return db.conn.Close()
}

func (db *DB) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS folders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS papers (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			authors TEXT,
			abstract TEXT,
			year INTEGER,
			journal TEXT,
			url TEXT,
			pdf_path TEXT,
			folder_id TEXT,
			category TEXT,
			tags TEXT,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(folder_id) REFERENCES folders(id)
		)`,
		`CREATE TABLE IF NOT EXISTS translations (
			id TEXT PRIMARY KEY,
			paper_id TEXT NOT NULL,
			section TEXT NOT NULL,
			original_text TEXT NOT NULL,
			translated_text TEXT NOT NULL,
			summary TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(paper_id) REFERENCES papers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS deepstart_sessions (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			root_prompt TEXT NOT NULL,
			current_query TEXT NOT NULL,
			target_folder_id TEXT,
			current_results_json TEXT NOT NULL DEFAULT '[]',
			current_analysis_json TEXT,
			selected_paper_ids_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(target_folder_id) REFERENCES folders(id)
		)`,
		`CREATE TABLE IF NOT EXISTS deepstart_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES deepstart_sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS deepstart_search_rounds (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			query TEXT NOT NULL,
			results_json TEXT NOT NULL DEFAULT '[]',
			analysis_json TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES deepstart_sessions(id)
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.conn.Exec(stmt); err != nil {
			return err
		}
	}

	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "url", typ: "TEXT"},
		{name: "folder_id", typ: "TEXT"},
	} {
		if err := db.ensurePaperColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_papers_folder_id ON papers(folder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_translations_paper_id ON translations(paper_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_sessions_updated_at ON deepstart_sessions(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_messages_session_id ON deepstart_messages(session_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_search_rounds_session_id ON deepstart_search_rounds(session_id, created_at ASC)`,
	} {
		if _, err := db.conn.Exec(stmt); err != nil {
			return err
		}
	}

	defaultFolder, err := db.ensureDefaultFolder()
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`UPDATE papers SET folder_id = ? WHERE folder_id IS NULL OR TRIM(folder_id) = ''`, defaultFolder.ID)
	return err
}

func (db *DB) ensurePaperColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(papers)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE papers ADD COLUMN %s %s`, columnName, columnType))
	return err
}

func (db *DB) ensureDefaultFolder() (Folder, error) {
	return db.CreateFolder(defaultFolderName)
}

func (db *DB) CreateFolder(name string) (Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Folder{}, fmt.Errorf("folder name cannot be empty")
	}

	existing, err := db.getFolderByName(name)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return Folder{}, err
	}

	folder := Folder{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.Exec(
		`INSERT INTO folders (id, name, created_at) VALUES (?, ?, ?)`,
		folder.ID,
		folder.Name,
		folder.CreatedAt,
	)
	if err != nil {
		return Folder{}, err
	}

	return folder, nil
}

func (db *DB) GetFolders() ([]Folder, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, created_at
		FROM folders
		ORDER BY CASE WHEN name = ? THEN 0 ELSE 1 END, created_at ASC
	`, defaultFolderName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []Folder
	for rows.Next() {
		var folder Folder
		if err := rows.Scan(&folder.ID, &folder.Name, &folder.CreatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}

	return folders, rows.Err()
}

func (db *DB) getFolderByName(name string) (Folder, error) {
	var folder Folder
	err := db.conn.QueryRow(
		`SELECT id, name, created_at FROM folders WHERE name = ?`,
		name,
	).Scan(&folder.ID, &folder.Name, &folder.CreatedAt)
	return folder, err
}

func (db *DB) UpsertPaper(paper *Paper) error {
	now := time.Now()
	if paper.ID == "" {
		paper.ID = uuid.NewString()
	}
	if paper.AddedAt.IsZero() {
		paper.AddedAt = now
	}
	if paper.UpdatedAt.IsZero() {
		paper.UpdatedAt = now
	}

	tagsJSON, err := json.Marshal(paper.Tags)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO papers (
			id, title, authors, abstract, year, journal, url, pdf_path, folder_id, category, tags, added_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			authors = excluded.authors,
			abstract = excluded.abstract,
			year = excluded.year,
			journal = excluded.journal,
			url = excluded.url,
			pdf_path = excluded.pdf_path,
			folder_id = excluded.folder_id,
			category = excluded.category,
			tags = excluded.tags,
			updated_at = excluded.updated_at
	`,
		paper.ID,
		paper.Title,
		paper.Authors,
		paper.Abstract,
		paper.Year,
		paper.Journal,
		paper.URL,
		paper.PDFPath,
		paper.FolderID,
		paper.Category,
		string(tagsJSON),
		paper.AddedAt,
		paper.UpdatedAt,
	)
	return err
}

func (db *DB) GetPapers(folderID string) ([]Paper, error) {
	baseQuery := `
		SELECT id, title, authors, abstract, year, journal, url, pdf_path, folder_id, category, tags, added_at, updated_at
		FROM papers
	`
	args := []interface{}{}
	if strings.TrimSpace(folderID) != "" {
		baseQuery += ` WHERE folder_id = ?`
		args = append(args, folderID)
	}
	baseQuery += ` ORDER BY added_at DESC`

	rows, err := db.conn.Query(baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		var paper Paper
		var authors sql.NullString
		var abstract sql.NullString
		var journal sql.NullString
		var urlValue sql.NullString
		var pdfPath sql.NullString
		var folderID sql.NullString
		var category sql.NullString
		var tagsRaw sql.NullString
		if err := rows.Scan(
			&paper.ID,
			&paper.Title,
			&authors,
			&abstract,
			&paper.Year,
			&journal,
			&urlValue,
			&pdfPath,
			&folderID,
			&category,
			&tagsRaw,
			&paper.AddedAt,
			&paper.UpdatedAt,
		); err != nil {
			return nil, err
		}
		paper.Authors = authors.String
		paper.Abstract = abstract.String
		paper.Journal = journal.String
		paper.URL = urlValue.String
		paper.PDFPath = pdfPath.String
		paper.FolderID = folderID.String
		paper.Category = category.String
		paper.Tags = decodeTags(tagsRaw.String)
		papers = append(papers, paper)
	}

	return papers, rows.Err()
}

func (db *DB) DeletePaper(id string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM translations WHERE paper_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM papers WHERE id = ?`, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) SaveTranslation(record *TranslationRecord) error {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}

	_, err := db.conn.Exec(`
		INSERT INTO translations (
			id, paper_id, section, original_text, translated_text, summary, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ID,
		record.PaperID,
		record.Section,
		record.OriginalText,
		record.TranslatedText,
		record.Summary,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

func (db *DB) GetTranslations(paperID string) ([]TranslationRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, paper_id, section, original_text, translated_text, summary, created_at, updated_at
		FROM translations
		WHERE paper_id = ?
		ORDER BY created_at DESC
	`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TranslationRecord
	for rows.Next() {
		var record TranslationRecord
		if err := rows.Scan(
			&record.ID,
			&record.PaperID,
			&record.Section,
			&record.OriginalText,
			&record.TranslatedText,
			&record.Summary,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func (db *DB) UpsertDeepStartSession(detail *DeepStartSessionDetail) error {
	if detail == nil {
		return fmt.Errorf("deepstart session detail cannot be nil")
	}

	now := time.Now()
	if detail.Summary.ID == "" {
		detail.Summary.ID = uuid.NewString()
	}
	if detail.Summary.CreatedAt.IsZero() {
		detail.Summary.CreatedAt = now
	}
	if detail.Summary.UpdatedAt.IsZero() {
		detail.Summary.UpdatedAt = now
	}

	resultsJSON, err := json.Marshal(detail.CurrentResults)
	if err != nil {
		return err
	}
	selectedJSON, err := json.Marshal(detail.SelectedPaperIDs)
	if err != nil {
		return err
	}

	var analysisJSON interface{}
	if detail.CurrentAnalysis != nil {
		payload, err := json.Marshal(detail.CurrentAnalysis)
		if err != nil {
			return err
		}
		analysisJSON = string(payload)
	}

	_, err = db.conn.Exec(`
		INSERT INTO deepstart_sessions (
			id, title, root_prompt, current_query, target_folder_id, current_results_json,
			current_analysis_json, selected_paper_ids_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			root_prompt = excluded.root_prompt,
			current_query = excluded.current_query,
			target_folder_id = excluded.target_folder_id,
			current_results_json = excluded.current_results_json,
			current_analysis_json = excluded.current_analysis_json,
			selected_paper_ids_json = excluded.selected_paper_ids_json,
			updated_at = excluded.updated_at
	`,
		detail.Summary.ID,
		detail.Summary.Title,
		detail.Summary.RootPrompt,
		detail.Summary.CurrentQuery,
		nullIfBlank(detail.Summary.TargetFolderID),
		string(resultsJSON),
		analysisJSON,
		string(selectedJSON),
		detail.Summary.CreatedAt,
		detail.Summary.UpdatedAt,
	)
	return err
}

func (db *DB) SaveDeepStartMessage(message *DeepStartMessage) error {
	if message == nil {
		return fmt.Errorf("deepstart message cannot be nil")
	}
	if message.ID == "" {
		message.ID = uuid.NewString()
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}

	_, err := db.conn.Exec(`
		INSERT INTO deepstart_messages (id, session_id, role, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		message.ID,
		message.SessionID,
		message.Role,
		message.Content,
		message.CreatedAt,
	)
	return err
}

func (db *DB) SaveDeepStartSearchRound(sessionID, query string, results []SearchPaper, analysis *DeepStartAnalysis) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return err
	}

	var analysisJSON interface{}
	if analysis != nil {
		payload, err := json.Marshal(analysis)
		if err != nil {
			return err
		}
		analysisJSON = string(payload)
	}

	_, err = db.conn.Exec(`
		INSERT INTO deepstart_search_rounds (id, session_id, query, results_json, analysis_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		uuid.NewString(),
		sessionID,
		query,
		string(resultsJSON),
		analysisJSON,
		time.Now(),
	)
	return err
}

func (db *DB) ListDeepStartSessions() ([]DeepStartSessionSummary, error) {
	rows, err := db.conn.Query(`
		SELECT id, title, root_prompt, current_query, target_folder_id, created_at, updated_at
		FROM deepstart_sessions
		ORDER BY updated_at DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []DeepStartSessionSummary
	for rows.Next() {
		var session DeepStartSessionSummary
		var targetFolderID sql.NullString
		if err := rows.Scan(
			&session.ID,
			&session.Title,
			&session.RootPrompt,
			&session.CurrentQuery,
			&targetFolderID,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, err
		}
		session.TargetFolderID = targetFolderID.String
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (db *DB) GetDeepStartSession(sessionID string) (*DeepStartSessionDetail, error) {
	var detail DeepStartSessionDetail
	var targetFolderID sql.NullString
	var resultsRaw string
	var analysisRaw sql.NullString
	var selectedRaw string

	err := db.conn.QueryRow(`
		SELECT id, title, root_prompt, current_query, target_folder_id, current_results_json,
		       current_analysis_json, selected_paper_ids_json, created_at, updated_at
		FROM deepstart_sessions
		WHERE id = ?
	`, sessionID).Scan(
		&detail.Summary.ID,
		&detail.Summary.Title,
		&detail.Summary.RootPrompt,
		&detail.Summary.CurrentQuery,
		&targetFolderID,
		&resultsRaw,
		&analysisRaw,
		&selectedRaw,
		&detail.Summary.CreatedAt,
		&detail.Summary.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	detail.Summary.TargetFolderID = targetFolderID.String
	detail.CurrentResults = decodeSearchPapers(resultsRaw)
	detail.CurrentAnalysis = decodeDeepStartAnalysis(analysisRaw.String)
	detail.SelectedPaperIDs = decodeStringSlice(selectedRaw)

	messages, err := db.getDeepStartMessages(sessionID)
	if err != nil {
		return nil, err
	}
	detail.Messages = messages

	return &detail, nil
}

func (db *DB) UpdateDeepStartSelections(sessionID string, selectedPaperIDs []string, targetFolderID string) error {
	selectedJSON, err := json.Marshal(selectedPaperIDs)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		UPDATE deepstart_sessions
		SET selected_paper_ids_json = ?, target_folder_id = ?, updated_at = ?
		WHERE id = ?
	`,
		string(selectedJSON),
		nullIfBlank(targetFolderID),
		time.Now(),
		sessionID,
	)
	return err
}

func (db *DB) getDeepStartMessages(sessionID string) ([]DeepStartMessage, error) {
	rows, err := db.conn.Query(`
		SELECT id, session_id, role, content, created_at
		FROM deepstart_messages
		WHERE session_id = ?
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []DeepStartMessage
	for rows.Next() {
		var message DeepStartMessage
		if err := rows.Scan(
			&message.ID,
			&message.SessionID,
			&message.Role,
			&message.Content,
			&message.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}

func decodeSearchPapers(raw string) []SearchPaper {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []SearchPaper{}
	}

	var papers []SearchPaper
	if err := json.Unmarshal([]byte(raw), &papers); err != nil {
		return []SearchPaper{}
	}
	return papers
}

func decodeDeepStartAnalysis(raw string) *DeepStartAnalysis {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var analysis DeepStartAnalysis
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		return nil
	}
	return &analysis
}

func decodeStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func nullIfBlank(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func decodeTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		return tags
	}

	parts := strings.Split(raw, ",")
	tags = make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}
