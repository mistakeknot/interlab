package experiment

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config is written as the first line of each segment in the JSONL.
type Config struct {
	Type             string   `json:"type"`                         // always "config"
	Name             string   `json:"name"`
	MetricName       string   `json:"metric_name"`
	MetricUnit       string   `json:"metric_unit"`
	Direction        string   `json:"direction"`                    // lower_is_better | higher_is_better
	BenchmarkCommand string   `json:"benchmark_command"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
	FilesInScope     []string `json:"files_in_scope,omitempty"`
	MaxExperiments   int      `json:"max_experiments,omitempty"`    // default 50
	MaxCrashes       int      `json:"max_crashes,omitempty"`        // default 3
	MaxNoImprovement int      `json:"max_no_improvement,omitempty"` // default 10
	RunID            string   `json:"run_id,omitempty"`
	BeadID           string   `json:"bead_id,omitempty"`
	Timestamp        string   `json:"timestamp"`
}

// Result is written after each experiment.
type Result struct {
	Type             string             `json:"type"`                        // always "result"
	Decision         string             `json:"decision"`                    // keep | discard | crash
	Description      string             `json:"description"`
	MetricValue      float64            `json:"metric_value,omitempty"`
	DurationMs       int64              `json:"duration_ms"`
	ExitCode         int                `json:"exit_code"`
	SecondaryMetrics map[string]float64 `json:"secondary_metrics,omitempty"`
	CommitHash       string             `json:"commit_hash,omitempty"`
	Timestamp        string             `json:"timestamp"`
}

// State holds the reconstructed campaign state.
type State struct {
	Config               Config
	SegmentID            int
	RunCount             int
	KeptCount            int
	DiscardedCount       int
	CrashCount           int
	ConsecutiveCrashes   int
	ConsecutiveNoImprove int
	BestMetric           float64
	BaselineMetric       float64
	HasBaseline          bool
	SecondaryMetricKeys  []string
}

func defaults(cfg *Config) {
	if cfg.MaxExperiments <= 0 {
		cfg.MaxExperiments = 50
	}
	if cfg.MaxCrashes <= 0 {
		cfg.MaxCrashes = 3
	}
	if cfg.MaxNoImprovement <= 0 {
		cfg.MaxNoImprovement = 10
	}
}

// ReconstructState reads the JSONL and rebuilds campaign state.
func ReconstructState(path string) (*State, error) {
	s := &State{}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open jsonl: %w", err)
	}
	defer f.Close()

	// Size scanner buffer to file — avoids 1MB allocation for small files.
	bufSize := 64 * 1024 // 64KB default
	if info, serr := f.Stat(); serr == nil && info.Size() < int64(bufSize) {
		bufSize = int(info.Size()) + 512 // small margin for safety
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, bufSize), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Byte-level type detection — avoids JSON parse for type discrimination.
		// We control the JSONL output so "type" is always present near the start.
		isConfig := bytes.Contains(line, []byte(`"type":"config"`))
		isResult := bytes.Contains(line, []byte(`"type":"result"`))
		if !isConfig && !isResult {
			continue
		}

		if isConfig {
			var cfg Config
			json.Unmarshal(line, &cfg)
			defaults(&cfg)
			s.Config = cfg
			s.SegmentID++
			s.RunCount = 0
			s.KeptCount = 0
			s.DiscardedCount = 0
			s.CrashCount = 0
			s.ConsecutiveCrashes = 0
			s.ConsecutiveNoImprove = 0
			s.BestMetric = 0
			s.HasBaseline = false
			s.SecondaryMetricKeys = nil
		} else {
			// Lightweight result struct — only decode fields needed for state.
			var res struct {
				Decision         string             `json:"decision"`
				MetricValue      float64            `json:"metric_value"`
				SecondaryMetrics map[string]float64 `json:"secondary_metrics"`
			}
			json.Unmarshal(line, &res)
			s.RunCount++

			switch res.Decision {
			case "keep":
				s.KeptCount++
				s.ConsecutiveCrashes = 0
				if !s.HasBaseline {
					s.BaselineMetric = res.MetricValue
					s.BestMetric = res.MetricValue
					s.HasBaseline = true
				} else if isBetter(res.MetricValue, s.BestMetric, s.Config.Direction) {
					s.BestMetric = res.MetricValue
					s.ConsecutiveNoImprove = 0
				} else {
					s.ConsecutiveNoImprove++
				}
			case "discard":
				s.DiscardedCount++
				s.ConsecutiveCrashes = 0
				s.ConsecutiveNoImprove++
			case "crash":
				s.CrashCount++
				s.ConsecutiveCrashes++
				s.ConsecutiveNoImprove++
			}

			if len(res.SecondaryMetrics) > 0 && s.SecondaryMetricKeys == nil {
				keys := make([]string, 0, len(res.SecondaryMetrics))
				for k := range res.SecondaryMetrics {
					keys = append(keys, k)
				}
				s.SecondaryMetricKeys = keys
			}
		}
	}

	return s, scanner.Err()
}

func isBetter(new, best float64, direction string) bool {
	if direction == "higher_is_better" {
		return new > best
	}
	return new < best
}

// CheckCircuitBreaker returns an error if any limit is exceeded.
func (s *State) CheckCircuitBreaker() error {
	cfg := s.Config
	defaults(&cfg)
	if s.ConsecutiveCrashes >= cfg.MaxCrashes {
		return fmt.Errorf("circuit breaker: %d consecutive crashes (limit %d)", s.ConsecutiveCrashes, cfg.MaxCrashes)
	}
	if s.RunCount >= cfg.MaxExperiments {
		return fmt.Errorf("circuit breaker: %d experiments (limit %d)", s.RunCount, cfg.MaxExperiments)
	}
	if s.HasBaseline && s.ConsecutiveNoImprove >= cfg.MaxNoImprovement {
		return fmt.Errorf("circuit breaker: %d consecutive experiments without improvement (limit %d)", s.ConsecutiveNoImprove, cfg.MaxNoImprovement)
	}
	return nil
}

// WriteConfigHeader appends a config entry to the JSONL.
func WriteConfigHeader(path string, cfg Config) error {
	cfg.Type = "config"
	cfg.Timestamp = time.Now().UTC().Format(time.RFC3339)
	defaults(&cfg)
	return appendJSONL(path, cfg)
}

// WriteResult appends a result entry to the JSONL.
func WriteResult(path string, res Result) error {
	res.Type = "result"
	res.Timestamp = time.Now().UTC().Format(time.RFC3339)
	return appendJSONL(path, res)
}

func appendJSONL(path string, v interface{}) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
