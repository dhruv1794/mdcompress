package rules

import (
	"bytes"

	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// HTMLComments removes standalone HTML comments from markdown source.
type HTMLComments struct{}

func (r *HTMLComments) Name() string { return "strip-html-comments" }
func (r *HTMLComments) Tier() Tier   { return TierSafe }

func (r *HTMLComments) Apply(doc ast.Node, ctx *Context) (ChangeSet, error) {
	var changes ChangeSet

	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.HTMLBlock:
			if commentRange, ok := htmlBlockCommentRange(n, ctx.Source); ok {
				addCommentRemoval(ctx.Source, commentRange, &changes)
			}
		case *ast.RawHTML:
			for i := 0; i < n.Segments.Len(); i++ {
				segment := n.Segments.At(i)
				if isHTMLComment(segment.Value(ctx.Source)) {
					addCommentRemoval(ctx.Source, segmentRange(segment), &changes)
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return changes, err
}

func htmlBlockCommentRange(node *ast.HTMLBlock, source []byte) (render.Range, bool) {
	lines := node.Lines()
	if lines.Len() == 0 {
		return render.Range{}, false
	}

	start := lines.At(0).Start
	end := lines.At(lines.Len() - 1).Stop
	if start < 0 || end < start || end > len(source) {
		return render.Range{}, false
	}
	if !isHTMLComment(source[start:end]) {
		return render.Range{}, false
	}
	return render.Range{Start: start, End: end}, true
}

func segmentRange(segment text.Segment) render.Range {
	return render.Range{Start: segment.Start, End: segment.Stop}
}

func isHTMLComment(content []byte) bool {
	trimmed := bytes.TrimSpace(content)
	return bytes.HasPrefix(trimmed, []byte("<!--")) && bytes.HasSuffix(trimmed, []byte("-->"))
}

func addCommentRemoval(source []byte, commentRange render.Range, changes *ChangeSet) {
	removal := expandOwnLineRange(source, commentRange)
	changes.Edits = append(changes.Edits, render.Edit{Start: removal.Start, End: removal.End})
	changes.Ranges = append(changes.Ranges, removal)
	changes.Stats.NodesAffected++
	changes.Stats.BytesSaved += removal.End - removal.Start
}

func expandOwnLineRange(source []byte, r render.Range) render.Range {
	lineStart := bytes.LastIndexByte(source[:r.Start], '\n') + 1
	lineEnd := r.End
	if nextNewline := bytes.IndexByte(source[r.End:], '\n'); nextNewline >= 0 {
		lineEnd = r.End + nextNewline + 1
	} else {
		lineEnd = len(source)
	}

	before := bytes.TrimSpace(source[lineStart:r.Start])
	afterEnd := lineEnd
	if afterEnd > r.End && source[afterEnd-1] == '\n' {
		afterEnd--
	}
	after := bytes.TrimSpace(source[r.End:afterEnd])
	if len(before) == 0 && len(after) == 0 {
		if lineStart > 0 && lineEnd < len(source) && source[lineStart-1] == '\n' && source[lineEnd] == '\n' {
			lineEnd++
		}
		return render.Range{Start: lineStart, End: lineEnd}
	}
	return r
}
