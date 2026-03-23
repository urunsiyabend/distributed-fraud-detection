package rules

import (
	"context"
	"fmt"

	"distributed-fraud-detection/internal/domain"
)

type DeviceRule struct {
	devices        domain.DeviceRepository
	missingScore   int
	unknownScore   int
	fallbackScore  int
}

func NewDeviceRule(devices domain.DeviceRepository, missingScore, unknownScore, fallbackScore int) *DeviceRule {
	return &DeviceRule{
		devices:       devices,
		missingScore:  missingScore,
		unknownScore:  unknownScore,
		fallbackScore: fallbackScore,
	}
}

func (r *DeviceRule) Name() string        { return "device" }
func (r *DeviceRule) FallbackScore() int   { return r.fallbackScore }

func (r *DeviceRule) Evaluate(ctx context.Context, tx domain.Transaction) (domain.RuleResult, error) {
	if tx.DeviceID == "" {
		return domain.NewRuleResult(
			"device",
			true,
			r.missingScore,
			"no device ID provided",
		)
	}

	known, err := r.devices.IsKnownDevice(ctx, tx.SenderID, tx.DeviceID)
	if err != nil {
		return domain.RuleResult{}, fmt.Errorf("device rule: checking device: %w", err)
	}

	if !known {
		return domain.NewRuleResult(
			"device",
			true,
			r.unknownScore,
			fmt.Sprintf("device %s not recognized for sender %s", tx.DeviceID, tx.SenderID),
		)
	}

	return domain.NewRuleResult("device", false, 0, "")
}
