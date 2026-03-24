package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedTime = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

func rr(name string, triggered bool, score int, reason string) RuleResult {
	r, _ := NewRuleResult(name, triggered, score, reason)
	return r
}

func TestNewFraudAssessment_Validation(t *testing.T) {
	tests := []struct {
		name          string
		transactionID string
		ruleResults   []RuleResult
		wantErr       string
	}{
		{
			"empty transaction ID",
			"",
			[]RuleResult{rr("r1", false, 0, "")},
			"transaction ID must not be empty",
		},
		{
			"nil rule results",
			"tx-1",
			nil,
			"at least one rule result is required",
		},
		{
			"empty rule results",
			"tx-1",
			[]RuleResult{},
			"at least one rule result is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFraudAssessment(tt.transactionID, tt.ruleResults, fixedTime)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestComputeRiskScore(t *testing.T) {
	tests := []struct {
		name     string
		results  []RuleResult
		expected int
	}{
		{
			"single triggered rule",
			[]RuleResult{rr("r1", true, 30, "reason")},
			30,
		},
		{
			"multiple triggered rules",
			[]RuleResult{
				rr("r1", true, 30, "reason"),
				rr("r2", true, 25, "reason"),
			},
			55,
		},
		{
			"cap at 100",
			[]RuleResult{
				rr("r1", true, 60, "reason"),
				rr("r2", true, 60, "reason"),
			},
			100,
		},
		{
			"non-triggered rules ignored",
			[]RuleResult{
				rr("r1", true, 20, "reason"),
				rr("r2", false, 0, ""),
			},
			20,
		},
		{
			"all non-triggered",
			[]RuleResult{rr("r1", false, 0, "")},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := computeRiskScore(tt.results)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, score.Value)
		})
	}
}

func TestDeriveDecision(t *testing.T) {
	tests := []struct {
		name     string
		score    int
		results  []RuleResult
		expected Decision
	}{
		{
			"approved — low score",
			20,
			[]RuleResult{rr("r1", true, 20, "reason")},
			DecisionApproved,
		},
		{
			"review — score 50",
			50,
			[]RuleResult{rr("r1", true, 50, "reason")},
			DecisionReview,
		},
		{
			"review — score 40 boundary",
			40,
			[]RuleResult{rr("r1", true, 40, "reason")},
			DecisionReview,
		},
		{
			"review — score 70 boundary",
			70,
			[]RuleResult{rr("r1", true, 70, "reason")},
			DecisionReview,
		},
		{
			"blocked — high score",
			80,
			[]RuleResult{rr("r1", true, 80, "reason")},
			DecisionBlocked,
		},
		{
			"blocked — critical single rule",
			30,
			[]RuleResult{
				rr("r1", true, 30, "reason"),
				rr("critical", true, 80, "critical reason"),
			},
			DecisionBlocked,
		},
		{
			"approved — zero score",
			0,
			[]RuleResult{rr("r1", false, 0, "")},
			DecisionApproved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := RiskScore{Value: tt.score}
			got := deriveDecision(score, tt.results)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFraudAssessment_Events(t *testing.T) {
	tests := []struct {
		name            string
		results         []RuleResult
		expectedEvents  int
		hasFraudEvent   bool
		expectDecision  Decision
	}{
		{
			"approved — only AssessmentCompleted",
			[]RuleResult{rr("r1", true, 10, "minor")},
			1,
			false,
			DecisionApproved,
		},
		{
			"review — only AssessmentCompleted",
			[]RuleResult{rr("r1", true, 50, "medium")},
			1,
			false,
			DecisionReview,
		},
		{
			"blocked — AssessmentCompleted + FraudDetected",
			[]RuleResult{rr("r1", true, 80, "critical")},
			2,
			true,
			DecisionBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fa, err := NewFraudAssessment("tx-1", tt.results, fixedTime)
			require.NoError(t, err)

			assert.Equal(t, tt.expectDecision, fa.Decision)
			assert.Len(t, fa.Events(), tt.expectedEvents)

			// First event is always AssessmentCompleted
			completed, ok := fa.Events()[0].(AssessmentCompletedEvent)
			require.True(t, ok)
			assert.Equal(t, "tx-1", completed.TransactionID)
			assert.Equal(t, tt.expectDecision, completed.Decision)
			assert.Equal(t, fixedTime, completed.Timestamp)

			if tt.hasFraudEvent {
				fraud, ok := fa.Events()[1].(FraudDetectedEvent)
				require.True(t, ok)
				assert.Equal(t, "tx-1", fraud.TransactionID)
				assert.Equal(t, fixedTime, fraud.Timestamp)
			}
		})
	}
}
