package orchestration

import (
	"testing"
)

func TestValidatePlan(t *testing.T) {
	validCampaign := CampaignSpec{
		Name:             "speed",
		MetricName:       "latency",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "go test -bench .",
		FilesInScope:     []string{"main.go"},
	}

	tests := []struct {
		name    string
		input   PlanInput
		wantErr string
	}{
		{
			name:    "empty goal",
			input:   PlanInput{Goal: "", Campaigns: []CampaignSpec{validCampaign}},
			wantErr: "goal is required",
		},
		{
			name:    "no campaigns",
			input:   PlanInput{Goal: "optimize", Campaigns: nil},
			wantErr: "at least 1 campaign is required",
		},
		{
			name: "duplicate name",
			input: PlanInput{
				Goal: "optimize",
				Campaigns: []CampaignSpec{
					validCampaign,
					{
						Name:             "speed",
						MetricName:       "throughput",
						MetricUnit:       "ops/s",
						Direction:        "higher_is_better",
						BenchmarkCommand: "bench.sh",
					},
				},
			},
			wantErr: "duplicate campaign name",
		},
		{
			name: "bad direction",
			input: PlanInput{
				Goal: "optimize",
				Campaigns: []CampaignSpec{
					{
						Name:             "quality",
						MetricName:       "score",
						MetricUnit:       "points",
						Direction:        "bigger_is_better",
						BenchmarkCommand: "score.sh",
					},
				},
			},
			wantErr: "invalid direction",
		},
		{
			name: "missing benchmark_command",
			input: PlanInput{
				Goal: "optimize",
				Campaigns: []CampaignSpec{
					{
						Name:       "quality",
						MetricName: "score",
						MetricUnit: "points",
						Direction:  "lower_is_better",
					},
				},
			},
			wantErr: "benchmark_command is required",
		},
		{
			name: "unknown dependency",
			input: PlanInput{
				Goal: "optimize",
				Campaigns: []CampaignSpec{
					{
						Name:             "speed",
						MetricName:       "latency",
						MetricUnit:       "ms",
						Direction:        "lower_is_better",
						BenchmarkCommand: "bench.sh",
						DependsOn:        []string{"nonexistent"},
					},
				},
			},
			wantErr: "unknown campaign",
		},
		{
			name: "valid plan",
			input: PlanInput{
				Goal:      "optimize all the things",
				ParentID:  "BEAD-123",
				Campaigns: []CampaignSpec{validCampaign},
			},
			wantErr: "",
		},
		{
			name: "valid plan with dependency",
			input: PlanInput{
				Goal: "multi-step optimization",
				Campaigns: []CampaignSpec{
					validCampaign,
					{
						Name:             "size",
						MetricName:       "binary_size",
						MetricUnit:       "bytes",
						Direction:        "lower_is_better",
						BenchmarkCommand: "du -b bin/app",
						DependsOn:        []string{"speed"},
					},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlan(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestDetectFileConflicts(t *testing.T) {
	t.Run("independent campaigns with overlapping file", func(t *testing.T) {
		campaigns := []CampaignSpec{
			{
				Name:             "speed",
				MetricName:       "latency",
				MetricUnit:       "ms",
				Direction:        "lower_is_better",
				BenchmarkCommand: "bench.sh",
				FilesInScope:     []string{"shared.go", "speed.go"},
			},
			{
				Name:             "size",
				MetricName:       "binary_size",
				MetricUnit:       "bytes",
				Direction:        "lower_is_better",
				BenchmarkCommand: "du -b bin/app",
				FilesInScope:     []string{"shared.go", "size.go"},
			},
		}

		conflicts := detectFileConflicts(campaigns)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d: %+v", len(conflicts), conflicts)
		}
		if conflicts[0].File != "shared.go" {
			t.Errorf("expected conflict on shared.go, got %q", conflicts[0].File)
		}
		if len(conflicts[0].Campaigns) != 2 {
			t.Errorf("expected 2 campaigns in conflict, got %d", len(conflicts[0].Campaigns))
		}
	})

	t.Run("dependent campaigns with overlapping file", func(t *testing.T) {
		campaigns := []CampaignSpec{
			{
				Name:             "speed",
				MetricName:       "latency",
				MetricUnit:       "ms",
				Direction:        "lower_is_better",
				BenchmarkCommand: "bench.sh",
				FilesInScope:     []string{"shared.go", "speed.go"},
			},
			{
				Name:             "size",
				MetricName:       "binary_size",
				MetricUnit:       "bytes",
				Direction:        "lower_is_better",
				BenchmarkCommand: "du -b bin/app",
				FilesInScope:     []string{"shared.go", "size.go"},
				DependsOn:        []string{"speed"},
			},
		}

		conflicts := detectFileConflicts(campaigns)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts (campaigns are dependent), got %d: %+v", len(conflicts), conflicts)
		}
	})
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
