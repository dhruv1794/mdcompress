// Package render produces compressed markdown by splicing byte ranges out
// of the original source. Rules walk the AST and report ranges; this
// package never re-emits markdown from the AST, which guarantees that any
// region not explicitly removed survives byte-for-byte.
package render

import "sort"

// Range is a half-open byte range [Start, End) in the source.
type Range struct {
	Start int
	End   int
}

// Splice returns source with the given ranges removed. Ranges may be in
// any order and may overlap; they are clamped, sorted, and merged before
// removal. Passing no ranges returns source unchanged.
func Splice(source []byte, ranges []Range) []byte {
	if len(ranges) == 0 {
		return source
	}

	clean := make([]Range, 0, len(ranges))
	for _, r := range ranges {
		if r.Start < 0 {
			r.Start = 0
		}
		if r.End > len(source) {
			r.End = len(source)
		}
		if r.Start >= r.End {
			continue
		}
		clean = append(clean, r)
	}
	if len(clean) == 0 {
		return source
	}

	sort.Slice(clean, func(i, j int) bool { return clean[i].Start < clean[j].Start })

	merged := clean[:1]
	for _, r := range clean[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		merged = append(merged, r)
	}

	out := make([]byte, 0, len(source))
	cursor := 0
	for _, r := range merged {
		out = append(out, source[cursor:r.Start]...)
		cursor = r.End
	}
	out = append(out, source[cursor:]...)
	return out
}
