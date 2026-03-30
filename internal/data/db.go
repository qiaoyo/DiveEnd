package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB is the database connection
type DB struct {
	*sql.DB
}

// Paper represents a paper in the database
type Paper struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Authors     string    `json:"authors"`
	Venue       string    `json:"venue"`
	Year        int       `json:"year"`
	Abstract    string    `json:"abstract"`
	URL         string    `json:"url"`
	PDFPath     string    `json:"pdf_path"`
	FolderID    int64     `json:"folder_id"`
	Category    string    `json:"category"`
	Tags        string    `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Folder represents a folder for organizing papers
type Folder struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// InitDB initializes the SQLite database
func InitDB(dataPath string) (*DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0700); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataPath, "diveend.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables
	if err := createTables(db); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func createTables(db *sql.DB) error {
	// Create folders table
	createFolders := `
	CREATE TABLE IF NOT EXISTS folders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Create papers table
	createPapers := `
	CREATE TABLE IF NOT EXISTS papers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		authors TEXT,
		venue TEXT,
		year INTEGER,
		abstract TEXT,
		url TEXT,
		pdf_path TEXT,
		folder_id INTEGER,
		category TEXT,
		tags TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (folder_id) REFERENCES folders(id)
	);`

	// Create paper translations table
	createTranslations := `
	CREATE TABLE IF NOT EXISTS translations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		paper_id INTEGER NOT NULL,
		section TEXT,
		original_text TEXT,
		translated_text TEXT,
		summary TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (paper_id) REFERENCES papers(id)
	);`

	if _, err := db.Exec(createFolders); err != nil {
		return err
	}
	if _, err := db.Exec(createPapers); err != nil {
		return err
	}
	if _, err := db.Exec(createTranslations); err != nil {
		return err
	}

	return nil
}

// CreateFolder inserts a new folder
func (db *DB) CreateFolder(name, description string) (int64, error) {
	res, err := db.Exec(`INSERT INTO folders (name, description) VALUES (?, ?)`, name, description)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetFolders returns all folders
func (db *DB) GetFolders() ([]Folder, error) {
	rows, err := db.Query(`SELECT id, name, description, created_at FROM folders ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.CreatedAt); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, nil
}

// InsertPaper inserts a new paper
func (db *DB) InsertPaper(p *Paper) (int64, error) {
	res, err := db.Exec(`
	INSERT INTO papers (title, authors, venue, year, abstract, url, pdf_path, folder_id, category, tags)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Title, p.Authors, p.Venue, p.Year, p.Abstract, p.URL, p.PDFPath, p.FolderID, p.Category, p.Tags,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetPapersByFolder returns papers in a folder
func (db *DB) GetPapersByFolder(folderID int64) ([]Paper, error) {
	rows, err := db.Query(`
	SELECT id, title, authors, venue, year, abstract, url, pdf_path, folder_id, category, tags, created_at, updated_at
	FROM papers WHERE folder_id = ? ORDER BY created_at DESC`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var papers []Paper
	for rows.Next() {
		var p Paper
		if err := rows.Scan(&p.ID, &p.Title, &p.Authors, &p.Venue, &p.Year, &p.Abstract, &p.URL, &p.PDFPath, &p.FolderID, &p.Category, &p.Tags, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		papers = append(papers, p)
	}
	return papers, nil
}

// GetPaper returns a single paper by ID
func (db *DB) GetPaper(id int64) (*Paper, error) {
	var p Paper
	err := db.QueryRow(`
	SELECT id, title, authors, venue, year, abstract, url, pdf_path, folder_id, category, tags, created_at, updated_at
	FROM papers WHERE id = ?`, id).Scan(&p.ID, &p.Title, &p.Authors, &p.Venue, &p.Year, &p.Abstract, &p.URL, &p.PDFPath, &p.FolderID, &p.Category, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveTranslation saves a translated section
func (db *DB) SaveTranslation(paperID int64, section, original, translated, summary string) error {
	_, err := db.Exec(`
	INSERT INTO translations (paper_id, section, original_text, translated_text, summary)
	VALUES (?, ?, ?, ?, ?)`, paperID, section, original, translated, summary)
	return err
}

// GetTranslationsForPaper gets all translations for a paper
func (db *DB) GetTranslationsForPaper(paperID int64) ([]struct {
	Section       string
	OriginalText  string
	TranslatedText string
	Summary       string
}, error) {
	rows, err := db.Query(`
	SELECT section, original_text, translated_text, summary FROM translations WHERE paper_id = ?`, paperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var translations []struct {
		Section        string
		OriginalText   string
		TranslatedText string
		Summary        string
	}

	for rows.Next() {
		var t struct {
			Section        string
			OriginalText   string
			TranslatedText string
			Summary        string
		}
		if err := rows.Scan(&t.Section, &t.OriginalText, &t.TranslatedText, &t.Summary); err != nil {
			return nil, err
		}
		translations = append(translations, t)
	}
	return translations, nil
}
