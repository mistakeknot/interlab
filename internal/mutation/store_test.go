package mutation

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecord(t *testing.T) {
	s := tempDB(t)

	id, isNewBest, bestQuality, err := s.Record(RecordParams{
		SessionID:     "sess-1",
		CampaignID:    "camp-1",
		TaskType:      "plugin-quality",
		Hypothesis:    "add docstrings",
		QualitySignal: 0.75,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
	if !isNewBest {
		t.Error("first record should be new best")
	}
	if bestQuality != 0.75 {
		t.Errorf("best quality = %v, want 0.75", bestQuality)
	}
}

func TestRecordIsNewBest(t *testing.T) {
	s := tempDB(t)

	s.Record(RecordParams{TaskType: "t1", Hypothesis: "first", QualitySignal: 0.5})
	_, isNewBest, _, _ := s.Record(RecordParams{TaskType: "t1", Hypothesis: "better", QualitySignal: 0.8})
	if !isNewBest {
		t.Error("0.8 > 0.5, should be new best")
	}

	_, isNewBest, _, _ = s.Record(RecordParams{TaskType: "t1", Hypothesis: "worse", QualitySignal: 0.3})
	if isNewBest {
		t.Error("0.3 < 0.8, should NOT be new best")
	}
}

func TestRecordRejectsNaN(t *testing.T) {
	s := tempDB(t)
	_, _, _, err := s.Record(RecordParams{TaskType: "t1", Hypothesis: "bad", QualitySignal: math.NaN()})
	if err == nil {
		t.Error("expected error for NaN quality_signal")
	}
}

func TestQuery(t *testing.T) {
	s := tempDB(t)

	s.Record(RecordParams{TaskType: "t1", Hypothesis: "low", QualitySignal: 0.2})
	s.Record(RecordParams{TaskType: "t1", Hypothesis: "high", QualitySignal: 0.9})
	s.Record(RecordParams{TaskType: "t2", Hypothesis: "other", QualitySignal: 0.5})

	results, err := s.Query(QueryParams{TaskType: "t1"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for t1, got %d", len(results))
	}
	if results[0].QualitySignal != 0.9 {
		t.Error("results should be sorted by quality DESC")
	}
}

func TestQueryEmpty(t *testing.T) {
	s := tempDB(t)
	results, err := s.Query(QueryParams{TaskType: "nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Error("nil slice: callers expect empty array, not null in JSON")
	}
}

func TestQueryMinQuality(t *testing.T) {
	s := tempDB(t)

	s.Record(RecordParams{TaskType: "t1", Hypothesis: "low", QualitySignal: 0.2})
	s.Record(RecordParams{TaskType: "t1", Hypothesis: "high", QualitySignal: 0.9})

	results, _ := s.Query(QueryParams{TaskType: "t1", MinQuality: 0.5})
	if len(results) != 1 {
		t.Fatalf("expected 1 result with min_quality 0.5, got %d", len(results))
	}
}

func TestQueryIsNewBestFilter(t *testing.T) {
	s := tempDB(t)

	s.Record(RecordParams{TaskType: "t1", Hypothesis: "first", QualitySignal: 0.5})
	s.Record(RecordParams{TaskType: "t1", Hypothesis: "worse", QualitySignal: 0.3})
	s.Record(RecordParams{TaskType: "t1", Hypothesis: "best", QualitySignal: 0.9})

	isNewBestOnly := true
	results, _ := s.Query(QueryParams{TaskType: "t1", IsNewBestOnly: &isNewBestOnly})
	if len(results) != 2 {
		t.Fatalf("expected 2 'new best' records, got %d", len(results))
	}
}

func TestGenealogy(t *testing.T) {
	s := tempDB(t)

	id1, _, _, _ := s.Record(RecordParams{TaskType: "t1", Hypothesis: "root", QualitySignal: 0.5, SessionID: "sess-root"})
	_, _, _, _ = s.Record(RecordParams{TaskType: "t1", Hypothesis: "child", QualitySignal: 0.7, InspiredBy: "sess-root", SessionID: "sess-child"})
	s.Record(RecordParams{TaskType: "t1", Hypothesis: "grandchild", QualitySignal: 0.9, InspiredBy: "sess-child", SessionID: "sess-gc"})

	tree, err := s.Genealogy(GenealogyParams{MutationID: id1})
	if err != nil {
		t.Fatalf("Genealogy: %v", err)
	}
	if tree.ID != id1 {
		t.Errorf("root ID = %d, want %d", tree.ID, id1)
	}
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(tree.Children))
	}
	if len(tree.Children[0].Children) != 1 {
		t.Fatalf("expected 1 grandchild, got %d", len(tree.Children[0].Children))
	}
}

func TestGenealogyNotFound(t *testing.T) {
	s := tempDB(t)
	_, err := s.Genealogy(GenealogyParams{MutationID: 99999})
	if err == nil {
		t.Error("expected error for nonexistent mutation")
	}
}

func TestAutoInitDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "mutations.db")
	s, err := NewStore(nested)
	if err != nil {
		t.Fatalf("NewStore with nested path: %v", err)
	}
	s.Close()
	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Error("DB file should exist after init")
	}
}
