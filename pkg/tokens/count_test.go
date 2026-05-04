package tokens

import (
	"testing"
)

func TestParseTokenizer(t *testing.T) {
	cases := []struct {
		input string
		want  Tokenizer
		err   bool
	}{
		{"", CL100k, false},
		{"cl100k", CL100k, false},
		{"cl100k_base", CL100k, false},
		{"o200k", O200k, false},
		{"o200k_base", O200k, false},
		{"BYTES", Bytes, false},
		{"gpt2", 0, true},
	}
	for _, c := range cases {
		got, err := ParseTokenizer(c.input)
		if c.err {
			if err == nil {
				t.Errorf("ParseTokenizer(%q) want error, got %v", c.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTokenizer(%q) error = %v", c.input, err)
		}
		if got != c.want {
			t.Errorf("ParseTokenizer(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestCountWithBytes(t *testing.T) {
	in := []byte("hello, world")
	if got := CountWith(in, Bytes); got != len(in) {
		t.Errorf("CountWith Bytes = %d, want %d", got, len(in))
	}
}

func TestCountWithCL100kAndO200kDiffer(t *testing.T) {
	in := []byte("# Project\n\nA short example with some words.\n")
	cl := CountWith(in, CL100k)
	o2 := CountWith(in, O200k)
	if cl <= 0 || o2 <= 0 {
		t.Fatalf("non-positive counts: cl=%d o2=%d", cl, o2)
	}
}

func TestSetDefault(t *testing.T) {
	original := DefaultTokenizer()
	t.Cleanup(func() { _ = SetDefault(original) })

	if err := SetDefault(O200k); err != nil {
		t.Fatal(err)
	}
	if got := DefaultTokenizer(); got != O200k {
		t.Errorf("DefaultTokenizer = %v, want O200k", got)
	}

	in := []byte("hello world")
	if Count(in) != CountWith(in, O200k) {
		t.Errorf("Count() should match CountWith(O200k) after SetDefault")
	}

	if err := SetDefault(Bytes); err != nil {
		t.Fatal(err)
	}
	if Count(in) != len(in) {
		t.Errorf("Count() with Bytes default should equal byte length")
	}
}

func TestTokenizerName(t *testing.T) {
	cases := map[Tokenizer]string{
		CL100k:       "cl100k_base",
		O200k:        "o200k_base",
		Bytes:        "bytes",
		Tokenizer(0): "unknown",
	}
	for tok, want := range cases {
		if got := tok.Name(); got != want {
			t.Errorf("%v.Name() = %q, want %q", tok, got, want)
		}
	}
}
