package rules

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCounter struct {
	count int
	err   error
}

func (m *mockCounter) CountBySender(_ context.Context, _ string, _ time.Time) (int, error) {
	return m.count, m.err
}

func testTx() domain.Transaction {
	return domain.Transaction{
		ID:        "tx-1",
		SenderID:  "user-1",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Amount:    domain.Money{Amount: 100, Currency: "USD"},
	}
}

func TestVelocityRule_Evaluate(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		counterErr    error
		maxCount      int
		score         int
		wantTriggered bool
		wantErr       bool
	}{
		{"count below max — not triggered", 3, nil, 5, 50, false, false},
		{"count equals max — triggered", 5, nil, 5, 50, true, false},
		{"count above max — triggered", 10, nil, 5, 50, true, false},
		{"count zero — not triggered", 0, nil, 5, 50, false, false},
		{"counter error — returns error", 0, errors.New("redis down"), 5, 50, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := &mockCounter{count: tt.count, err: tt.counterErr}
			rule := NewVelocityRule(counter, tt.maxCount, 10, tt.score, 25)

			assert.Equal(t, "velocity", rule.Name())
			assert.Equal(t, 25, rule.FallbackScore())

			result, err := rule.Evaluate(context.Background(), testTx())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTriggered, result.Triggered)
			if tt.wantTriggered {
				assert.Equal(t, tt.score, result.Score)
				assert.NotEmpty(t, result.Reason)
			}
		})
	}
}
