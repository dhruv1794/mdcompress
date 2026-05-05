package rules_test

import (
	"bytes"
	"testing"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func applyCodeBlocks(t *testing.T, input []byte, tier rules.Tier) []byte {
	t.Helper()
	rule := &rules.CodeBlocks{}
	cs, err := rule.Apply(&rules.Context{Source: input, Config: &rules.Config{Tier: tier}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return render.ApplyEdits(input, cs.Edits)
}

func TestCodeBlocksStripsShellPrompts(t *testing.T) {
	in := []byte("```bash\n$ echo hi\n$ ls -la\n```\n")
	got := applyCodeBlocks(t, in, rules.TierSafe)
	want := []byte("```bash\necho hi\nls -la\n```\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCodeBlocksStripsYAMLComments(t *testing.T) {
	in := []byte("```yaml\n# comment\nkey: value\n```\n")
	got := applyCodeBlocks(t, in, rules.TierSafe)
	want := []byte("```yaml\nkey: value\n```\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCodeBlocksGoImportsAggressive(t *testing.T) {
	in := []byte("```go\npackage main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() { fmt.Println(\"hi\") }\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if bytes.Contains(got, []byte("import (")) {
		t.Fatalf("import block not stripped: %q", got)
	}
	if !bytes.Contains(got, []byte("fmt.Println")) {
		t.Fatalf("body content lost: %q", got)
	}
}

func TestCodeBlocksGoErrorBoilerplateAggressive(t *testing.T) {
	in := []byte("```go\nx, err := f()\nif err != nil {\n\treturn err\n}\nuse(x)\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if bytes.Contains(got, []byte("if err != nil")) {
		t.Fatalf("err check not stripped: %q", got)
	}
	if !bytes.Contains(got, []byte("use(x)")) {
		t.Fatalf("body content lost: %q", got)
	}
}

func TestCodeBlocksPythonImportsAggressive(t *testing.T) {
	in := []byte("```python\nimport os\nfrom sys import argv\n\nprint(argv)\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if bytes.Contains(got, []byte("import")) {
		t.Fatalf("imports not stripped: %q", got)
	}
}

func TestCodeBlocksSafeTierKeepsImports(t *testing.T) {
	in := []byte("```go\nimport \"fmt\"\nfunc main() {}\n```\n")
	got := applyCodeBlocks(t, in, rules.TierSafe)
	if !bytes.Contains(got, []byte("import \"fmt\"")) {
		t.Fatalf("safe tier should keep imports: %q", got)
	}
}

func TestCodeBlocksDedupesConsecutiveDuplicateBlocks(t *testing.T) {
	in := []byte("```js\nconsole.log(1);\n```\n\n```js\nconsole.log(1);\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if !bytes.Contains(got, []byte("duplicate")) {
		t.Fatalf("expected duplicate marker, got %q", got)
	}
}

func TestCodeBlocksNoFencesIsNoop(t *testing.T) {
	in := []byte("# Title\n\nNo fences here.\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if !bytes.Equal(got, in) {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestCodeBlocksUnknownLanguageLeftAlone(t *testing.T) {
	in := []byte("```fortran\nPROGRAM HELLO\nWRITE(*,*) 'HI'\nEND PROGRAM\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if !bytes.Equal(got, in) {
		t.Fatalf("unknown lang should be unchanged: got %q", got)
	}
}

func TestCodeBlocksRustUseAggressive(t *testing.T) {
	in := []byte("```rust\nuse std::io;\nuse std::fs;\n\nfn main() { println!(\"hi\"); }\n```\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if bytes.Contains(got, []byte("use std")) {
		t.Fatalf("use stmts not stripped: %q", got)
	}
	if !bytes.Contains(got, []byte("println!")) {
		t.Fatalf("body lost: %q", got)
	}
}

func TestCodeBlocksFenceInsideStringNotTriggered(t *testing.T) {
	in := []byte("Some text without any fences and a stray ` backtick.\n")
	got := applyCodeBlocks(t, in, rules.TierAggressive)
	if !bytes.Equal(got, in) {
		t.Fatalf("got %q want %q", got, in)
	}
}
