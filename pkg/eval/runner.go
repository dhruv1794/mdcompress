package eval

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhruv1794/mdcompress/pkg/compress"
	"github.com/dhruv1794/mdcompress/pkg/rules"
)

func Run(opts Options) (Report, error) {
	opts = normalizeOptions(opts)
	if opts.Backend == nil {
		return Report{}, fmt.Errorf("eval backend is required")
	}

	paths, err := markdownPaths(opts.Repo)
	if err != nil {
		return Report{}, err
	}
	disabled, err := disabledRulesFor(opts.Rule)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		GeneratedAt:     time.Now().UTC(),
		Repo:            opts.Repo,
		Backend:         opts.Backend.Name(),
		Model:           opts.Model,
		Rule:            opts.Rule,
		Tier:            opts.Tier.String(),
		Threshold:       opts.Threshold,
		QuestionsPerDoc: opts.QuestionsPerDoc,
		Seeds:           opts.Seeds,
	}

	for _, path := range paths {
		file, err := runFile(opts, path, disabled)
		if err != nil {
			return report, err
		}
		report.Files = append(report.Files, file)
	}
	report.AverageScore = averageFileScore(report.Files)
	report.Passed = report.AverageScore >= opts.Threshold
	return report, nil
}

func normalizeOptions(opts Options) Options {
	if strings.TrimSpace(opts.Repo) == "" {
		opts.Repo = "."
	}
	if opts.Tier == 0 {
		opts.Tier = compress.TierSafe
	}
	if opts.QuestionsPerDoc <= 0 {
		opts.QuestionsPerDoc = DefaultQuestionsPerDoc
	}
	if opts.Threshold <= 0 {
		opts.Threshold = DefaultThreshold
	}
	if opts.Seeds <= 0 {
		opts.Seeds = DefaultSeeds
	}
	return opts
}

func runFile(opts Options, path string, disabled []string) (FileResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileResult{}, err
	}
	result, err := compress.Compress(content, compress.Options{
		Tier:          opts.Tier,
		DisabledRules: disabled,
	})
	if err != nil {
		return FileResult{}, err
	}

	rel, err := filepath.Rel(opts.Repo, path)
	if err != nil {
		rel = path
	}
	file := FileResult{
		Source:       filepath.ToSlash(rel),
		TokensBefore: result.TokensBefore,
		TokensAfter:  result.TokensAfter,
		TokensSaved:  result.TokensSaved(),
		RulesFired:   result.RulesFired,
	}
	if bytes.Equal(content, result.Output) {
		file.AverageScore = 1
		file.Passed = true
		return file, nil
	}

	for seed := 0; seed < opts.Seeds; seed++ {
		questions, err := generateQuestions(opts.Backend, string(content), opts.QuestionsPerDoc, seed+1)
		if err != nil {
			return file, fmt.Errorf("%s: generate questions: %w", file.Source, err)
		}
		for _, question := range questions {
			originalAnswer, err := AnswerQuestion(opts.Backend, string(content), question)
			if err != nil {
				return file, fmt.Errorf("%s: answer original: %w", file.Source, err)
			}
			compressedAnswer, err := AnswerQuestion(opts.Backend, string(result.Output), question)
			if err != nil {
				return file, fmt.Errorf("%s: answer compressed: %w", file.Source, err)
			}
			score, reason, err := JudgeAnswers(opts.Backend, question, originalAnswer, compressedAnswer)
			if err != nil {
				return file, fmt.Errorf("%s: judge answers: %w", file.Source, err)
			}
			file.Questions = append(file.Questions, QuestionResult{
				Question:         question,
				OriginalAnswer:   originalAnswer,
				CompressedAnswer: compressedAnswer,
				Score:            score,
				Reason:           reason,
			})
		}
	}
	file.AverageScore = averageQuestionScore(file.Questions)
	file.Passed = file.AverageScore >= opts.Threshold
	return file, nil
}

func disabledRulesFor(onlyRule string) ([]string, error) {
	onlyRule = strings.TrimSpace(onlyRule)
	if onlyRule == "" {
		return nil, nil
	}
	var disabled []string
	found := false
	for _, rule := range rules.AllRules() {
		if rule.Name() == onlyRule {
			found = true
			continue
		}
		disabled = append(disabled, rule.Name())
	}
	if !found {
		return nil, fmt.Errorf("unknown rule %q", onlyRule)
	}
	return disabled, nil
}

func markdownPaths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if excludedEvalDir(path) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func excludedEvalDir(path string) bool {
	slash := filepath.ToSlash(filepath.Clean(path))
	for _, prefix := range []string{".git", ".mdcompress/cache", "node_modules", "vendor"} {
		if slash == prefix || strings.HasSuffix(slash, "/"+prefix) || strings.Contains(slash, "/"+prefix+"/") {
			return true
		}
	}
	return false
}

func averageQuestionScore(results []QuestionResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var total float64
	for _, result := range results {
		total += result.Score
	}
	return total / float64(len(results))
}

func averageFileScore(files []FileResult) float64 {
	if len(files) == 0 {
		return 0
	}
	var total float64
	for _, file := range files {
		total += file.AverageScore
	}
	return total / float64(len(files))
}
