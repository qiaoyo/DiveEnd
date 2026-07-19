package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type DeepStartSearchRoundRecord struct {
	ID        string
	SessionID string
	Query     string
	Results   []SearchPaper
	Analysis  *DeepStartAnalysis
	CreatedAt time.Time
}

func NewDB(dataPath string) (*DB, error) {
	if err := ensurePlainDirectory(dataPath); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataPath, "diveend.db")
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		conn.Close()
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
			name TEXT NOT NULL,
			parent_id TEXT,
			path TEXT,
			is_system INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(parent_id) REFERENCES folders(id)
		)`,
		`CREATE TABLE IF NOT EXISTS papers (
			id TEXT PRIMARY KEY,
			source_paper_id TEXT,
			title TEXT NOT NULL,
			authors TEXT,
			abstract TEXT,
			year INTEGER,
			journal TEXT,
			url TEXT,
			pdf_path TEXT,
			download_status TEXT,
			download_error TEXT,
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
			processing_status TEXT NOT NULL DEFAULT 'completed',
			initial_ready_count INTEGER NOT NULL DEFAULT 0,
			total_planned_count INTEGER NOT NULL DEFAULT 0,
			background_remaining INTEGER NOT NULL DEFAULT 0,
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
		`CREATE TABLE IF NOT EXISTS deepstart_enrichment_cache (
			cache_key TEXT PRIMARY KEY,
			institutions_json TEXT NOT NULL DEFAULT '[]',
			keywords_json TEXT NOT NULL DEFAULT '[]',
			source_label TEXT NOT NULL DEFAULT '',
			publication_venue TEXT NOT NULL DEFAULT '',
			publication_year INTEGER NOT NULL DEFAULT 0,
			citation_count INTEGER NOT NULL DEFAULT 0,
			openalex_attempted INTEGER NOT NULL DEFAULT 0,
			crossref_attempted INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS deepread_parse_cache (
			paper_id TEXT PRIMARY KEY,
			pdf_path TEXT,
			status TEXT NOT NULL DEFAULT 'idle',
			error_message TEXT,
			markdown TEXT,
			sections_json TEXT NOT NULL DEFAULT '[]',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(paper_id) REFERENCES papers(id)
		)`,
		`CREATE TABLE IF NOT EXISTS deepread_notes (
			id TEXT PRIMARY KEY,
			paper_id TEXT NOT NULL,
			section TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(paper_id) REFERENCES papers(id)
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
		{name: "source_paper_id", typ: "TEXT"},
		{name: "download_status", typ: "TEXT"},
		{name: "download_error", typ: "TEXT"},
	} {
		if err := db.ensurePaperColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "parent_id", typ: "TEXT"},
		{name: "path", typ: "TEXT"},
		{name: "is_system", typ: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := db.ensureFolderColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "processing_status", typ: "TEXT NOT NULL DEFAULT 'completed'"},
		{name: "initial_ready_count", typ: "INTEGER NOT NULL DEFAULT 0"},
		{name: "total_planned_count", typ: "INTEGER NOT NULL DEFAULT 0"},
		{name: "background_remaining", typ: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := db.ensureDeepStartSessionColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	for _, column := range []struct {
		name string
		typ  string
	}{
		{name: "publication_venue", typ: "TEXT NOT NULL DEFAULT ''"},
		{name: "publication_year", typ: "INTEGER NOT NULL DEFAULT 0"},
		{name: "citation_count", typ: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := db.ensureDeepStartEnrichmentCacheColumn(column.name, column.typ); err != nil {
			return err
		}
	}

	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_papers_folder_id ON papers(folder_id)`,
		`CREATE INDEX IF NOT EXISTS idx_papers_download_status ON papers(download_status)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_papers_folder_source_paper ON papers(folder_id, source_paper_id) WHERE source_paper_id IS NOT NULL AND TRIM(source_paper_id) <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_folders_path ON folders(path) WHERE path IS NOT NULL AND TRIM(path) <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_folders_parent_name ON folders(parent_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_folders_parent_id ON folders(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_translations_paper_id ON translations(paper_id)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_sessions_updated_at ON deepstart_sessions(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_messages_session_id ON deepstart_messages(session_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_search_rounds_session_id ON deepstart_search_rounds(session_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepstart_enrichment_cache_updated_at ON deepstart_enrichment_cache(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deepread_notes_paper_id ON deepread_notes(paper_id, created_at DESC)`,
	} {
		if _, err := db.conn.Exec(stmt); err != nil {
			return err
		}
	}

	if _, err := db.conn.Exec(`UPDATE folders SET path = name WHERE path IS NULL OR TRIM(path) = ''`); err != nil {
		return err
	}

	defaultFolder, err := db.ensureDefaultFolder()
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`UPDATE papers SET folder_id = ? WHERE folder_id IS NULL OR TRIM(folder_id) = ''`, defaultFolder.ID)
	if err != nil {
		return err
	}
	if _, err := db.conn.Exec(`UPDATE papers SET source_paper_id = id WHERE source_paper_id IS NULL OR TRIM(source_paper_id) = ''`); err != nil {
		return err
	}
	if _, err := db.conn.Exec(`
		UPDATE papers
		SET download_status = CASE
			WHEN pdf_path IS NOT NULL AND TRIM(pdf_path) <> '' THEN 'downloaded'
			ELSE 'queued'
		END
		WHERE download_status IS NULL OR TRIM(download_status) = ''
	`); err != nil {
		return err
	}

	// 创建 screening 相关的表
	if err := db.migrateScreening(); err != nil {
		return err
	}

	// 创建 sync baidu 相关的表
	if err := db.migrateSync(); err != nil {
		return err
	}

	return nil
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

func (db *DB) ensureFolderColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(folders)`)
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

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE folders ADD COLUMN %s %s`, columnName, columnType))
	return err
}

func (db *DB) ensureDeepStartEnrichmentCacheColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(deepstart_enrichment_cache)`)
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

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE deepstart_enrichment_cache ADD COLUMN %s %s`, columnName, columnType))
	return err
}

func (db *DB) ensureDeepStartSessionColumn(columnName, columnType string) error {
	rows, err := db.conn.Query(`PRAGMA table_info(deepstart_sessions)`)
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

	_, err = db.conn.Exec(fmt.Sprintf(`ALTER TABLE deepstart_sessions ADD COLUMN %s %s`, columnName, columnType))
	return err
}

func (db *DB) ensureDefaultFolder() (Folder, error) {
	if folder, err := db.getRootFolderByName(defaultFolderName); err == nil {
		if !folder.IsSystem || strings.TrimSpace(folder.Path) != defaultFolderName {
			if _, execErr := db.conn.Exec(
				`UPDATE folders SET is_system = 1, path = ? WHERE id = ?`,
				defaultFolderName,
				folder.ID,
			); execErr != nil {
				return Folder{}, execErr
			}
			return db.GetFolderByID(folder.ID)
		}
		return folder, nil
	} else if err != sql.ErrNoRows {
		return Folder{}, err
	}

	if legacy, err := db.getRootFolderByName("Inbox"); err == nil {
		if _, execErr := db.conn.Exec(
			`UPDATE folders SET name = ?, path = ?, is_system = 1 WHERE id = ?`,
			defaultFolderName,
			defaultFolderName,
			legacy.ID,
		); execErr == nil {
			return db.GetFolderByID(legacy.ID)
		}
	}

	if folder, err := db.getFolderByPath(defaultFolderName); err == nil {
		if !folder.IsSystem {
			if _, execErr := db.conn.Exec(`UPDATE folders SET is_system = 1 WHERE id = ?`, folder.ID); execErr != nil {
				return Folder{}, execErr
			}
			return db.GetFolderByID(folder.ID)
		}
		return folder, nil
	} else if err != sql.ErrNoRows {
		return Folder{}, err
	}

	folder, err := db.CreateFolderNode("", defaultFolderName)
	if err != nil {
		return Folder{}, err
	}
	if !folder.IsSystem {
		if _, execErr := db.conn.Exec(`UPDATE folders SET is_system = 1 WHERE id = ?`, folder.ID); execErr != nil {
			return Folder{}, execErr
		}
		return db.GetFolderByID(folder.ID)
	}
	return folder, nil
}

func (db *DB) CreateFolder(name string) (Folder, error) {
	return db.CreateFolderNode("", name)
}

func (db *DB) CreateFolderNode(parentID, name string) (Folder, error) {
	name = strings.TrimSpace(name)
	if err := validateFolderSegmentStrict(name); err != nil {
		return Folder{}, err
	}
	name = normalizeFolderSegment(name)

	parentID = strings.TrimSpace(parentID)
	parentPath := ""
	if parentID != "" {
		parentFolder, err := db.GetFolderByID(parentID)
		if err != nil {
			return Folder{}, err
		}
		parentPath = strings.TrimSpace(parentFolder.Path)
	}

	path := name
	if parentPath != "" {
		path = parentPath + "/" + name
	}
	path = normalizeFolderPath(path)

	existing, err := db.getFolderByPath(path)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return Folder{}, err
	}

	folder := Folder{
		ID:        uuid.NewString(),
		Name:      name,
		ParentID:  parentID,
		Path:      path,
		IsSystem:  false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.Exec(
		`INSERT INTO folders (id, name, parent_id, path, is_system, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		folder.ID,
		folder.Name,
		nullIfBlank(folder.ParentID),
		folder.Path,
		boolToInt(folder.IsSystem),
		folder.CreatedAt,
	)
	if err != nil {
		return Folder{}, err
	}

	return folder, nil
}

func (db *DB) GetFolders() ([]Folder, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, parent_id, path, is_system, created_at
		FROM folders
		ORDER BY is_system DESC, path ASC, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []Folder
	for rows.Next() {
		var folder Folder
		var (
			parentID sql.NullString
			path     sql.NullString
			isSystem int
		)
		if err := rows.Scan(&folder.ID, &folder.Name, &parentID, &path, &isSystem, &folder.CreatedAt); err != nil {
			return nil, err
		}
		folder.ParentID = strings.TrimSpace(parentID.String)
		folder.Path = normalizeFolderPath(path.String)
		if folder.Path == "" {
			folder.Path = normalizeFolderPath(folder.Name)
		}
		folder.IsSystem = isSystem > 0
		folders = append(folders, folder)
	}

	return folders, rows.Err()
}

func (db *DB) getFolderByName(name string) (Folder, error) {
	return db.getRootFolderByName(name)
}

func (db *DB) getRootFolderByName(name string) (Folder, error) {
	name = strings.TrimSpace(name)
	var folder Folder
	var (
		parentID sql.NullString
		path     sql.NullString
		isSystem int
	)
	err := db.conn.QueryRow(
		`SELECT id, name, parent_id, path, is_system, created_at FROM folders WHERE name = ? AND (parent_id IS NULL OR TRIM(parent_id) = '') LIMIT 1`,
		name,
	).Scan(&folder.ID, &folder.Name, &parentID, &path, &isSystem, &folder.CreatedAt)
	folder.ParentID = strings.TrimSpace(parentID.String)
	folder.Path = normalizeFolderPath(path.String)
	if folder.Path == "" {
		folder.Path = normalizeFolderPath(folder.Name)
	}
	folder.IsSystem = isSystem > 0
	return folder, err
}

func (db *DB) getFolderByPath(path string) (Folder, error) {
	path = normalizeFolderPath(path)
	var folder Folder
	var (
		parentID sql.NullString
		pathRaw  sql.NullString
		isSystem int
	)
	err := db.conn.QueryRow(
		`SELECT id, name, parent_id, path, is_system, created_at FROM folders WHERE path = ? LIMIT 1`,
		path,
	).Scan(&folder.ID, &folder.Name, &parentID, &pathRaw, &isSystem, &folder.CreatedAt)
	folder.ParentID = strings.TrimSpace(parentID.String)
	folder.Path = normalizeFolderPath(pathRaw.String)
	if folder.Path == "" {
		folder.Path = normalizeFolderPath(folder.Name)
	}
	folder.IsSystem = isSystem > 0
	return folder, err
}

func (db *DB) GetFolderByID(folderID string) (Folder, error) {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return Folder{}, sql.ErrNoRows
	}
	var folder Folder
	var (
		parentID sql.NullString
		pathRaw  sql.NullString
		isSystem int
	)
	err := db.conn.QueryRow(
		`SELECT id, name, parent_id, path, is_system, created_at FROM folders WHERE id = ? LIMIT 1`,
		folderID,
	).Scan(&folder.ID, &folder.Name, &parentID, &pathRaw, &isSystem, &folder.CreatedAt)
	if err != nil {
		return Folder{}, err
	}
	folder.ParentID = strings.TrimSpace(parentID.String)
	folder.Path = normalizeFolderPath(pathRaw.String)
	if folder.Path == "" {
		folder.Path = normalizeFolderPath(folder.Name)
	}
	folder.IsSystem = isSystem > 0
	return folder, nil
}

func (db *DB) DeleteFolder(folderID string) error {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return nil
	}
	_, err := db.conn.Exec(`DELETE FROM folders WHERE id = ?`, folderID)
	return err
}

func (db *DB) UpsertPaper(paper *Paper) error {
	now := time.Now()
	if paper.ID == "" {
		paper.ID = uuid.NewString()
	}
	if strings.TrimSpace(paper.SourcePaperID) == "" {
		paper.SourcePaperID = paper.ID
	}
	if strings.TrimSpace(paper.DownloadStatus) == "" {
		if strings.TrimSpace(paper.PDFPath) != "" {
			paper.DownloadStatus = "downloaded"
		} else {
			paper.DownloadStatus = "queued"
		}
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
			id, source_paper_id, title, authors, abstract, year, journal, url, pdf_path, download_status, download_error, folder_id, category, tags, added_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_paper_id = excluded.source_paper_id,
			title = excluded.title,
			authors = excluded.authors,
			abstract = excluded.abstract,
			year = excluded.year,
			journal = excluded.journal,
			url = excluded.url,
			pdf_path = excluded.pdf_path,
			download_status = excluded.download_status,
			download_error = excluded.download_error,
			folder_id = excluded.folder_id,
			category = excluded.category,
			tags = excluded.tags,
			updated_at = excluded.updated_at
	`,
		paper.ID,
		paper.SourcePaperID,
		paper.Title,
		paper.Authors,
		paper.Abstract,
		paper.Year,
		paper.Journal,
		paper.URL,
		paper.PDFPath,
		paper.DownloadStatus,
		nullIfBlank(paper.DownloadError),
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
		SELECT id, source_paper_id, title, authors, abstract, year, journal, url, pdf_path, download_status, download_error, folder_id, category, tags, added_at, updated_at
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
		var downloadStatus sql.NullString
		var downloadError sql.NullString
		var folderID sql.NullString
		var category sql.NullString
		var tagsRaw sql.NullString
		if err := rows.Scan(
			&paper.ID,
			&paper.SourcePaperID,
			&paper.Title,
			&authors,
			&abstract,
			&paper.Year,
			&journal,
			&urlValue,
			&pdfPath,
			&downloadStatus,
			&downloadError,
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
		paper.DownloadStatus = strings.TrimSpace(downloadStatus.String)
		if paper.DownloadStatus == "" {
			if strings.TrimSpace(paper.PDFPath) != "" {
				paper.DownloadStatus = "downloaded"
			} else {
				paper.DownloadStatus = "queued"
			}
		}
		paper.DownloadError = strings.TrimSpace(downloadError.String)
		paper.FolderID = folderID.String
		paper.Category = category.String
		paper.Tags = decodeTags(tagsRaw.String)
		papers = append(papers, paper)
	}

	return papers, rows.Err()
}

func (db *DB) GetPaperByFolderAndSource(folderID, sourcePaperID string) (*Paper, error) {
	folderID = strings.TrimSpace(folderID)
	sourcePaperID = strings.TrimSpace(sourcePaperID)
	if folderID == "" || sourcePaperID == "" {
		return nil, sql.ErrNoRows
	}

	var (
		paper          Paper
		authors        sql.NullString
		abstract       sql.NullString
		journal        sql.NullString
		urlValue       sql.NullString
		pdfPath        sql.NullString
		downloadStatus sql.NullString
		downloadError  sql.NullString
		category       sql.NullString
		tagsRaw        sql.NullString
	)
	err := db.conn.QueryRow(`
		SELECT id, source_paper_id, title, authors, abstract, year, journal, url, pdf_path, download_status, download_error, folder_id, category, tags, added_at, updated_at
		FROM papers
		WHERE folder_id = ? AND source_paper_id = ?
		LIMIT 1
	`, folderID, sourcePaperID).Scan(
		&paper.ID,
		&paper.SourcePaperID,
		&paper.Title,
		&authors,
		&abstract,
		&paper.Year,
		&journal,
		&urlValue,
		&pdfPath,
		&downloadStatus,
		&downloadError,
		&paper.FolderID,
		&category,
		&tagsRaw,
		&paper.AddedAt,
		&paper.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	paper.Authors = authors.String
	paper.Abstract = abstract.String
	paper.Journal = journal.String
	paper.URL = urlValue.String
	paper.PDFPath = pdfPath.String
	paper.DownloadStatus = strings.TrimSpace(downloadStatus.String)
	if paper.DownloadStatus == "" {
		if strings.TrimSpace(paper.PDFPath) != "" {
			paper.DownloadStatus = "downloaded"
		} else {
			paper.DownloadStatus = "queued"
		}
	}
	paper.DownloadError = strings.TrimSpace(downloadError.String)
	paper.Category = category.String
	paper.Tags = decodeTags(tagsRaw.String)

	return &paper, nil
}

func (db *DB) GetPaperByID(paperID string) (*Paper, error) {
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, sql.ErrNoRows
	}

	var (
		paper          Paper
		authors        sql.NullString
		abstract       sql.NullString
		journal        sql.NullString
		urlValue       sql.NullString
		pdfPath        sql.NullString
		downloadStatus sql.NullString
		downloadError  sql.NullString
		category       sql.NullString
		tagsRaw        sql.NullString
	)
	err := db.conn.QueryRow(`
		SELECT id, source_paper_id, title, authors, abstract, year, journal, url, pdf_path, download_status, download_error, folder_id, category, tags, added_at, updated_at
		FROM papers
		WHERE id = ?
		LIMIT 1
	`, paperID).Scan(
		&paper.ID,
		&paper.SourcePaperID,
		&paper.Title,
		&authors,
		&abstract,
		&paper.Year,
		&journal,
		&urlValue,
		&pdfPath,
		&downloadStatus,
		&downloadError,
		&paper.FolderID,
		&category,
		&tagsRaw,
		&paper.AddedAt,
		&paper.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	paper.Authors = authors.String
	paper.Abstract = abstract.String
	paper.Journal = journal.String
	paper.URL = urlValue.String
	paper.PDFPath = pdfPath.String
	paper.DownloadStatus = strings.TrimSpace(downloadStatus.String)
	if paper.DownloadStatus == "" {
		if strings.TrimSpace(paper.PDFPath) != "" {
			paper.DownloadStatus = "downloaded"
		} else {
			paper.DownloadStatus = "queued"
		}
	}
	paper.DownloadError = strings.TrimSpace(downloadError.String)
	paper.Category = category.String
	paper.Tags = decodeTags(tagsRaw.String)

	return &paper, nil
}

func (db *DB) UpdatePaperDownloadState(paperID, status, pdfPath, downloadError string) error {
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return fmt.Errorf("paper id cannot be empty")
	}

	status = strings.TrimSpace(status)
	if status == "" {
		status = "queued"
	}

	_, err := db.conn.Exec(`
		UPDATE papers
		SET download_status = ?, pdf_path = ?, download_error = ?, updated_at = ?
		WHERE id = ?
	`, status, nullIfBlank(pdfPath), nullIfBlank(downloadError), time.Now(), paperID)
	return err
}

type FolderPaperStats struct {
	Total       int
	Queued      int
	Downloading int
	Downloaded  int
	Failed      int
}

func (db *DB) GetFolderPaperStats(folderID string) (FolderPaperStats, error) {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return FolderPaperStats{}, fmt.Errorf("folder id cannot be empty")
	}

	rows, err := db.conn.Query(`
		SELECT COALESCE(download_status, ''), COUNT(*)
		FROM papers
		WHERE folder_id = ?
		GROUP BY COALESCE(download_status, '')
	`, folderID)
	if err != nil {
		return FolderPaperStats{}, err
	}
	defer rows.Close()

	stats := FolderPaperStats{}
	for rows.Next() {
		var (
			status string
			count  int
		)
		if err := rows.Scan(&status, &count); err != nil {
			return FolderPaperStats{}, err
		}
		stats.Total += count
		switch strings.TrimSpace(strings.ToLower(status)) {
		case "queued":
			stats.Queued += count
		case "downloading":
			stats.Downloading += count
		case "downloaded":
			stats.Downloaded += count
		case "failed":
			stats.Failed += count
		default:
			stats.Queued += count
		}
	}

	return stats, rows.Err()
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
	if _, err := tx.Exec(`DELETE FROM deepread_notes WHERE paper_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM deepread_parse_cache WHERE paper_id = ?`, id); err != nil {
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

func (db *DB) GetDeepReadParseCache(paperID string) (*DeepReadParseCache, error) {
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return nil, sql.ErrNoRows
	}

	var (
		cache       DeepReadParseCache
		pdfPath     sql.NullString
		errorRaw    sql.NullString
		markdownRaw sql.NullString
		sectionsRaw string
	)
	err := db.conn.QueryRow(`
		SELECT paper_id, pdf_path, status, error_message, markdown, sections_json, updated_at
		FROM deepread_parse_cache
		WHERE paper_id = ?
		LIMIT 1
	`, paperID).Scan(
		&cache.PaperID,
		&pdfPath,
		&cache.Status,
		&errorRaw,
		&markdownRaw,
		&sectionsRaw,
		&cache.LastPreparedAt,
	)
	if err != nil {
		return nil, err
	}

	cache.PDFPath = strings.TrimSpace(pdfPath.String)
	cache.ErrorMessage = strings.TrimSpace(errorRaw.String)
	cache.Markdown = markdownRaw.String
	cache.Sections = decodeDeepReadSections(sectionsRaw)
	return &cache, nil
}

func (db *DB) UpsertDeepReadParseCache(cache *DeepReadParseCache) error {
	if cache == nil {
		return fmt.Errorf("deepread parse cache cannot be nil")
	}
	cache.PaperID = strings.TrimSpace(cache.PaperID)
	if cache.PaperID == "" {
		return fmt.Errorf("paper id cannot be empty")
	}
	cache.Status = strings.TrimSpace(cache.Status)
	if cache.Status == "" {
		cache.Status = "idle"
	}
	if cache.LastPreparedAt.IsZero() {
		cache.LastPreparedAt = time.Now()
	}

	sectionsJSON, err := json.Marshal(cache.Sections)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		INSERT INTO deepread_parse_cache (
			paper_id, pdf_path, status, error_message, markdown, sections_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(paper_id) DO UPDATE SET
			pdf_path = excluded.pdf_path,
			status = excluded.status,
			error_message = excluded.error_message,
			markdown = excluded.markdown,
			sections_json = excluded.sections_json,
			updated_at = excluded.updated_at
	`,
		cache.PaperID,
		nullIfBlank(cache.PDFPath),
		cache.Status,
		nullIfBlank(cache.ErrorMessage),
		nullIfBlank(cache.Markdown),
		string(sectionsJSON),
		cache.LastPreparedAt,
	)
	return err
}

func (db *DB) SaveDeepReadNote(note *DeepReadNote) error {
	if note == nil {
		return fmt.Errorf("deepread note cannot be nil")
	}
	note.PaperID = strings.TrimSpace(note.PaperID)
	note.Section = strings.TrimSpace(note.Section)
	note.Content = strings.TrimSpace(note.Content)
	if note.PaperID == "" {
		return fmt.Errorf("paper id cannot be empty")
	}
	if note.Section == "" {
		note.Section = "General"
	}
	if note.Content == "" {
		return fmt.Errorf("note content cannot be empty")
	}
	if note.ID == "" {
		note.ID = uuid.NewString()
	}
	now := time.Now()
	if note.CreatedAt.IsZero() {
		note.CreatedAt = now
	}
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = note.CreatedAt
	}

	_, err := db.conn.Exec(`
		INSERT INTO deepread_notes (
			id, paper_id, section, content, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		note.ID,
		note.PaperID,
		note.Section,
		note.Content,
		note.CreatedAt,
		note.UpdatedAt,
	)
	return err
}

func (db *DB) GetDeepReadNotes(paperID string) ([]DeepReadNote, error) {
	paperID = strings.TrimSpace(paperID)
	if paperID == "" {
		return []DeepReadNote{}, nil
	}

	rows, err := db.conn.Query(`
		SELECT id, paper_id, section, content, created_at, updated_at
		FROM deepread_notes
		WHERE paper_id = ?
		ORDER BY created_at DESC
	`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := make([]DeepReadNote, 0, 8)
	for rows.Next() {
		var note DeepReadNote
		if err := rows.Scan(
			&note.ID,
			&note.PaperID,
			&note.Section,
			&note.Content,
			&note.CreatedAt,
			&note.UpdatedAt,
		); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
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
	if detail.Summary.InitialReadyCount < 0 {
		detail.Summary.InitialReadyCount = 0
	}
	if detail.Summary.TotalPlannedCount < detail.Summary.InitialReadyCount {
		detail.Summary.TotalPlannedCount = detail.Summary.InitialReadyCount
	}
	if detail.Summary.BackgroundRemaining < 0 {
		detail.Summary.BackgroundRemaining = 0
	}
	status := strings.TrimSpace(detail.Summary.ProcessingStatus)
	if status == "" {
		if detail.Summary.BackgroundRemaining > 0 {
			status = "background_processing"
		} else {
			status = "completed"
		}
	}
	detail.Summary.ProcessingStatus = status

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
			current_analysis_json, selected_paper_ids_json, processing_status, initial_ready_count,
			total_planned_count, background_remaining, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			root_prompt = excluded.root_prompt,
			current_query = excluded.current_query,
			target_folder_id = excluded.target_folder_id,
			current_results_json = excluded.current_results_json,
			current_analysis_json = excluded.current_analysis_json,
			selected_paper_ids_json = excluded.selected_paper_ids_json,
			processing_status = excluded.processing_status,
			initial_ready_count = excluded.initial_ready_count,
			total_planned_count = excluded.total_planned_count,
			background_remaining = excluded.background_remaining,
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
		detail.Summary.ProcessingStatus,
		detail.Summary.InitialReadyCount,
		detail.Summary.TotalPlannedCount,
		detail.Summary.BackgroundRemaining,
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

func (db *DB) ListDeepStartSearchRounds(sessionID string, limit int) ([]DeepStartSearchRoundRecord, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return []DeepStartSearchRoundRecord{}, nil
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(`
		SELECT id, session_id, query, results_json, analysis_json, created_at
		FROM deepstart_search_rounds
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rounds := make([]DeepStartSearchRoundRecord, 0, limit)
	for rows.Next() {
		var (
			record      DeepStartSearchRoundRecord
			resultsJSON string
			analysisRaw sql.NullString
		)
		if err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.Query,
			&resultsJSON,
			&analysisRaw,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		record.Results = decodeSearchPapers(resultsJSON)
		record.Analysis = decodeDeepStartAnalysis(analysisRaw.String)
		rounds = append(rounds, record)
	}

	return rounds, rows.Err()
}

func (db *DB) DeleteDeepStartSearchRound(roundID string) error {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" {
		return nil
	}
	_, err := db.conn.Exec(`DELETE FROM deepstart_search_rounds WHERE id = ?`, roundID)
	return err
}

func (db *DB) GetDeepStartEnrichmentCache(cacheKey string) (*DeepStartEnrichmentCache, error) {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return nil, sql.ErrNoRows
	}

	var (
		entry            DeepStartEnrichmentCache
		institutionsJSON string
		keywordsJSON     string
		errorMessage     sql.NullString
		openAlexAttempt  int
		crossrefAttempt  int
	)
	err := db.conn.QueryRow(`
		SELECT cache_key, institutions_json, keywords_json, source_label,
		       publication_venue, publication_year, citation_count,
		       openalex_attempted, crossref_attempted, error_message, updated_at
		FROM deepstart_enrichment_cache
		WHERE cache_key = ?
	`, cacheKey).Scan(
		&entry.CacheKey,
		&institutionsJSON,
		&keywordsJSON,
		&entry.SourceLabel,
		&entry.PublicationVenue,
		&entry.PublicationYear,
		&entry.CitationCount,
		&openAlexAttempt,
		&crossrefAttempt,
		&errorMessage,
		&entry.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	entry.Institutions = decodeStringSlice(institutionsJSON)
	entry.Keywords = decodeStringSlice(keywordsJSON)
	entry.OpenAlexAttempted = openAlexAttempt > 0
	entry.CrossrefAttempted = crossrefAttempt > 0
	entry.ErrorMessage = strings.TrimSpace(errorMessage.String)
	return &entry, nil
}

func (db *DB) UpsertDeepStartEnrichmentCache(entry *DeepStartEnrichmentCache) error {
	if entry == nil {
		return fmt.Errorf("deepstart enrichment cache cannot be nil")
	}
	cacheKey := strings.TrimSpace(entry.CacheKey)
	if cacheKey == "" {
		return fmt.Errorf("deepstart enrichment cache key cannot be empty")
	}

	institutionsJSON, err := json.Marshal(entry.Institutions)
	if err != nil {
		return err
	}
	keywordsJSON, err := json.Marshal(entry.Keywords)
	if err != nil {
		return err
	}

	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now()
	}

	_, err = db.conn.Exec(`
		INSERT INTO deepstart_enrichment_cache (
			cache_key, institutions_json, keywords_json, source_label,
			publication_venue, publication_year, citation_count,
			openalex_attempted, crossref_attempted, error_message, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			institutions_json = excluded.institutions_json,
			keywords_json = excluded.keywords_json,
			source_label = excluded.source_label,
			publication_venue = excluded.publication_venue,
			publication_year = excluded.publication_year,
			citation_count = excluded.citation_count,
			openalex_attempted = excluded.openalex_attempted,
			crossref_attempted = excluded.crossref_attempted,
			error_message = excluded.error_message,
			updated_at = excluded.updated_at
	`,
		cacheKey,
		string(institutionsJSON),
		string(keywordsJSON),
		strings.TrimSpace(entry.SourceLabel),
		strings.TrimSpace(entry.PublicationVenue),
		entry.PublicationYear,
		entry.CitationCount,
		boolToInt(entry.OpenAlexAttempted),
		boolToInt(entry.CrossrefAttempted),
		nullIfBlank(entry.ErrorMessage),
		entry.UpdatedAt,
	)
	return err
}

func (db *DB) ListDeepStartSessions() ([]DeepStartSessionSummary, error) {
	rows, err := db.conn.Query(`
		SELECT id, title, root_prompt, current_query, target_folder_id,
		       processing_status, initial_ready_count, total_planned_count, background_remaining,
		       created_at, updated_at
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
			&session.ProcessingStatus,
			&session.InitialReadyCount,
			&session.TotalPlannedCount,
			&session.BackgroundRemaining,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, err
		}
		session.TargetFolderID = targetFolderID.String
		if strings.TrimSpace(session.ProcessingStatus) == "" {
			session.ProcessingStatus = "completed"
		}
		if session.BackgroundRemaining < 0 {
			session.BackgroundRemaining = 0
		}
		if session.TotalPlannedCount < session.InitialReadyCount {
			session.TotalPlannedCount = session.InitialReadyCount
		}
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
		       current_analysis_json, selected_paper_ids_json, processing_status, initial_ready_count,
		       total_planned_count, background_remaining, created_at, updated_at
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
		&detail.Summary.ProcessingStatus,
		&detail.Summary.InitialReadyCount,
		&detail.Summary.TotalPlannedCount,
		&detail.Summary.BackgroundRemaining,
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
	if detail.Summary.InitialReadyCount <= 0 {
		detail.Summary.InitialReadyCount = len(detail.CurrentResults)
	}
	if detail.Summary.BackgroundRemaining < 0 {
		detail.Summary.BackgroundRemaining = 0
	}
	if detail.Summary.TotalPlannedCount <= 0 {
		detail.Summary.TotalPlannedCount = detail.Summary.InitialReadyCount + detail.Summary.BackgroundRemaining
	}
	if detail.Summary.TotalPlannedCount < detail.Summary.InitialReadyCount {
		detail.Summary.TotalPlannedCount = detail.Summary.InitialReadyCount
	}
	if strings.TrimSpace(detail.Summary.ProcessingStatus) == "" {
		if detail.Summary.BackgroundRemaining > 0 {
			detail.Summary.ProcessingStatus = "background_processing"
		} else {
			detail.Summary.ProcessingStatus = "completed"
		}
	}

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

func decodeDeepReadSections(raw string) []DeepReadSection {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []DeepReadSection{}
	}

	var sections []DeepReadSection
	if err := json.Unmarshal([]byte(raw), &sections); err != nil {
		return []DeepReadSection{}
	}
	return sections
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

// nullIfTime 将 time.Time 转换为 sql.NullTime
func nullIfTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// nullIfTimePtr 将 *time.Time 转换为 sql.NullTime
func nullIfTimePtr(t *time.Time) sql.NullTime {
	if t == nil || t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
