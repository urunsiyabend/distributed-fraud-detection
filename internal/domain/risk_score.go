package domain

import "fmt"

type RiskScore struct {
	Value int
}

func NewRiskScore(value int) (RiskScore, error) {
	if value < 0 || value > 100 {
		return RiskScore{}, fmt.Errorf("risk score must be between 0 and 100, got %d", value)
	}
	return RiskScore{Value: value}, nil
}

func (r RiskScore) IsHighRisk() bool {
	return r.Value > 70
}

func (r RiskScore) IsReview() bool {
	return r.Value >= 40 && r.Value <= 70
}
