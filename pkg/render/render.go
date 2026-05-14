// Package render produces compressed markdown by applying byte-range edits
// to the original source. Rules walk the AST and report ranges; this
// package never re-emits markdown from the AST, which guarantees that any
// region not explicitly changed survives byte-for-byte.
package render

import "sort"

// Range is a half-open byte range [Start, End) in the source.
type Range struct {
	Start int
	End   int
}

// Edit replaces a half-open byte range [Start, End) with Replacement.
// An empty Replacement removes the range.
type Edit struct {
	Start       int
	End         int
	Replacement []byte
}

// Splice returns source with the given ranges removed. Ranges may be in
// any order and may overlap; they are clamped, sorted, and merged before
// removal. Passing no ranges returns source unchanged.
func Splice(source []byte, ranges []Range) []byte {
	if len(ranges) == 0 {
		return source
	}
	edits := make([]Edit, 0, len(ranges))
	for _, r := range ranges {
		edits = append(edits, Edit{Start: r.Start, End: r.End})
	}
	return ApplyEdits(source, edits)
}

// ApplyEdits returns source with the given edits applied. Edits may be in
// any order. Overlapping removals are merged; overlapping replacements keep
// the first edit and ignore later conflicting edits.
func ApplyEdits(source []byte, edits []Edit) []byte {
	if len(edits) == 0 {
		return source
	}

	clean := make([]Edit, 0, len(edits))
	for _, edit := range edits {
		if edit.Start < 0 {
			edit.Start = 0
		}
		if edit.End > len(source) {
			edit.End = len(source)
		}
		if edit.Start > edit.End {
			continue
		}
		if edit.Start == edit.End && len(edit.Replacement) == 0 {
			continue
		}
		clean = append(clean, edit)
	}
	if len(clean) == 0 {
		return source
	}

	// Sort by Start; for ties, pure inserts (Start==End) come before edits
	// that consume bytes at the same offset, so an insert at X and a
	// replacement at [X, Y) both apply with the insert text landing before
	// the replaced bytes.
	sort.SliceStable(clean, func(i, j int) bool {
		if clean[i].Start != clean[j].Start {
			return clean[i].Start < clean[j].Start
		}
		iInsert := clean[i].Start == clean[i].End
		jInsert := clean[j].Start == clean[j].End
		if iInsert != jInsert {
			return iInsert
		}
		return false
	})

	merged := clean[:1]
	for _, edit := range clean[1:] {
		last := &merged[len(merged)-1]
		// True overlap: edit consumes bytes already claimed by a prior edit.
		// Adjacent edits (edit.Start == last.End) are NOT an overlap and must
		// each apply — the previous `<=` check silently dropped the second.
		if edit.Start < last.End {
			if len(last.Replacement) == 0 && len(edit.Replacement) == 0 {
				if edit.End > last.End {
					last.End = edit.End
				}
			}
			continue
		}
		merged = append(merged, edit)
	}

	out := make([]byte, 0, len(source))
	cursor := 0
	for _, edit := range merged {
		out = append(out, source[cursor:edit.Start]...)
		out = append(out, edit.Replacement...)
		cursor = edit.End
	}
	out = append(out, source[cursor:]...)
	return out
}
