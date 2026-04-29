package compress

import (
	"fmt"
	"strings"
)

// Tier selects the rule set to run.
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

// ParseTier parses a CLI/config tier name.
func ParseTier(value string) (Tier, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "safe":
		return TierSafe, nil
	case "aggressive":
		return TierAggressive, nil
	case "llm":
		return TierLLM, nil
	default:
		return 0, fmt.Errorf("unknown tier %q", value)
	}
}

// Options controls a compression run.
type Options struct {
	Tier          Tier
	EnabledRules  []string
	DisabledRules []string
}
