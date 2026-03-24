package domain

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

type AssessmentCompletedEvent struct {
	TransactionID string
	Decision      Decision
	RiskScore     RiskScore
	Timestamp     time.Time
}

func (e AssessmentCompletedEvent) EventName() string    { return "assessment.completed" }
func (e AssessmentCompletedEvent) OccurredAt() time.Time { return e.Timestamp }

type FraudDetectedEvent struct {
	TransactionID string
	RiskScore     RiskScore
	RuleResults   []RuleResult
	Timestamp     time.Time
}

func (e FraudDetectedEvent) EventName() string    { return "fraud.detected" }
func (e FraudDetectedEvent) OccurredAt() time.Time { return e.Timestamp }

type AssessmentUpdatedEvent struct {
	TransactionID    string
	PreviousDecision Decision
	NewDecision      Decision
	RiskScore        RiskScore
	SlowPathRules    []RuleResult
	Timestamp        time.Time
}

func (e AssessmentUpdatedEvent) EventName() string    { return "assessment.updated" }
func (e AssessmentUpdatedEvent) OccurredAt() time.Time { return e.Timestamp }
