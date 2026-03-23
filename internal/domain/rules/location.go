package rules

import (
	"context"
	"fmt"

	"distributed-fraud-detection/internal/domain"
)

type LocationRule struct {
	locations     domain.LocationRepository
	maxDistanceKm float64
	score         int
	fallbackScore int
}

func NewLocationRule(locations domain.LocationRepository, maxDistanceKm float64, score, fallbackScore int) *LocationRule {
	return &LocationRule{
		locations:     locations,
		maxDistanceKm: maxDistanceKm,
		score:         score,
		fallbackScore: fallbackScore,
	}
}

func (r *LocationRule) Name() string        { return "location" }
func (r *LocationRule) FallbackScore() int   { return r.fallbackScore }

func (r *LocationRule) Evaluate(ctx context.Context, tx domain.Transaction) (domain.RuleResult, error) {
	lastLoc, err := r.locations.GetLastLocation(ctx, tx.SenderID)
	if err != nil {
		return domain.RuleResult{}, fmt.Errorf("location rule: getting last location: %w", err)
	}

	distance := lastLoc.DistanceKm(tx.Location)

	if distance > r.maxDistanceKm {
		return domain.NewRuleResult(
			"location",
			true,
			r.score,
			fmt.Sprintf("impossible travel: %.0fkm from last location (limit: %.0fkm)", distance, r.maxDistanceKm),
		)
	}

	return domain.NewRuleResult("location", false, 0, "")
}
