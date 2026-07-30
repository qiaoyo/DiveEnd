package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type screeningSQLExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// screening 相关的数据库迁移方法
func (db *DB) migrateScreening() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS screening_sessions (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'upload',
			total_papers INTEGER DEFAULT 0,
			current_node_json TEXT,
			selected_options_json TEXT NOT NULL DEFAULT '[]',
			path_history_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS screening_papers (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_path TEXT,
			file_size INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			title TEXT,
			authors TEXT,
			abstract TEXT,
			full_text TEXT,
			sections_json TEXT,
			selection TEXT,
			reason TEXT,
			target_folder_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES screening_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_papers_session_id ON screening_papers(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_screening_papers_status ON screening_papers(status)`,
	}

	for _, stmt := range statements {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	return nil
}

// UpsertScreeningSession 创建或更新筛选会话
func (db *DB) UpsertScreeningSession(session *ScreeningSession) error {
	now := time.Now()
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}

	_, err := db.conn.Exec(`
		INSERT INTO screening_sessions (
			id, title, status, total_papers, current_node_json,
			selected_options_json, path_history_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			total_papers = excluded.total_papers,
			current_node_json = excluded.current_node_json,
			selected_options_json = excluded.selected_options_json,
			path_history_json = excluded.path_history_json,
			updated_at = excluded.updated_at
	`,
		session.ID,
		session.Title,
		session.Status,
		session.TotalPapers,
		session.CurrentNodeJSON,
		session.SelectedOptionsJSON,
		session.PathHistoryJSON,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert screening session: %w", err)
	}

	return nil
}

// GetScreeningSession 获取筛选会话
func (db *DB) GetScreeningSession(sessionID string) (*ScreeningSession, error) {
	var session ScreeningSession

	err := db.conn.QueryRow(`
		SELECT id, title, status, total_papers, current_node_json,
		       selected_options_json, path_history_json, created_at, updated_at
		FROM screening_sessions
		WHERE id = ?
	`, sessionID).Scan(
		&session.ID,
		&session.Title,
		&session.Status,
		&session.TotalPapers,
		&session.CurrentNodeJSON,
		&session.SelectedOptionsJSON,
		&session.PathHistoryJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get screening session: %w", err)
	}

	return &session, nil
}

// ListScreeningSessions 获取所有筛选会话列表
func (db *DB) ListScreeningSessions() ([]ScreeningSession, error) {
	rows, err := db.conn.Query(`
		SELECT id, title, status, total_papers, created_at, updated_at
		FROM screening_sessions
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list screening sessions: %w", err)
	}
	defer rows.Close()

	var sessions []ScreeningSession
	for rows.Next() {
		var session ScreeningSession
		if err := rows.Scan(
			&session.ID,
			&session.Title,
			&session.Status,
			&session.TotalPapers,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan screening session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// DeleteScreeningSession 删除筛选会话（级联删除关联的 papers）
func (db *DB) DeleteScreeningSession(sessionID string) error {
	_, err := db.conn.Exec(`DELETE FROM screening_sessions WHERE id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete screening session: %w", err)
	}
	return nil
}

// UpsertScreeningPaper 创建或更新筛选论文
func (db *DB) UpsertScreeningPaper(paper *ScreeningPaper) error {
	return upsertScreeningPaper(db.conn, paper)
}

func upsertScreeningPaper(execer screeningSQLExecer, paper *ScreeningPaper) error {
	now := time.Now()
	if paper.ID == "" {
		paper.ID = uuid.NewString()
	}
	if paper.CreatedAt.IsZero() {
		paper.CreatedAt = now
	}
	if paper.UpdatedAt.IsZero() {
		paper.UpdatedAt = now
	}

	_, err := execer.Exec(`
		INSERT INTO screening_papers (
			id, session_id, file_name, file_path, file_size, status,
			title, authors, abstract, full_text, sections_json,
			selection, reason, target_folder_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			session_id = excluded.session_id,
			file_name = excluded.file_name,
			file_path = excluded.file_path,
			file_size = excluded.file_size,
			status = excluded.status,
			title = excluded.title,
			authors = excluded.authors,
			abstract = excluded.abstract,
			full_text = excluded.full_text,
			sections_json = excluded.sections_json,
			selection = excluded.selection,
			reason = excluded.reason,
			target_folder_id = excluded.target_folder_id,
			updated_at = excluded.updated_at
	`,
		paper.ID,
		paper.SessionID,
		paper.FileName,
		nullIfBlank(paper.FilePath),
		paper.FileSize,
		paper.Status,
		nullIfBlank(paper.Title),
		nullIfBlank(paper.Authors),
		nullIfBlank(paper.Abstract),
		nullIfBlank(paper.FullText),
		nullIfBlank(paper.SectionsJSON),
		nullIfBlank(paper.Selection),
		nullIfBlank(paper.Reason),
		nullIfBlank(paper.TargetFolderID),
		paper.CreatedAt,
		paper.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert screening paper: %w", err)
	}

	return nil
}

func (db *DB) ApplyScreeningDecision(
	sessionID string,
	papers []ScreeningPaper,
	currentNodeJSON string,
	selectedOptionsJSON string,
	pathHistoryJSON string,
) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin screening decision transaction: %w", err)
	}
	defer tx.Rollback()

	for index := range papers {
		if err := upsertScreeningPaper(tx, &papers[index]); err != nil {
			return err
		}
	}
	result, err := tx.Exec(`
		UPDATE screening_sessions
		SET current_node_json = ?, selected_options_json = ?, path_history_json = ?, updated_at = ?
		WHERE id = ?
	`, currentNodeJSON, selectedOptionsJSON, pathHistoryJSON, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update screening decision: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("screening session not found")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit screening decision: %w", err)
	}
	return nil
}

func (db *DB) ApplyInitialScreeningDecision(
	sessionID string,
	papers []ScreeningPaper,
	currentNodeJSON string,
	totalPapers int,
) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin initial screening transaction: %w", err)
	}
	defer tx.Rollback()

	for index := range papers {
		if err := upsertScreeningPaper(tx, &papers[index]); err != nil {
			return err
		}
	}
	result, err := tx.Exec(`
		UPDATE screening_sessions
		SET status = 'screen', total_papers = ?, current_node_json = ?,
		    selected_options_json = '[]', updated_at = ?
		WHERE id = ?
	`, totalPapers, currentNodeJSON, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to initialize screening decision: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("screening session not found")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit initial screening decision: %w", err)
	}
	return nil
}

// GetScreeningPapers 获取会话中的所有论文
func (db *DB) GetScreeningPapers(sessionID string) ([]ScreeningPaper, error) {
	rows, err := db.conn.Query(`
		SELECT id, session_id, file_name, file_path, file_size, status,
		       title, authors, abstract, full_text, sections_json,
		       selection, reason, target_folder_id, created_at, updated_at
		FROM screening_papers
		WHERE session_id = ?
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get screening papers: %w", err)
	}
	defer rows.Close()

	var papers []ScreeningPaper
	for rows.Next() {
		var paper ScreeningPaper
		var title, authors, abstract, fullText, sectionsJSON, selection, reason, targetFolderID sql.NullString
		var filePath sql.NullString

		if err := rows.Scan(
			&paper.ID,
			&paper.SessionID,
			&paper.FileName,
			&filePath,
			&paper.FileSize,
			&paper.Status,
			&title,
			&authors,
			&abstract,
			&fullText,
			&sectionsJSON,
			&selection,
			&reason,
			&targetFolderID,
			&paper.CreatedAt,
			&paper.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan screening paper: %w", err)
		}

		paper.FilePath = filePath.String
		paper.Title = title.String
		paper.Authors = authors.String
		paper.Abstract = abstract.String
		paper.FullText = fullText.String
		paper.SectionsJSON = sectionsJSON.String
		paper.Selection = selection.String
		paper.Reason = reason.String
		paper.TargetFolderID = targetFolderID.String

		papers = append(papers, paper)
	}

	return papers, rows.Err()
}

// GetScreeningPapersByStatus 按状态获取论文
func (db *DB) GetScreeningPapersByStatus(sessionID, status string) ([]ScreeningPaper, error) {
	rows, err := db.conn.Query(`
		SELECT id, session_id, file_name, file_path, file_size, status,
		       title, authors, abstract, full_text, sections_json,
		       selection, reason, target_folder_id, created_at, updated_at
		FROM screening_papers
		WHERE session_id = ? AND status = ?
		ORDER BY created_at ASC
	`, sessionID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get screening papers by status: %w", err)
	}
	defer rows.Close()

	var papers []ScreeningPaper
	for rows.Next() {
		var paper ScreeningPaper
		var title, authors, abstract, fullText, sectionsJSON, selection, reason, targetFolderID sql.NullString
		var filePath sql.NullString

		if err := rows.Scan(
			&paper.ID,
			&paper.SessionID,
			&paper.FileName,
			&filePath,
			&paper.FileSize,
			&paper.Status,
			&title,
			&authors,
			&abstract,
			&fullText,
			&sectionsJSON,
			&selection,
			&reason,
			&targetFolderID,
			&paper.CreatedAt,
			&paper.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan screening paper: %w", err)
		}

		paper.FilePath = filePath.String
		paper.Title = title.String
		paper.Authors = authors.String
		paper.Abstract = abstract.String
		paper.FullText = fullText.String
		paper.SectionsJSON = sectionsJSON.String
		paper.Selection = selection.String
		paper.Reason = reason.String
		paper.TargetFolderID = targetFolderID.String

		papers = append(papers, paper)
	}

	return papers, rows.Err()
}

// UpdateScreeningPaperStatus 更新论文状态
func (db *DB) UpdateScreeningPaperStatus(paperID, status string) error {
	_, err := db.conn.Exec(`
		UPDATE screening_papers
		SET status = ?, updated_at = ?
		WHERE id = ?
	`, status, time.Now(), paperID)

	if err != nil {
		return fmt.Errorf("failed to beupdate screening paper status: %w", err)
	}

	return nil
}

// UpdateScreeningPaperContent 更新论文提取的内容
func (db *DB) UpdateScreeningPaperContent(paperID, title, authors, abstract, fullText, sectionsJSON string) error {
	_, err := db.conn.Exec(`
		UPDATE screening_papers
		SET title = ?, authors = ?, abstract = ?, full_text = ?, sections_json = ?, updated_at = ?
		WHERE id = ?
	`, nullIfBlank(title), nullIfBlank(authors), nullIfBlank(abstract),
		nullIfBlank(fullText), nullIfBlank(sectionsJSON), time.Now(), paperID)

	if err != nil {
		return fmt.Errorf("failed to update screening paper content: %w", err)
	}

	return nil
}

// UpdateScreeningSessionNode 更新会话的当前决策树节点
func (db *DB) UpdateScreeningSessionNode(sessionID, currentNodeJSON, selectedOptionsJSON string) error {
	_, err := db.conn.Exec(`
		UPDATE screening_sessions
		SET current_node_json = ?, selected_options_json = ?, updated_at = ?
		WHERE id = ?
	`, nullIfBlank(currentNodeJSON), nullIfBlank(selectedOptionsJSON), time.Now(), sessionID)

	if err != nil {
		return fmt.Errorf("failed to update screening session node: %w", err)
	}

	return nil
}

// UpdateScreeningSessionStatus 更新会话状态和论文总数
func (db *DB) UpdateScreeningSessionStatus(sessionID, status string, totalPapers int) error {
	_, err := db.conn.Exec(`
		UPDATE screening_sessions
		SET status = ?, total_papers = ?, updated_at = ?
		WHERE id = ?
	`, status, totalPapers, time.Now(), sessionID)

	if err != nil {
		return fmt.Errorf("failed to update screening session status: %w", err)
	}

	return nil
}

// UpdateScreeningSessionPathHistory 更新路径历史
func (db *DB) UpdateScreeningSessionPathHistory(sessionID, pathHistoryJSON string) error {
	_, err := db.conn.Exec(`
		UPDATE screening_sessions
		SET path_history_json = ?, updated_at = ?
		WHERE id = ?
	`, nullIfBlank(pathHistoryJSON), time.Now(), sessionID)

	if err != nil {
		return fmt.Errorf("failed to update screening session path history: %w", err)
	}

	return nil
}

// GetScreeningSessionDetail 获取会话详情（包含论文和路径历史）
func (db *DB) GetScreeningSessionDetail(sessionID string) (*ScreeningSessionDetail, error) {
	session, err := db.GetScreeningSession(sessionID)
	if err != nil {
		return nil, err
	}

	papers, err := db.GetScreeningPapers(sessionID)
	if err != nil {
		return nil, err
	}

	detail := &ScreeningSessionDetail{
		Session: *session,
		Papers:  papers,
	}

	// 解析当前节点
	if session.CurrentNodeJSON != "" && strings.TrimSpace(session.CurrentNodeJSON) != "null" {
		var node ScreeningDecisionNode
		if err := json.Unmarshal([]byte(session.CurrentNodeJSON), &node); err == nil {
			detail.CurrentNode = &node
		}
	}

	// 解析路径历史
	if session.PathHistoryJSON != "" && strings.TrimSpace(session.PathHistoryJSON) != "null" {
		var pathHistory []PathHistoryItem
		if err := json.Unmarshal([]byte(session.PathHistoryJSON), &pathHistory); err == nil {
			detail.PathHistory = pathHistory
		}
	}

	return detail, nil
}

// BatchUpsertScreeningPapers 批量插入/更新论文
func (db *DB) BatchUpsertScreeningPapers(papers []ScreeningPaper) error {
	if len(papers) == 0 {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO screening_papers (
			id, session_id, file_name, file_path, file_size, status,
			title, authors, abstract, full_text, sections_json,
			selection, reason, target_folder_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			session_id = excluded.session_id,
			file_name = excluded.file_name,
			file_path = excluded.file_path,
			file_size = excluded.file_size,
			status = excluded.status,
			title = excluded.title,
			authors = excluded.authors,
			abstract = excluded.abstract,
			full_text = excluded.full_text,
			sections_json = excluded.sections_json,
			selection = excluded.selection,
			reason = excluded.reason,
			target_folder_id = excluded.target_folder_id,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, paper := range papers {
		id := paper.ID
		if id == "" {
			id = uuid.NewString()
		}

		createdAt := paper.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		updatedAt := paper.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}

		_, err := stmt.Exec(
			id,
			paper.SessionID,
			paper.FileName,
			nullIfBlank(paper.FilePath),
			paper.FileSize,
			paper.Status,
			nullIfBlank(paper.Title),
			nullIfBlank(paper.Authors),
			nullIfBlank(paper.Abstract),
			nullIfBlank(paper.FullText),
			nullIfBlank(paper.SectionsJSON),
			nullIfBlank(paper.Selection),
			nullIfBlank(paper.Reason),
			nullIfBlank(paper.TargetFolderID),
			createdAt,
			updatedAt,
		)

		if err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	return tx.Commit()
}
