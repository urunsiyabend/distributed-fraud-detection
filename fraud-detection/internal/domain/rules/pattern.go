package rules

import (
	"context"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
)

type PatternRule struct {
	score         int
	fallbackScore int
}

func NewPatternRule(score, fallbackScore int) *PatternRule {
	return &PatternRule{score: score, fallbackScore: fallbackScore}
}

func (r *PatternRule) Name() string        { return "pattern" }
func (r *PatternRule) FallbackScore() int   { return r.fallbackScore }

func (r *PatternRule) Evaluate(_ context.Context, _ domain.Transaction) (domain.RuleResult, error) {
	// Placeholder: deep pattern analysis (ML model, historical patterns, etc.)
	return domain.NewRuleResult("pattern", false, 0, "")
}
