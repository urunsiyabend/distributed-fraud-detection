package application

import (
	"context"
	"fmt"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain/rules"
)

type FraudRuleFactory struct {
	counter domain.TransactionCounter
	devices domain.DeviceRepository
	config  domain.ConfigRepository
}

func NewFraudRuleFactory(
	counter domain.TransactionCounter,
	devices domain.DeviceRepository,
	config domain.ConfigRepository,
) *FraudRuleFactory {
	return &FraudRuleFactory{
		counter: counter,
		devices: devices,
		config:  config,
	}
}

func (f *FraudRuleFactory) Build(ctx context.Context, tx domain.Transaction) ([]domain.Rule, error) {
	amountThreshold, err := f.config.GetFloat(ctx, "rules.amount.threshold")
	if err != nil {
		return nil, fmt.Errorf("loading amount threshold: %w", err)
	}

	maxCount, err := f.config.GetInt(ctx, "rules.velocity.max_count")
	if err != nil {
		return nil, fmt.Errorf("loading velocity max count: %w", err)
	}

	windowMins, err := f.config.GetInt(ctx, "rules.velocity.window_minutes")
	if err != nil {
		return nil, fmt.Errorf("loading velocity window: %w", err)
	}

	velocityScore, err := f.config.GetInt(ctx, "rules.velocity.score")
	if err != nil {
		return nil, fmt.Errorf("loading velocity score: %w", err)
	}

	velocityFallback, err := f.config.GetInt(ctx, "rules.velocity.fallback_score")
	if err != nil {
		return nil, fmt.Errorf("loading velocity fallback score: %w", err)
	}

	amountScore, err := f.config.GetInt(ctx, "rules.amount.score")
	if err != nil {
		return nil, fmt.Errorf("loading amount score: %w", err)
	}

	amountCriticalScore, err := f.config.GetInt(ctx, "rules.amount.critical_score")
	if err != nil {
		return nil, fmt.Errorf("loading amount critical score: %w", err)
	}

	amountFallback, err := f.config.GetInt(ctx, "rules.amount.fallback_score")
	if err != nil {
		return nil, fmt.Errorf("loading amount fallback score: %w", err)
	}

	deviceMissingScore, err := f.config.GetInt(ctx, "rules.device.missing_score")
	if err != nil {
		return nil, fmt.Errorf("loading device missing score: %w", err)
	}

	deviceUnknownScore, err := f.config.GetInt(ctx, "rules.device.unknown_score")
	if err != nil {
		return nil, fmt.Errorf("loading device unknown score: %w", err)
	}

	deviceFallback, err := f.config.GetInt(ctx, "rules.device.fallback_score")
	if err != nil {
		return nil, fmt.Errorf("loading device fallback score: %w", err)
	}

	return []domain.Rule{
		rules.NewVelocityRule(f.counter, maxCount, windowMins, velocityScore, velocityFallback),
		rules.NewAmountRule(amountThreshold, amountScore, amountCriticalScore, amountFallback),
		rules.NewDeviceRule(f.devices, deviceMissingScore, deviceUnknownScore, deviceFallback),
	}, nil
}
