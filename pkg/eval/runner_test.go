package eval

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

type fakeBackend struct{}

func (fakeBackend) Name() string {
	return "fake"
}

func (fakeBackend) Complete(prompt string) (string, error) {
	switch {
	case strings.Contains(prompt, "Generate exactly"):
		return `{"questions":[{"id":"q1","text":"What command installs dependencies?"}]}`, nil
	case strings.Contains(prompt, "Answer the question"):
		if strings.Contains(prompt, "npm install") {
			return "Run npm install.", nil
		}
		return "not found", nil
	case strings.Contains(prompt, "Score 1.0"):
		if strings.Count(prompt, "Run npm install.") >= 2 {
			return `{"score":1.0,"reason":"same command"}`, nil
		}
		return `{"score":0.0,"reason":"missing command"}`, nil
	default:
		return "", nil
	}
}

type failingBackend struct{}

func (failingBackend) Name() string {
	return "failing"
}

func (failingBackend) Complete(prompt string) (string, error) {
	return "", os.ErrInvalid
}

func TestRunPassesNoOpCompressionWithoutCallingBackend(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Project\n\nRun `npm install`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{
		Repo:            dir,
		Tier:            compress.TierSafe,
		QuestionsPerDoc: 3,
		Threshold:       0.95,
		Backend:         failingBackend{},
		Model:           "unused",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.AverageScore != 1 {
		t.Fatalf("report score = %.3f passed=%v, want 1/pass", report.AverageScore, report.Passed)
	}
	if len(report.Files) != 1 || !report.Files[0].Passed || report.Files[0].AverageScore != 1 {
		t.Fatalf("file report = %#v, want one passing no-op file", report.Files)
	}
	if len(report.Files[0].Questions) != 0 {
		t.Fatalf("Questions = %d, want 0 for no-op fast path", len(report.Files[0].Questions))
	}
}

func TestRunProducesPassingReport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Project\n\n<!-- hidden -->\n\nRun `npm install`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".mdcompress", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mdcompress", "cache", "generated.md"), []byte("Run `rm -rf`.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{
		Repo:            dir,
		Tier:            compress.TierSafe,
		QuestionsPerDoc: 1,
		Threshold:       0.95,
		Backend:         fakeBackend{},
		Model:           "fake-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("report did not pass: %#v", report)
	}
	if report.AverageScore != 1 {
		t.Fatalf("AverageScore = %v, want 1", report.AverageScore)
	}
	if len(report.Files) != 1 {
		t.Fatalf("Files = %d, want 1", len(report.Files))
	}
	if report.Files[0].Source != "README.md" {
		t.Fatalf("Source = %q, want README.md", report.Files[0].Source)
	}
	if report.Files[0].TokensSaved <= 0 {
		t.Fatalf("TokensSaved = %d, want positive", report.Files[0].TokensSaved)
	}
}

func TestRunSingleRuleRejectsUnknownRule(t *testing.T) {
	_, err := Run(Options{
		Repo:    t.TempDir(),
		Rule:    "missing-rule",
		Backend: fakeBackend{},
	})
	if err == nil {
		t.Fatalf("expected unknown rule error")
	}
}

func TestWriteReports(t *testing.T) {
	report := Report{
		Repo:            ".",
		Backend:         "fake",
		Model:           "fake-model",
		Tier:            "safe",
		Threshold:       0.95,
		QuestionsPerDoc: 1,
		AverageScore:    1,
		Passed:          true,
		Files: []FileResult{{
			Source:       "README.md",
			TokensBefore: 10,
			TokensAfter:  8,
			AverageScore: 1,
			Passed:       true,
		}},
	}

	var jsonBuf bytes.Buffer
	if err := WriteJSON(&jsonBuf, report); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Repo != "." {
		t.Fatalf("decoded Repo = %q, want .", decoded.Repo)
	}

	var md bytes.Buffer
	if err := WriteMarkdown(&md, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# mdcompress Faithfulness Eval", "Status: PASS", "`README.md`"} {
		if !strings.Contains(md.String(), want) {
			t.Fatalf("markdown missing %q:\n%s", want, md.String())
		}
	}
}
