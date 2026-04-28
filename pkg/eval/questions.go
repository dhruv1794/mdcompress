package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

func GenerateQuestions(backend Backend, content string, count int) ([]Question, error) {
	return generateQuestions(backend, content, count, 0)
}

func generateQuestions(backend Backend, content string, count int, seed int) ([]Question, error) {
	if count <= 0 {
		count = DefaultQuestionsPerDoc
	}
	prompt := fmt.Sprintf(`You are evaluating markdown compression faithfulness.

Evaluation seed: %d

Generate exactly %d factual questions that can be answered from the document below.
Focus on technical facts, commands, configuration keys, constraints, numbers, APIs, and behavior.
Avoid subjective questions and avoid questions that require outside knowledge.

Return only JSON in this shape:
{"questions":[{"id":"q1","text":"..."}]}

Document:
%s`, seed, count, fenced(content))

	raw, err := backend.Complete(prompt)
	if err != nil {
		return nil, err
	}
	return parseQuestions(raw, count)
}

func parseQuestions(raw string, limit int) ([]Question, error) {
	var wrapped struct {
		Questions []Question `json:"questions"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &wrapped); err == nil && len(wrapped.Questions) > 0 {
		return normalizeQuestions(wrapped.Questions, limit), nil
	}

	var questions []Question
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*0123456789.) ")
		if line == "" {
			continue
		}
		questions = append(questions, Question{
			ID:   fmt.Sprintf("q%d", len(questions)+1),
			Text: line,
		})
		if limit > 0 && len(questions) >= limit {
			break
		}
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("backend returned no parseable questions")
	}
	return questions, nil
}

func normalizeQuestions(questions []Question, limit int) []Question {
	out := make([]Question, 0, len(questions))
	for _, q := range questions {
		q.Text = strings.TrimSpace(q.Text)
		if q.Text == "" {
			continue
		}
		if q.ID == "" {
			q.ID = fmt.Sprintf("q%d", len(out)+1)
		}
		out = append(out, q)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func AnswerQuestion(backend Backend, content string, question Question) (string, error) {
	prompt := fmt.Sprintf(`Answer the question using only the document below.
If the document does not contain the answer, say "not found".
Keep the answer concise and factual.

Question: %s

Document:
%s`, question.Text, fenced(content))

	answer, err := backend.Complete(prompt)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}

func fenced(content string) string {
	return "```markdown\n" + content + "\n```"
}
