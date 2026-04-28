// Package rules contains deterministic markdown compression rules.
package rules

import (
	"github.com/dhruv1794/mdcompress/pkg/render"
	"github.com/yuin/goldmark/ast"
)

// Tier identifies a rule's risk level.
type Tier int

const (
	TierSafe Tier = iota + 1
	TierAggressive
	TierLLM
)

func (t Tier) String() string {
	switch t {
	case TierSafe:
		return "safe"
	case TierAggressive:
		return "aggressive"
	case TierLLM:
		return "llm"
	default:
		return "unknown"
	}
}

// Context is passed to every rule. Rules read Source and return byte ranges
// to remove; they should not mutate the source bytes.
type Context struct {
	Source []byte
	Config *Config
}

// Config contains the active rule configuration.
type Config struct {
	Tier     Tier
	Disabled map[string]bool
}

// Stats reports a rule's effect.
type Stats struct {
	NodesAffected int
	BytesSaved    int
}

// ChangeSet describes source ranges removed by a rule.
type ChangeSet struct {
	Ranges []render.Range
	Stats  Stats
}

// Rule is the contract implemented by every compression rule.
type Rule interface {
	Name() string
	Tier() Tier
	Apply(doc ast.Node, ctx *Context) (ChangeSet, error)
}
