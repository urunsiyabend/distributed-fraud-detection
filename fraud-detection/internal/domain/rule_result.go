package domain

import "fmt"

type RuleResult struct {
	RuleName  string
	Triggered bool
	Score     int
	Reason    string
	Fallback  bool
}

func NewRuleResult(ruleName string, triggered bool, score int, reason string) (RuleResult, error) {
	if ruleName == "" {
		return RuleResult{}, fmt.Errorf("rule name must not be empty")
	}
	if score < 0 || score > 100 {
		return RuleResult{}, fmt.Errorf("rule score must be between 0 and 100, got %d", score)
	}
	if triggered && reason == "" {
		return RuleResult{}, fmt.Errorf("triggered rule must have a reason")
	}
	return RuleResult{
		RuleName:  ruleName,
		Triggered: triggered,
		Score:     score,
		Reason:    reason,
	}, nil
}

func NewFallbackRuleResult(ruleName string, score int, err error) RuleResult {
	return RuleResult{
		RuleName:  ruleName,
		Triggered: true,
		Score:     score,
		Reason:    fmt.Sprintf("fallback: %v", err),
		Fallback:  true,
	}
}
