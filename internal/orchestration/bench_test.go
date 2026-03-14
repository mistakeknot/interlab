package orchestration

import (
	"fmt"
	"testing"
)

// BenchmarkValidatePlan measures plan validation for various campaign counts.
func BenchmarkValidatePlan(b *testing.B) {
	sizes := []int{3, 10, 25}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("campaigns_%d", n), func(b *testing.B) {
			input := generatePlanInput(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				validatePlan(input)
			}
		})
	}
}

// BenchmarkDetectFileConflicts measures conflict detection scaling.
func BenchmarkDetectFileConflicts(b *testing.B) {
	sizes := []int{3, 10, 25}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("campaigns_%d", n), func(b *testing.B) {
			input := generatePlanInput(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				detectFileConflicts(input.Campaigns)
			}
		})
	}
}

func generatePlanInput(n int) PlanInput {
	campaigns := make([]CampaignSpec, n)
	for i := 0; i < n; i++ {
		campaigns[i] = CampaignSpec{
			Name:             fmt.Sprintf("campaign-%d", i),
			MetricName:       fmt.Sprintf("metric_%d", i),
			MetricUnit:       "ns",
			Direction:        "lower_is_better",
			BenchmarkCommand: fmt.Sprintf("echo METRIC metric_%d=100", i),
			FilesInScope:     []string{fmt.Sprintf("file_%d.go", i)},
		}
		if i > 0 && i%3 == 0 {
			campaigns[i].DependsOn = []string{campaigns[i-1].Name}
		}
	}
	return PlanInput{
		Goal:      "benchmark test",
		Campaigns: campaigns,
	}
}
