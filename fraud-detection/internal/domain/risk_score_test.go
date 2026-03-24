package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRiskScore(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"zero", 0, false},
		{"mid", 50, false},
		{"max", 100, false},
		{"negative", -1, true},
		{"over max", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRiskScore(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.value, got.Value)
		})
	}
}

func TestRiskScore_IsHighRisk(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected bool
	}{
		{"70 is not high risk", 70, false},
		{"71 is high risk", 71, true},
		{"100 is high risk", 100, true},
		{"0 is not high risk", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := RiskScore{Value: tt.value}
			assert.Equal(t, tt.expected, rs.IsHighRisk())
		})
	}
}

func TestRiskScore_IsReview(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected bool
	}{
		{"39 is not review", 39, false},
		{"40 is review", 40, true},
		{"55 is review", 55, true},
		{"70 is review", 70, true},
		{"71 is not review", 71, false},
		{"0 is not review", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := RiskScore{Value: tt.value}
			assert.Equal(t, tt.expected, rs.IsReview())
		})
	}
}
