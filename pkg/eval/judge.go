package eval

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type judgement struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

func JudgeAnswers(backend Backend, question Question, originalAnswer, compressedAnswer string) (float64, string, error) {
	prompt := fmt.Sprintf(`You are judging whether markdown compression preserved factual content.

Question: %s

Answer from original document:
%s

Answer from compressed document:
%s

Return only JSON in this shape:
{"score":1.0,"reason":"brief reason"}

Score 1.0 when the compressed answer preserves all facts needed to answer the question.
Score 0.5 when it is partially equivalent or missing minor detail.
Score 0.0 when it contradicts the original, is not found, or loses important facts.`, question.Text, fenced(originalAnswer), fenced(compressedAnswer))

	raw, err := backend.Complete(prompt)
	if err != nil {
		return 0, "", err
	}
	score, reason, err := parseJudgement(raw)
	if err != nil {
		return 0, "", err
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score, reason, nil
}

func parseJudgement(raw string) (float64, string, error) {
	var j judgement
	if err := json.Unmarshal([]byte(extractJSON(raw)), &j); err == nil {
		return j.Score, strings.TrimSpace(j.Reason), nil
	}

	re := regexp.MustCompile(`(?i)score[^0-9]*(0(?:\.\d+)?|1(?:\.0+)?)`)
	match := re.FindStringSubmatch(raw)
	if len(match) == 2 {
		score, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			return score, strings.TrimSpace(raw), nil
		}
	}
	return 0, "", fmt.Errorf("backend returned no parseable judgement")
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		return strings.TrimSpace(raw)
	}
	start := strings.IndexAny(raw, "{[")
	end := strings.LastIndexAny(raw, "}]")
	if start >= 0 && end >= start {
		return raw[start : end+1]
	}
	return raw
}
