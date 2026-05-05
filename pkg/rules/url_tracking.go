package rules

import (
	"regexp"
)

// URLTracking strips known marketing/tracking query parameters from URLs that
// appear inside markdown links and bare URLs. The parameters carry no
// information an LLM can use — they only identify the click — so removing
// them is lossless for any reader.
//
// We only strip parameters from a fixed allow-list of known trackers; other
// query strings (which may carry semantic identity, e.g. ?id=42) are kept.
type URLTracking struct{}

func (r *URLTracking) Name() string { return "strip-url-tracking-params" }
func (r *URLTracking) Tier() Tier   { return TierSafe }

// Match a URL inside a markdown link "(...)", autolink "<...>", or bare
// "https://...". The capture is the URL itself.
var urlMatchRe = regexp.MustCompile(`https?://[^\s)>\]]+`)

// Tracking parameter keys. Only matched as full ?key= or &key= prefixes; we
// keep this list small and conservative — better to under-strip than to
// remove a parameter that turns out to be semantic.
var trackingParamRe = regexp.MustCompile(
	`([?&])(utm_[a-z_]+|fbclid|gclid|mc_eid|mc_cid|msclkid|ref|ref_src|igshid|_ga|_gl|yclid|wbraid|gbraid|si)=[^&#\s)>\]]*`,
)

func (r *URLTracking) Apply(ctx *Context) (ChangeSet, error) {
	source := ctx.Source
	lines := sourceLines(source)
	var changes ChangeSet

	for _, line := range lines {
		if line.InFence {
			continue
		}
		text := source[line.Start:line.End]
		for _, urlMatch := range urlMatchRe.FindAllIndex(text, -1) {
			urlStart, urlEnd := urlMatch[0], urlMatch[1]
			cleaned := stripTrackingParams(text[urlStart:urlEnd])
			if len(cleaned) >= urlEnd-urlStart {
				continue
			}
			addReplacement(&changes,
				line.Start+urlStart,
				line.Start+urlEnd,
				string(cleaned))
		}
	}
	return changes, nil
}

// stripTrackingParams removes ?utm_*=..., &utm_*=..., etc. from a URL.
// Returns the cleaned URL. If no params matched, returns the input unchanged.
//
// Cleanup details:
//   - Matches ?key=val or &key=val.
//   - After removal, if the URL ends in "?" or "&", the trailing punctuation
//     is dropped.
//   - If the FIRST query param was tracking and is removed, the next & is
//     promoted to a ? to keep the URL syntactically valid.
func stripTrackingParams(url []byte) []byte {
	matches := trackingParamRe.FindAllSubmatchIndex(url, -1)
	if len(matches) == 0 {
		return url
	}

	out := make([]byte, 0, len(url))
	cursor := 0
	for _, m := range matches {
		out = append(out, url[cursor:m[0]]...)
		cursor = m[1]
	}
	out = append(out, url[cursor:]...)

	// Repair structure: collapse "?&" → "?", trailing "?" or "&" gone, etc.
	out = repairURLPunctuation(out)
	return out
}

func repairURLPunctuation(url []byte) []byte {
	out := make([]byte, 0, len(url))
	seenQuestion := false
	for i := 0; i < len(url); i++ {
		c := url[i]
		switch c {
		case '?':
			if seenQuestion {
				// Drop redundant '?'.
				continue
			}
			// Drop "?&" → "?" by skipping a following '&'.
			seenQuestion = true
			out = append(out, c)
			for i+1 < len(url) && url[i+1] == '&' {
				i++
			}
		case '&':
			// First separator and we haven't seen '?' yet: promote to '?'.
			if !seenQuestion {
				seenQuestion = true
				out = append(out, '?')
				continue
			}
			// Collapse "&&" → "&".
			if len(out) > 0 && out[len(out)-1] == '&' {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	// Trim trailing '?' or '&'.
	for len(out) > 0 && (out[len(out)-1] == '?' || out[len(out)-1] == '&') {
		out = out[:len(out)-1]
	}
	return out
}

