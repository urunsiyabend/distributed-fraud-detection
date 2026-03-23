package domain

import "fmt"

type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionBlocked  Decision = "blocked"
	DecisionReview   Decision = "review"
)

func NewDecision(value string) (Decision, error) {
	d := Decision(value)
	switch d {
	case DecisionApproved, DecisionBlocked, DecisionReview:
		return d, nil
	default:
		return "", fmt.Errorf("invalid decision: %s (must be approved, blocked, or review)", value)
	}
}
