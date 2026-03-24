package domain

import "context"

type Rule interface {
	Name() string
	FallbackScore() int
	Evaluate(ctx context.Context, tx Transaction) (RuleResult, error)
}

type RuleFactory interface {
	Build(ctx context.Context, tx Transaction) ([]Rule, error)
}

type RuleMetrics interface {
	RuleFallback(ruleName string)
	RuleTriggered(ruleName string)
}

type AssessmentMetrics interface {
	AssessmentDuration(seconds float64)
	DecisionMade(decision Decision)
}
