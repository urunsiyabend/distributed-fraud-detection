package rules

import (
	"context"
	"fmt"
	"time"

	"distributed-fraud-detection/internal/domain"
)

type VelocityRule struct {
	counter       domain.TransactionCounter
	maxCount      int
	windowMins    int
	score         int
	fallbackScore int
}

func NewVelocityRule(counter domain.TransactionCounter, maxCount, windowMins, score, fallbackScore int) *VelocityRule {
	return &VelocityRule{
		counter:       counter,
		maxCount:      maxCount,
		windowMins:    windowMins,
		score:         score,
		fallbackScore: fallbackScore,
	}
}

func (r *VelocityRule) Name() string        { return "velocity" }
func (r *VelocityRule) FallbackScore() int   { return r.fallbackScore }

func (r *VelocityRule) Evaluate(ctx context.Context, tx domain.Transaction) (domain.RuleResult, error) {
	since := tx.Timestamp.Add(-time.Duration(r.windowMins) * time.Minute)

	count, err := r.counter.CountBySender(ctx, tx.SenderID, since)
	if err != nil {
		return domain.RuleResult{}, fmt.Errorf("velocity rule: counting transactions: %w", err)
	}

	if count >= r.maxCount {
		return domain.NewRuleResult(
			"velocity",
			true,
			r.score,
			fmt.Sprintf("sender %s made %d transactions in %d minutes (limit: %d)", tx.SenderID, count, r.windowMins, r.maxCount),
		)
	}

	return domain.NewRuleResult("velocity", false, 0, "")
}
