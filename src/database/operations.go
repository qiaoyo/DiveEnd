package database

import (
	"time"

	"gorm.io/gorm"
)

// MetricOperations handles metric-related database operations
type MetricOperations struct {
	db *gorm.DB
}

// NewMetricOperations creates a new metric operations instance
func NewMetricOperations(db *DB) *MetricOperations {
	return &MetricOperations{db: db.conn}
}

// Create creates a new metric
func (o *MetricOperations) Create(metric *Metric) error {
	return o.db.Create(metric).Error
}

// CreateBatch creates multiple metrics
func (o *MetricOperations) CreateBatch(metrics []*Metric) error {
	return o.db.CreateInBatches(metrics, 100).Error
}

// GetByPaperID retrieves all metrics for a paper
func (o *MetricOperations) GetByPaperID(paperID uint) ([]Metric, error) {
	var metrics []Metric
	if err := o.db.Where("paper_id = ?", paperID).Find(&metrics).Error; err != nil {
		return nil, err
	}
	return metrics, nil
}

// Update updates a metric
func (o *MetricOperations) Update(metric *Metric) error {
	return o.db.Save(metric).Error
}

// Delete deletes a metric
func (o *MetricOperations) Delete(id uint) error {
	return o.db.Delete(&Metric{}, id).Error
}

// DeleteByPaperID deletes all metrics for a paper
func (o *MetricOperations) DeleteByPaperID(paperID uint) error {
	return o.db.Where("paper_id = ?", paperID).Delete(&Metric{}).Error
}

// BaselineOperations handles baseline-related database operations
type BaselineOperations struct {
	db *gorm.DB
}

// NewBaselineOperations creates a new baseline operations instance
func NewBaselineOperations(db *DB) *BaselineOperations {
	return &BaselineOperations{db: db.conn}
}

// Create creates a new baseline
func (o *BaselineOperations) Create(baseline *Baseline) error {
	return o.db.Create(baseline).Error
}

// CreateBatch creates multiple baselines
func (o *BaselineOperations) CreateBatch(baselines []*Baseline) error {
	return o.db.CreateInBatches(baselines, 100).Error
}

// GetByPaperID retrieves all baselines for a paper
func (o *BaselineOperations) GetByPaperID(paperID uint) ([]Baseline, error) {
	var baselines []Baseline
	if err := o.db.Where("paper_id = ?", paperID).Find(&baselines).Error; err != nil {
		return nil, err
	}
	return baselines, nil
}

// GetByPaperAndMetric retrieves baselines for a specific metric
func (o *BaselineOperations) GetByPaperAndMetric(paperID uint, metricName string) ([]Baseline, error) {
	var baselines []Baseline
	if err := o.db.Where("paper_id = ? AND metric_name = ?", paperID, metricName).Find(&baselines).Error; err != nil {
		return nil, err
	}
	return baselines, nil
}

// Update updates a baseline
func (o *BaselineOperations) Update(baseline *Baseline) error {
	return o.db.Save(baseline).Error
}

// Delete deletes a baseline
func (o *BaselineOperations) Delete(id uint) error {
	return o.db.Delete(&Baseline{}, id).Error
}

// DeleteByPaperID deletes all baselines for a paper
func (o *BaselineOperations) DeleteByPaperID(paperID uint) error {
	return o.db.Where("paper_id = ?", paperID).Delete(&Baseline{}).Error
}

// TranslationOperations handles translation-related database operations
type TranslationOperations struct {
	db *gorm.DB
}

// NewTranslationOperations creates a new translation operations instance
func NewTranslationOperations(db *DB) *TranslationOperations {
	return &TranslationOperations{db: db.conn}
}

// Create creates a new translation
func (o *TranslationOperations) Create(translation *Translation) error {
	return o.db.Create(translation).Error
}

// GetByPaperID retrieves the translation for a paper
func (o *TranslationOperations) GetByPaperID(paperID uint) (*Translation, error) {
	var translation Translation
	if err := o.db.Where("paper_id = ?", paperID).First(&translation).Error; err != nil {
		return nil, err
	}
	return &translation, nil
}

// GetByStatus retrieves translations by status
func (o *TranslationOperations) GetByStatus(status string) ([]Translation, error) {
	var translations []Translation
	if err := o.db.Where("status = ?", status).Find(&translations).Error; err != nil {
		return nil, err
	}
	return translations, nil
}

// Update updates a translation
func (o *TranslationOperations) Update(translation *Translation) error {
	return o.db.Save(translation).Error
}

// UpdateStatus updates only the status field
func (o *TranslationOperations) UpdateStatus(id uint, status string) error {
	return o.db.Model(&Translation{}).Where("id = ?", id).Update("status", status).Error
}

// Delete deletes a translation
func (o *TranslationOperations) Delete(id uint) error {
	return o.db.Delete(&Translation{}, id).Error
}

// DeleteByPaperID deletes the translation for a paper
func (o *TranslationOperations) DeleteByPaperID(paperID uint) error {
	return o.db.Where("paper_id = ?", paperID).Delete(&Translation{}).Error
}

// ScreeningSessionOperations handles screening session operations
type ScreeningSessionOperations struct {
	db *gorm.DB
}

// NewScreeningSessionOperations creates a new screening session operations instance
func NewScreeningSessionOperations(db *DB) *ScreeningSessionOperations {
	return &ScreeningSessionOperations{db: db.conn}
}

// Create creates a new screening session
func (o *ScreeningSessionOperations) Create(session *ScreeningSession) error {
	return o.db.Create(session).Error
}

// GetByID retrieves a screening session by ID
func (o *ScreeningSessionOperations) GetByID(id uint) (*ScreeningSession, error) {
	var session ScreeningSession
	if err := o.db.First(&session, id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// GetAll retrieves all screening sessions ordered by creation time
func (o *ScreeningSessionOperations) GetAll() ([]ScreeningSession, error) {
	var sessions []ScreeningSession
	if err := o.db.Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// Update updates a screening session
func (o *ScreeningSessionOperations) Update(session *ScreeningSession) error {
	return o.db.Save(session).Error
}

// MarkCompleted marks a session as completed
func (o *ScreeningSessionOperations) MarkCompleted(id uint) error {
	now := time.Now()
	return o.db.Model(&ScreeningSession{}).Where("id = ?", id).Update("completed_at", now).Error
}

// Delete deletes a screening session
func (o *ScreeningSessionOperations) Delete(id uint) error {
	return o.db.Delete(&ScreeningSession{}, id).Error
}
