package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read jsonl: %w", err)
	}

	for _, line := range bytes.Split(data, []byte("\n")) {
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
			s.RunCount++
			// Extract decision via byte scan to avoid JSON parse for discard/crash.
			var decision string
			var metricValue float64
			if bytes.Contains(line, []byte(`"decision":"keep"`)) {
				decision = "keep"
			} else if bytes.Contains(line, []byte(`"decision":"discard"`)) {
				decision = "discard"
			} else if bytes.Contains(line, []byte(`"decision":"crash"`)) {
				decision = "crash"
			}

			switch decision {
			case "keep":
				s.KeptCount++
				s.ConsecutiveCrashes = 0
				// Extract metric_value via byte scan — avoids json.Unmarshal.
				metricValue = extractMetricValue(line)
				if !s.HasBaseline {
					s.BaselineMetric = metricValue
					s.BestMetric = metricValue
					s.HasBaseline = true
				} else if isBetter(metricValue, s.BestMetric, s.Config.Direction) {
					s.BestMetric = metricValue
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

			// Track secondary metric keys (first result only)
			if s.SecondaryMetricKeys == nil && bytes.Contains(line, []byte(`"secondary_metrics"`)) {
				var sm struct {
					SecondaryMetrics map[string]float64 `json:"secondary_metrics"`
				}
				json.Unmarshal(line, &sm)
				if len(sm.SecondaryMetrics) > 0 {
					keys := make([]string, 0, len(sm.SecondaryMetrics))
					for k := range sm.SecondaryMetrics {
						keys = append(keys, k)
					}
					s.SecondaryMetricKeys = keys
				}
			}
		}
	}

	return s, nil
}

// extractMetricValue extracts "metric_value":N from JSON bytes without parsing.
func extractMetricValue(line []byte) float64 {
	key := []byte(`"metric_value":`)
	idx := bytes.Index(line, key)
	if idx < 0 {
		return 0
	}
	start := idx + len(key)
	// Find end of number: next comma, closing brace, or end of line
	end := start
	for end < len(line) && line[end] != ',' && line[end] != '}' {
		end++
	}
	val, _ := strconv.ParseFloat(string(line[start:end]), 64)
	return val
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
