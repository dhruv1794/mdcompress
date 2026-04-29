package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyze_NoTools(t *testing.T) {
	dir := t.TempDir()
	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Found {
			t.Errorf("tool %s should not be found in empty dir", tool.Name)
		}
	}
}

func TestAnalyze_Markdownlint(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".markdownlint.json"), []byte("{}"), 0o644)

	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Name == "markdownlint" && !tool.Found {
			t.Errorf("markdownlint should be detected")
		}
	}
}

func TestAnalyze_Vale(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".vale.ini"), []byte("[*]\n"), 0o644)

	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Name == "vale" && !tool.Found {
			t.Errorf("vale should be detected via .vale.ini")
		}
	}
}

func TestAnalyze_ValeDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".vale"), 0o755)

	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Name == "vale" && !tool.Found {
			t.Errorf("vale should be detected via .vale directory")
		}
	}
}

func TestAnalyze_Prettier(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".prettierrc"), []byte("{}"), 0o644)

	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Name == "prettier" && !tool.Found {
			t.Errorf("prettier should be detected")
		}
	}
}

func TestAnalyze_RemarkLint(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".remarkrc"), []byte("{}"), 0o644)

	report, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range report.Tools {
		if tool.Name == "remark-lint" && !tool.Found {
			t.Errorf("remark-lint should be detected")
		}
	}
}

func TestGenerateConfig(t *testing.T) {
	cfg := GenerateConfig(Report{}, "safe")
	if cfg == "" {
		t.Error("config should not be empty")
	}
	if cfg != "" && !contains(cfg, "version: 1") {
		t.Error("config should contain version")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
