package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVerdictJSON(t *testing.T) {
	tests := []struct {
		raw  string
		want Verdict
	}{
		{`{"verdict":"pass","reason":"ok"}`, VerdictPass},
		{"```json\n{\"verdict\":\"degraded\",\"reason\":\"missed Node version\"}\n```", VerdictDegraded},
		{`Some preamble. {"verdict":"fail","reason":"contradicts"}`, VerdictFail},
		{`{"verdict":"PASS","reason":"casing should still parse"}`, VerdictPass},
	}
	for _, tt := range tests {
		v, _, err := parseVerdict(tt.raw)
		if err != nil {
			t.Fatalf("parseVerdict(%q) error: %v", tt.raw, err)
		}
		if v != tt.want {
			t.Fatalf("parseVerdict(%q) = %s, want %s", tt.raw, v, tt.want)
		}
	}
}

func TestParseVerdictRejectsUnknown(t *testing.T) {
	if _, _, err := parseVerdict(`{"verdict":"maybe","reason":"x"}`); err == nil {
		t.Fatal("expected error for unknown verdict")
	}
}

func TestLoadCorpusJSONL(t *testing.T) {
	body := strings.Join([]string{
		`{"id":"a","repo":"r","file":"f.md","question":"q?","expected_answer":"a"}`,
		`{"id":"b","repo":"r","file":"g.md","question":"q2?","expected_answer":"b"}`,
	}, "\n")
	tmp := filepath.Join(t.TempDir(), "corpus.jsonl")
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCorpus(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("unexpected tuples: %+v", got)
	}
}

func TestVerdictTallyPassRate(t *testing.T) {
	tally := VerdictTally{Pass: 3, Degraded: 1, Fail: 1, Total: 5}
	if got := tally.PassRate(); got != 0.6 {
		t.Fatalf("PassRate = %v, want 0.6", got)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := newCorpusCache(dir, false)
	if err := c.PutString("k", "hello"); err != nil {
		t.Fatal(err)
	}
	got, ok := c.GetString("k")
	if !ok || got != "hello" {
		t.Fatalf("GetString = (%q, %v), want (hello, true)", got, ok)
	}
	type ent struct {
		V string `json:"v"`
	}
	if err := c.PutJSON("j", ent{V: "x"}); err != nil {
		t.Fatal(err)
	}
	var out ent
	if !c.GetJSON("j", &out) || out.V != "x" {
		t.Fatalf("GetJSON returned %+v", out)
	}
}

func TestNoCacheReturnsMisses(t *testing.T) {
	c := newCorpusCache(t.TempDir(), true)
	if err := c.PutString("k", "v"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.GetString("k"); ok {
		t.Fatal("nocache should miss")
	}
}

func TestIsNotFoundAnswer(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"not found", true},
		{"Not found.", true},
		{"NOT FOUND", true},
		{"  not found  ", true},
		{`"not found"`, true},
		{"not found in the document", true},
		{"not found, the section is missing", true},
		{"Builds are handled by CI.", false},
		{"", false},
		{"the answer is not found here, but check section X", false},
	}
	for _, tt := range tests {
		got := isNotFoundAnswer(tt.in)
		if got != tt.want {
			t.Fatalf("isNotFoundAnswer(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
