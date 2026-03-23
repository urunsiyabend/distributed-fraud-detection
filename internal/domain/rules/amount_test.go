package rules

import (
	"context"
	"testing"

	"distributed-fraud-detection/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmountRule_Evaluate(t *testing.T) {
	// threshold=1000, score=40, criticalScore=80, fallbackScore=20
	tests := []struct {
		name          string
		amount        float64
		threshold     float64
		wantTriggered bool
		wantScore     int
	}{
		{"below threshold — not triggered", 500, 1000, false, 0},
		{"at threshold — not triggered", 1000, 1000, false, 0},
		{"above threshold — normal score", 1500, 1000, true, 40},
		{"above 3x threshold — critical score", 3500, 1000, true, 80},
		{"exactly 3x — normal score", 3000, 1000, true, 40},
		{"just above 3x — critical score", 3000.01, 1000, true, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewAmountRule(tt.threshold, 40, 80, 20)

			assert.Equal(t, "amount", rule.Name())
			assert.Equal(t, 20, rule.FallbackScore())

			tx := domain.Transaction{
				Amount: domain.Money{Amount: tt.amount, Currency: "USD"},
			}

			result, err := rule.Evaluate(context.Background(), tx)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTriggered, result.Triggered)
			if tt.wantTriggered {
				assert.Equal(t, tt.wantScore, result.Score)
				assert.Contains(t, result.Reason, "exceeds threshold")
			}
		})
	}
}
