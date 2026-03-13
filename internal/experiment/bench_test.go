package experiment

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkReconstructState measures state reconstruction latency at various JSONL sizes.
// This is the hot path — called on every tool invocation.
func BenchmarkReconstructState(b *testing.B) {
	sizes := []int{10, 50, 100, 500}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("entries_%d", n), func(b *testing.B) {
			dir := b.TempDir()
			jsonlPath := filepath.Join(dir, "interlab.jsonl")
			generateTestJSONL(b, jsonlPath, n)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := ReconstructState(jsonlPath)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWriteResult measures JSONL append latency.
func BenchmarkWriteResult(b *testing.B) {
	dir := b.TempDir()
	jsonlPath := filepath.Join(dir, "interlab.jsonl")

	cfg := Config{
		Name:       "bench",
		MetricName: "latency",
		Direction:  "lower_is_better",
	}
	if err := WriteConfigHeader(jsonlPath, cfg); err != nil {
		b.Fatal(err)
	}

	result := Result{
		Decision:    "keep",
		Description: "bench iteration",
		MetricValue: 42.5,
		DurationMs:  100,
		ExitCode:    0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteResult(jsonlPath, result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseMetrics measures METRIC line parsing throughput.
func BenchmarkParseMetrics(b *testing.B) {
	// Simulate benchmark output with 10 metric lines + 40 noise lines
	var output string
	for i := 0; i < 40; i++ {
		output += fmt.Sprintf("some benchmark output line %d\n", i)
		if i%4 == 0 {
			output += fmt.Sprintf("METRIC metric_%d=%.2f\n", i/4, float64(i)*1.5)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseMetrics(output)
	}
}

func generateTestJSONL(tb testing.TB, path string, numResults int) {
	tb.Helper()

	cfg := Config{
		Name:             "bench-campaign",
		MetricName:       "latency_ns",
		MetricUnit:       "ns",
		Direction:        "lower_is_better",
		BenchmarkCommand: "echo METRIC latency_ns=100",
		FilesInScope:     []string{"internal/experiment/state.go"},
	}
	if err := WriteConfigHeader(path, cfg); err != nil {
		tb.Fatal(err)
	}

	for i := 0; i < numResults; i++ {
		result := Result{
			Decision:    "keep",
			Description: fmt.Sprintf("iteration %d", i),
			MetricValue: float64(1000 - i),
			DurationMs:  int64(100 + i),
			ExitCode:    0,
			SecondaryMetrics: map[string]float64{
				"allocs":     float64(50 + i%10),
				"memory_kb":  float64(1024 + i*2),
			},
		}
		if err := WriteResult(path, result); err != nil {
			tb.Fatal(err)
		}
	}

	// Verify file exists
	info, err := os.Stat(path)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Logf("generated JSONL: %d entries, %d bytes", numResults+1, info.Size())
}
