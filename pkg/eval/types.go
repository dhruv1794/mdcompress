// Package eval checks whether compressed markdown preserves factual content.
package eval

import (
	"time"

	"github.com/dhruv1794/mdcompress/pkg/compress"
)

const (
	DefaultBackend         = "ollama"
	DefaultModel           = "llama3.1:8b"
	DefaultQuestionsPerDoc = 10
	DefaultThreshold       = 0.95
	DefaultSeeds           = 1
)

type Backend interface {
	Name() string
	Complete(prompt string) (string, error)
}

type Options struct {
	Repo            string
	Rule            string
	Tier            compress.Tier
	QuestionsPerDoc int
	Threshold       float64
	Seeds           int
	Backend         Backend
	Model           string
}

type Question struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type QuestionResult struct {
	Question         Question `json:"question"`
	OriginalAnswer   string   `json:"original_answer"`
	CompressedAnswer string   `json:"compressed_answer"`
	Score            float64  `json:"score"`
	Reason           string   `json:"reason,omitempty"`
}

type FileResult struct {
	Source       string           `json:"source"`
	TokensBefore int              `json:"tokens_before"`
	TokensAfter  int              `json:"tokens_after"`
	TokensSaved  int              `json:"tokens_saved"`
	AverageScore float64          `json:"average_score"`
	Passed       bool             `json:"passed"`
	Questions    []QuestionResult `json:"questions"`
	RulesFired   map[string]int   `json:"rules_fired,omitempty"`
}

type Report struct {
	GeneratedAt     time.Time    `json:"generated_at"`
	Repo            string       `json:"repo"`
	Backend         string       `json:"backend"`
	Model           string       `json:"model"`
	Rule            string       `json:"rule,omitempty"`
	Tier            string       `json:"tier"`
	Threshold       float64      `json:"threshold"`
	QuestionsPerDoc int          `json:"questions_per_doc"`
	Seeds           int          `json:"seeds"`
	AverageScore    float64      `json:"average_score"`
	Passed          bool         `json:"passed"`
	Files           []FileResult `json:"files"`
}
