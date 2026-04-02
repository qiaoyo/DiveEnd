package database

import (
	"time"
)

// Paper represents an academic paper in the database
type Paper struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Title            string    `gorm:"not null" json:"title"`
	Authors          string    `gorm:"type:text" json:"authors"` // JSON array as string
	Affiliations     string    `gorm:"type:text" json:"affiliations"` // JSON array as string
	Abstract         string    `gorm:"type:text" json:"abstract"`
	Problem          string    `gorm:"type:text" json:"problem"`
	Method           string    `gorm:"type:text" json:"method"`
	GithubURL        string    `json:"github_url"`
	ArxivURL         string    `json:"arxiv_url"`
	PDFURL           string    `json:"pdf_url"`
	PDFPath          string    `json:"pdf_path"`
	PublishedAt      *time.Time `json:"published_at"`
	Source           string    `json:"source"` // e.g., "arxiv", "google_scholar", "manual"
	Status           string    `gorm:"default:'unread'" json:"status"` // unread, reading, read, translated
	RelevanceTags    string    `gorm:"type:text" json:"relevance_tags"` // JSON array
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relationships
	Metrics   []Metric   `json:"metrics,omitempty"`
	Baselines []Baseline `json:"baselines,omitempty"`
}

// Metric represents a performance metric for a paper
type Metric struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	PaperID       uint   `gorm:"not null;index" json:"paper_id"`
	MetricName    string `gorm:"not null" json:"metric_name"`
	DatasetOrTask string `json:"dataset_or_task"`
	OursValue     string `json:"ours_value"`
	Unit          string `json:"unit"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Baseline represents a baseline comparison for a metric
type Baseline struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	PaperID    uint   `gorm:"not null;index" json:"paper_id"`
	MetricName string `gorm:"not null" json:"metric_name"`
	MethodName string `gorm:"not null" json:"method_name"`
	Value      string `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ScreeningSession represents a paper screening session
type ScreeningSession struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	CriteriaLog    string    `gorm:"type:text" json:"criteria_log"` // JSON array of decision path
	ResultPaperIDs string    `gorm:"type:text" json:"result_paper_ids"` // JSON array
	PaperCount     int       `json:"paper_count"`
	SelectedCount  int       `json:"selected_count"`
}

// Translation represents a cached translation of a paper
type Translation struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PaperID        uint      `gorm:"uniqueIndex" json:"paper_id"`
	Status         string    `gorm:"default:'pending'" json:"status"` // pending, processing, completed, error
	FullText       string    `gorm:"type:text" json:"full_text"`
	AbstractCN     string    `gorm:"type:text" json:"abstract_cn"`
	IntroductionCN string    `gorm:"type:text" json:"introduction_cn"`
	MethodCN       string    `gorm:"type:text" json:"method_cn"`
	ResultsCN      string    `gorm:"type:text" json:"results_cn"`
	ConclusionCN   string    `gorm:"type:text" json:"conclusion_cn"`
	ErrorMessage   string    `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
