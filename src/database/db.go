package database

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB wraps the GORM database connection
type DB struct {
	conn *gorm.DB
}

// Config database configuration
type Config struct {
	DataDir  string
	DBName   string
	LogLevel logger.LogLevel
}

// DefaultConfig returns the default database configuration
func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir:  dataDir,
		DBName:   "diveend.db",
		LogLevel: logger.Silent,
	}
}

// New creates a new database connection
func New(cfg Config) (*DB, error) {
	// Ensure data directory exists
	dbDir := filepath.Join(cfg.DataDir, "data")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, cfg.DBName)

	// Open database connection
	conn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate all models
	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// migrate runs database migrations
func migrate(conn *gorm.DB) error {
	return conn.AutoMigrate(
		&Paper{},
		&Metric{},
		&Baseline{},
		&ScreeningSession{},
		&Translation{},
	)
}

// Connection returns the underlying GORM connection
func (db *DB) Connection() *gorm.DB {
	return db.conn
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreatePaper creates a new paper record
func (db *DB) CreatePaper(paper *Paper) error {
	return db.conn.Create(paper).Error
}

// GetPaperByID retrieves a paper by ID with all relationships
func (db *DB) GetPaperByID(id uint) (*Paper, error) {
	var paper Paper
	if err := db.conn.Preload("Metrics").Preload("Baselines").First(&paper, id).Error; err != nil {
		return nil, err
	}
	return &paper, nil
}

// UpdatePaper updates a paper record
func (db *DB) UpdatePaper(paper *Paper) error {
	return db.conn.Save(paper).Error
}

// DeletePaper deletes a paper record
func (db *DB) DeletePaper(id uint) error {
	return db.conn.Delete(&Paper{}, id).Error
}

// ListPapers retrieves a list of papers with optional filtering
func (db *DB) ListPapers(offset, limit int, status string) ([]Paper, error) {
	var papers []Paper
	query := db.conn.Offset(offset).Limit(limit)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&papers).Error; err != nil {
		return nil, err
	}

	return papers, nil
}
