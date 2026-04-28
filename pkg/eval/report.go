package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

func WriteJSON(w io.Writer, report Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteMarkdown(w io.Writer, report Report) error {
	status := "FAIL"
	if report.Passed {
		status = "PASS"
	}
	if _, err := fmt.Fprintf(w, "# mdcompress Faithfulness Eval\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Status: %s\n- Repo: `%s`\n- Tier: `%s`\n", status, report.Repo, report.Tier); err != nil {
		return err
	}
	if report.Rule != "" {
		if _, err := fmt.Fprintf(w, "- Rule: `%s`\n", report.Rule); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "- Backend: `%s`\n- Model: `%s`\n- Threshold: %.2f\n- Average score: %.3f\n- Files: %d\n\n", report.Backend, report.Model, report.Threshold, report.AverageScore, len(report.Files)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| File | Score | Result | Tokens |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---|---:|---|---:|"); err != nil {
		return err
	}
	for _, file := range report.Files {
		fileStatus := "FAIL"
		if file.Passed {
			fileStatus = "PASS"
		}
		if _, err := fmt.Fprintf(w, "| `%s` | %.3f | %s | %d -> %d |\n", file.Source, file.AverageScore, fileStatus, file.TokensBefore, file.TokensAfter); err != nil {
			return err
		}
	}

	failures := failingQuestions(report)
	if len(failures) == 0 {
		return nil
	}
	if _, err := fmt.Fprint(w, "\n## Low-Scoring Questions\n\n"); err != nil {
		return err
	}
	for _, item := range failures {
		if _, err := fmt.Fprintf(w, "### `%s` %s (%.3f)\n\n", item.file, item.question.Question.ID, item.question.Score); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Question: %s\n- Reason: %s\n\n", item.question.Question.Text, item.question.Reason); err != nil {
			return err
		}
	}
	return nil
}

type failingQuestion struct {
	file     string
	question QuestionResult
}

func failingQuestions(report Report) []failingQuestion {
	var out []failingQuestion
	for _, file := range report.Files {
		for _, question := range file.Questions {
			if question.Score < report.Threshold {
				out = append(out, failingQuestion{file: file.Source, question: question})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].question.Score == out[j].question.Score {
			return out[i].file < out[j].file
		}
		return out[i].question.Score < out[j].question.Score
	})
	return out
}
