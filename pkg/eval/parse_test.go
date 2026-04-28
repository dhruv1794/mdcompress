package eval

import "testing"

func TestParseQuestionsJSON(t *testing.T) {
	questions, err := parseQuestions("```json\n{\"questions\":[{\"text\":\"What is supported?\"}]}\n```", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 1 || questions[0].ID != "q1" || questions[0].Text != "What is supported?" {
		t.Fatalf("questions = %#v", questions)
	}
}

func TestParseJudgementClampsScore(t *testing.T) {
	score, reason, err := parseJudgement(`{"score":1.25,"reason":"equivalent"}`)
	if err != nil {
		t.Fatal(err)
	}
	if score != 1.25 || reason != "equivalent" {
		t.Fatalf("score=%v reason=%q", score, reason)
	}
}
