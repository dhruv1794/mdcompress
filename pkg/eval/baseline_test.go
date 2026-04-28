package eval

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

type baselineBackend struct{}

func (baselineBackend) Name() string {
	return "baseline"
}

func (baselineBackend) Complete(prompt string) (string, error) {
	switch {
	case strings.Contains(prompt, "Generate exactly"):
		return `{"questions":[{"id":"q1","text":"What stable fact is stated by this document?"}]}`, nil
	case strings.Contains(prompt, "Answer the question"):
		fact := stableFact(prompt)
		if fact == "" {
			return "not found", nil
		}
		return "Stable fact: " + fact, nil
	case strings.Contains(prompt, "Score 1.0"):
		if strings.Count(prompt, "Stable fact:") >= 2 {
			return `{"score":1.0,"reason":"stable fact preserved"}`, nil
		}
		return `{"score":0.0,"reason":"stable fact missing"}`, nil
	default:
		return "", fmt.Errorf("unexpected eval prompt")
	}
}

var stableFactPattern = regexp.MustCompile(`Stable fact:\s*([^\n]+)`)

func stableFact(prompt string) string {
	match := stableFactPattern.FindStringSubmatch(prompt)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func TestTier1BaselineEvalCorpus(t *testing.T) {
	report, err := Run(Options{
		Repo:            filepath.Join("..", "..", "internal", "testdata", "eval-corpus"),
		Tier:            compress.TierSafe,
		QuestionsPerDoc: 1,
		Threshold:       0.95,
		Seeds:           1,
		Backend:         baselineBackend{},
		Model:           "deterministic-baseline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 10 {
		t.Fatalf("Files = %d, want 10", len(report.Files))
	}
	if !report.Passed {
		t.Fatalf("Tier-1 baseline failed with average score %.3f", report.AverageScore)
	}
	if report.AverageScore != 1 {
		t.Fatalf("AverageScore = %.3f, want 1.000", report.AverageScore)
	}
	for _, file := range report.Files {
		if !file.Passed || file.AverageScore != 1 {
			t.Fatalf("%s baseline score = %.3f passed=%v", file.Source, file.AverageScore, file.Passed)
		}
	}
}
