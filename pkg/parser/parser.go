// Package parser wraps goldmark to produce an AST from markdown source.
//
// The parser is intentionally thin: rules consume the AST plus the original
// source bytes and emit byte ranges to splice. The renderer never re-emits
// markdown from the AST, so the output is byte-faithful by construction
// outside of the spliced regions.
package parser

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var md = goldmark.New()

// Parse returns the goldmark AST for the given markdown source.
func Parse(content []byte) (ast.Node, error) {
	reader := text.NewReader(content)
	return md.Parser().Parse(reader), nil
}
