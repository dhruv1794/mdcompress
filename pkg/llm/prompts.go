package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// promptVersion is bumped whenever the rewrite or judge prompt changes so
// cached responses for the old prompt are not reused by the new one.
const promptVersion = "v1"

const rewriteTemplate = `Rewrite the following markdown section to use fewer words while
preserving every factual claim, code identifier, version number,
command, and link. Do not change code blocks. Do not change tables.
Do not change quoted text. Do not change headings. Output only the
rewritten markdown — no preamble, no explanation, no surrounding
fences.

Section:
%s
`

const judgeTemplate = `You are judging whether a markdown rewrite preserved every factual claim
from the original. Compare the two sections sentence by sentence.

Original:
%s

Rewrite:
%s

Score 1.0 when the rewrite preserves all factual claims, code identifiers,
version numbers, commands, links, and named entities from the original.
Score 0.5 when minor detail is missing or paraphrased loosely.
Score 0.0 when any factual claim, code identifier, version number, command,
link, or named entity in the original is missing from or contradicted by the
rewrite.

Return only JSON in this shape:
{"score":1.0,"reason":"brief reason"}`

// RewritePrompt returns the strict preserve-facts rewrite prompt for a section.
func RewritePrompt(section string) string {
	return fmt.Sprintf(rewriteTemplate, section)
}

// JudgePrompt returns the per-section faithfulness judge prompt.
func JudgePrompt(original, rewrite string) string {
	return fmt.Sprintf(judgeTemplate, original, rewrite)
}

// PromptVersion returns the current prompt version tag for cache keying.
func PromptVersion() string { return promptVersion }

// PromptHash returns the hex sha256 of the rewrite prompt template plus
// version. Cached entries that were produced under a different prompt do not
// collide with new runs.
func PromptHash() string {
	sum := sha256.Sum256([]byte(promptVersion + "\x00" + rewriteTemplate))
	return hex.EncodeToString(sum[:])
}

// SectionHash hashes a normalized form of the section text so identical
// sections share a cache entry across files.
func SectionHash(section string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(section)))
	return hex.EncodeToString(sum[:])
}

// CacheKey combines section, prompt, and model identifiers per V6-SPEC.
func CacheKey(sectionSHA, promptSHA, modelID string) string {
	sum := sha256.Sum256([]byte(sectionSHA + "\x00" + promptSHA + "\x00" + modelID))
	return hex.EncodeToString(sum[:])
}
