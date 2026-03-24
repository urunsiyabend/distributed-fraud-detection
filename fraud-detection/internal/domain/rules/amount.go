package rules

import (
	"context"
	"fmt"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
)

type AmountRule struct {
	threshold      float64
	score          int
	criticalScore  int
	fallbackScore  int
}

func NewAmountRule(threshold float64, score, criticalScore, fallbackScore int) *AmountRule {
	return &AmountRule{
		threshold:     threshold,
		score:         score,
		criticalScore: criticalScore,
		fallbackScore: fallbackScore,
	}
}

func (r *AmountRule) Name() string        { return "amount" }
func (r *AmountRule) FallbackScore() int   { return r.fallbackScore }

func (r *AmountRule) Evaluate(_ context.Context, tx domain.Transaction) (domain.RuleResult, error) {
	if tx.Amount.Amount > r.threshold {
		score := r.score
		if tx.Amount.Amount > r.threshold*3 {
			score = r.criticalScore
		}

		return domain.NewRuleResult(
			"amount",
			true,
			score,
			fmt.Sprintf("amount %.2f %s exceeds threshold %.2f", tx.Amount.Amount, tx.Amount.Currency, r.threshold),
		)
	}

	return domain.NewRuleResult("amount", false, 0, "")
}
