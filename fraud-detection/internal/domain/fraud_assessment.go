package domain

import (
	"fmt"
	"time"
)

type FraudAssessment struct {
	TransactionID string
	RuleResults   []RuleResult
	RiskScore     RiskScore
	Decision      Decision
	events        []DomainEvent
}

func NewFraudAssessment(transactionID string, ruleResults []RuleResult, now time.Time) (FraudAssessment, error) {
	if transactionID == "" {
		return FraudAssessment{}, fmt.Errorf("transaction ID must not be empty")
	}
	if len(ruleResults) == 0 {
		return FraudAssessment{}, fmt.Errorf("at least one rule result is required")
	}

	score, err := computeRiskScore(ruleResults)
	if err != nil {
		return FraudAssessment{}, fmt.Errorf("computing risk score: %w", err)
	}

	decision := deriveDecision(score, ruleResults)

	fa := FraudAssessment{
		TransactionID: transactionID,
		RuleResults:   ruleResults,
		RiskScore:     score,
		Decision:      decision,
	}

	fa.events = append(fa.events, AssessmentCompletedEvent{
		TransactionID: transactionID,
		Decision:      decision,
		RiskScore:     score,
		Timestamp:     now,
	})

	if decision == DecisionBlocked {
		fa.events = append(fa.events, FraudDetectedEvent{
			TransactionID: transactionID,
			RiskScore:     score,
			RuleResults:   ruleResults,
			Timestamp:     now,
		})
	}

	return fa, nil
}

func (f FraudAssessment) Events() []DomainEvent {
	return f.events
}

func computeRiskScore(results []RuleResult) (RiskScore, error) {
	total := 0
	for _, r := range results {
		if r.Triggered {
			total += r.Score
		}
	}
	if total > 100 {
		total = 100
	}
	return NewRiskScore(total)
}

func deriveDecision(score RiskScore, results []RuleResult) Decision {
	// Any single critical rule (score >= 80) triggers an immediate block.
	for _, r := range results {
		if r.Triggered && r.Score >= 80 {
			return DecisionBlocked
		}
	}

	if score.IsHighRisk() {
		return DecisionBlocked
	}
	if score.IsReview() {
		return DecisionReview
	}
	return DecisionApproved
}
