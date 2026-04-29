package rules

import (
	"strings"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

type Frontmatter struct{}

func (r *Frontmatter) Name() string { return "strip-frontmatter" }
func (r *Frontmatter) Tier() Tier   { return TierSafe }

func (r *Frontmatter) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	_ = doc
	source := ctx.Source
	srcStr := string(source)

	delim, ok := frontmatterDelimiter(srcStr)
	if !ok {
		return ChangeSet{}, nil
	}

	afterFirst := srcStr[len(delim):]
	closeIdx := strings.Index(afterFirst, delim)
	if closeIdx < 0 {
		return ChangeSet{}, nil
	}

	closeStart := len(delim) + closeIdx
	closeEnd := closeStart + len(delim)
	if closeEnd < len(source) && source[closeEnd] == '\n' {
		closeEnd++
	}

	var changes ChangeSet
	addRange(&changes, render.Range{Start: 0, End: closeEnd})
	return changes, nil
}

func frontmatterDelimiter(src string) (string, bool) {
	if strings.HasPrefix(src, "---\n") || strings.HasPrefix(src, "---\r\n") {
		return "---", true
	}
	if strings.HasPrefix(src, "+++\n") || strings.HasPrefix(src, "+++\r\n") {
		return "+++", true
	}
	return "", false
}
