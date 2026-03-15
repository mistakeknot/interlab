package mutation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type RecordParams struct {
	SessionID     string
	CampaignID    string
	TaskType      string
	Hypothesis    string
	QualitySignal float64
	InspiredBy    string            // optional session_id
	Metadata      map[string]string // arbitrary key-value
}

type Mutation struct {
	ID            int64             `json:"id"`
	SessionID     string            `json:"session_id"`
	CampaignID    string            `json:"campaign_id"`
	TaskType      string            `json:"task_type"`
	Hypothesis    string            `json:"hypothesis"`
	QualitySignal float64           `json:"quality_signal"`
	IsNewBest     bool              `json:"is_new_best"`
	InspiredBy    string            `json:"inspired_by,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     string            `json:"created_at"`
}

type QueryParams struct {
	TaskType          string
	CampaignID        string
	IsNewBestOnly     *bool
	MinQuality        float64
	InspiredBySession string
	Limit             int
}

type GenealogyParams struct {
	MutationID int64
	SessionID  string
	MaxDepth   int
}

type GenealogyNode struct {
	ID            int64            `json:"id"`
	Hypothesis    string           `json:"hypothesis"`
	QualitySignal float64          `json:"quality_signal"`
	IsNewBest     bool             `json:"is_new_best"`
	SessionID     string           `json:"session_id"`
	InspiredBy    string           `json:"inspired_by"`
	Children      []*GenealogyNode `json:"children,omitempty"`
}

// DDL statements executed individually for proper error reporting.
var ddl = []string{
	`CREATE TABLE IF NOT EXISTS mutations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL DEFAULT '',
		campaign_id TEXT NOT NULL DEFAULT '',
		task_type TEXT NOT NULL,
		hypothesis TEXT NOT NULL,
		quality_signal REAL NOT NULL,
		is_new_best INTEGER NOT NULL DEFAULT 0,
		inspired_by TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_mutations_task_quality ON mutations(task_type, quality_signal DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_mutations_campaign ON mutations(campaign_id)`,
	`CREATE INDEX IF NOT EXISTS idx_mutations_inspired_by ON mutations(inspired_by)`,
	`CREATE INDEX IF NOT EXISTS idx_mutations_session ON mutations(session_id)`,
}

func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Serialize writes and set busy timeout for concurrent access
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	for _, stmt := range ddl {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("init schema: %w", err)
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "interlab", "mutations.db")
}

func (s *Store) Record(p RecordParams) (id int64, isNewBest bool, bestQuality float64, err error) {
	if math.IsNaN(p.QualitySignal) || math.IsInf(p.QualitySignal, 0) {
		return 0, false, 0, fmt.Errorf("quality_signal must be a finite number, got %v", p.QualitySignal)
	}

	// Use transaction to prevent TOCTOU race on is_new_best.
	// SetMaxOpenConns(1) serializes concurrent writers.
	tx, err := s.db.Begin()
	if err != nil {
		return 0, false, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	// Check current best for this task type (inside transaction)
	var currentBest sql.NullFloat64
	err = tx.QueryRow(
		"SELECT MAX(quality_signal) FROM mutations WHERE task_type = ?",
		p.TaskType,
	).Scan(&currentBest)
	if err != nil {
		return 0, false, 0, fmt.Errorf("query current best: %w", err)
	}

	if currentBest.Valid {
		isNewBest = p.QualitySignal > currentBest.Float64
		if isNewBest {
			bestQuality = p.QualitySignal
		} else {
			bestQuality = currentBest.Float64
		}
	} else {
		isNewBest = true
		bestQuality = p.QualitySignal
	}

	metaJSON := "{}"
	if len(p.Metadata) > 0 {
		b, _ := json.Marshal(p.Metadata)
		metaJSON = string(b)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	res, err := tx.Exec(
		`INSERT INTO mutations (session_id, campaign_id, task_type, hypothesis, quality_signal, is_new_best, inspired_by, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.SessionID, p.CampaignID, p.TaskType, p.Hypothesis,
		p.QualitySignal, boolToInt(isNewBest), p.InspiredBy, metaJSON, now,
	)
	if err != nil {
		return 0, false, 0, fmt.Errorf("insert mutation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, false, 0, fmt.Errorf("commit: %w", err)
	}

	id, _ = res.LastInsertId()
	return id, isNewBest, bestQuality, nil
}

func (s *Store) Query(p QueryParams) ([]Mutation, error) {
	query := "SELECT id, session_id, campaign_id, task_type, hypothesis, quality_signal, is_new_best, inspired_by, metadata, created_at FROM mutations WHERE 1=1"
	var args []any

	if p.TaskType != "" {
		query += " AND task_type = ?"
		args = append(args, p.TaskType)
	}
	if p.CampaignID != "" {
		query += " AND campaign_id = ?"
		args = append(args, p.CampaignID)
	}
	if p.IsNewBestOnly != nil && *p.IsNewBestOnly {
		query += " AND is_new_best = 1"
	}
	if p.MinQuality > 0 {
		query += " AND quality_signal >= ?"
		args = append(args, p.MinQuality)
	}
	if p.InspiredBySession != "" {
		query += " AND inspired_by = ?"
		args = append(args, p.InspiredBySession)
	}

	query += " ORDER BY quality_signal DESC"

	limit := p.Limit
	if limit <= 0 {
		limit = 20
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mutations: %w", err)
	}
	defer rows.Close()

	results := make([]Mutation, 0) // empty slice, not nil (marshals as [] not null)
	for rows.Next() {
		var m Mutation
		var isNewBestInt int
		var metaJSON string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.CampaignID, &m.TaskType, &m.Hypothesis, &m.QualitySignal, &isNewBestInt, &m.InspiredBy, &metaJSON, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		m.IsNewBest = isNewBestInt == 1
		if metaJSON != "{}" {
			json.Unmarshal([]byte(metaJSON), &m.Metadata)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (s *Store) Genealogy(p GenealogyParams) (*GenealogyNode, error) {
	maxDepth := p.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	// Find the root mutation
	var rootID int64
	if p.MutationID > 0 {
		rootID = p.MutationID
	} else if p.SessionID != "" {
		err := s.db.QueryRow("SELECT id FROM mutations WHERE session_id = ? ORDER BY created_at DESC LIMIT 1", p.SessionID).Scan(&rootID)
		if err != nil {
			return nil, fmt.Errorf("find mutation by session: %w", err)
		}
	} else {
		return nil, fmt.Errorf("must provide mutation_id or session_id")
	}

	// Build the node
	node, err := s.loadNode(rootID)
	if err != nil {
		return nil, err
	}

	// Load descendants (mutations inspired by this session)
	if maxDepth > 0 {
		if err := s.loadDescendants(node, maxDepth-1); err != nil {
			return node, fmt.Errorf("partial tree (root loaded, descendants failed): %w", err)
		}
	}

	return node, nil
}

func (s *Store) loadNode(id int64) (*GenealogyNode, error) {
	var n GenealogyNode
	var isNewBestInt int
	err := s.db.QueryRow(
		"SELECT id, hypothesis, quality_signal, is_new_best, session_id, inspired_by FROM mutations WHERE id = ?", id,
	).Scan(&n.ID, &n.Hypothesis, &n.QualitySignal, &isNewBestInt, &n.SessionID, &n.InspiredBy)
	if err != nil {
		return nil, fmt.Errorf("load node %d: %w", id, err)
	}
	n.IsNewBest = isNewBestInt == 1
	return &n, nil
}

func (s *Store) loadDescendants(node *GenealogyNode, depth int) error {
	if depth <= 0 || node.SessionID == "" {
		return nil
	}

	// Collect child IDs first, then close rows before recursing.
	// This avoids holding a connection during recursive loadNode calls
	// which would deadlock with MaxOpenConns(1).
	rows, err := s.db.Query(
		"SELECT id FROM mutations WHERE inspired_by = ? ORDER BY created_at ASC",
		node.SessionID,
	)
	if err != nil {
		return fmt.Errorf("query descendants of %d: %w", node.ID, err)
	}

	var childIDs []int64
	for rows.Next() {
		var childID int64
		if err := rows.Scan(&childID); err != nil {
			rows.Close()
			return fmt.Errorf("scan child of %d: %w", node.ID, err)
		}
		childIDs = append(childIDs, childID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, childID := range childIDs {
		child, err := s.loadNode(childID)
		if err != nil {
			return err
		}
		if err := s.loadDescendants(child, depth-1); err != nil {
			return err
		}
		node.Children = append(node.Children, child)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
