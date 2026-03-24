package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain/rules"
)

type SlowPathAssessor struct {
	locationRepo domain.LocationRepository
	config       domain.ConfigRepository
	publisher    domain.EventPublisher
	notifier     domain.WebhookNotifier
	idempotency  domain.IdempotencyStore
	ruleMetrics  domain.RuleMetrics
	logger       *slog.Logger
	now          func() time.Time
}

type SlowPathDeps struct {
	LocationRepo domain.LocationRepository
	Config       domain.ConfigRepository
	Publisher    domain.EventPublisher
	Notifier     domain.WebhookNotifier
	Idempotency  domain.IdempotencyStore
	RuleMetrics  domain.RuleMetrics
	Logger       *slog.Logger
	Now          func() time.Time
}

func NewSlowPathAssessor(deps SlowPathDeps) *SlowPathAssessor {
	return &SlowPathAssessor{
		locationRepo: deps.LocationRepo,
		config:       deps.Config,
		publisher:    deps.Publisher,
		notifier:     deps.Notifier,
		idempotency:  deps.Idempotency,
		ruleMetrics:  deps.RuleMetrics,
		logger:       deps.Logger,
		now:          deps.Now,
	}
}

func (s *SlowPathAssessor) Process(ctx context.Context, event domain.AssessmentCompletedEvent) error {
	slowRules, err := s.buildSlowRules(ctx)
	if err != nil {
		return fmt.Errorf("building slow path rules: %w", err)
	}

	// Build a minimal transaction for rule evaluation.
	// In production, you'd load the full transaction from a store.
	tx := domain.Transaction{
		ID:       event.TransactionID,
		SenderID: event.TransactionID, // placeholder — need TransactionRepository
	}

	var results []domain.RuleResult
	for _, rule := range slowRules {
		result, err := rule.Evaluate(ctx, tx)
		if err != nil {
			s.ruleMetrics.RuleFallback(rule.Name())
			results = append(results, domain.NewFallbackRuleResult(rule.Name(), rule.FallbackScore(), err))
			continue
		}
		if result.Triggered {
			s.ruleMetrics.RuleTriggered(rule.Name())
		}
		results = append(results, result)
	}

	// Combine fast path score with slow path results
	combinedScore := event.RiskScore.Value
	for _, r := range results {
		if r.Triggered {
			combinedScore += r.Score
		}
	}
	if combinedScore > 100 {
		combinedScore = 100
	}

	newScore, _ := domain.NewRiskScore(combinedScore)
	newDecision := deriveSlowPathDecision(newScore, results)

	// Did the decision change?
	if newDecision != event.Decision {
		s.logger.InfoContext(ctx, "slow path decision override",
			slog.String("transaction_id", event.TransactionID),
			slog.String("fast_decision", string(event.Decision)),
			slog.String("slow_decision", string(newDecision)),
			slog.Int("combined_score", combinedScore),
		)

		updatedEvent := domain.AssessmentUpdatedEvent{
			TransactionID:    event.TransactionID,
			PreviousDecision: event.Decision,
			NewDecision:      newDecision,
			RiskScore:        newScore,
			SlowPathRules:    results,
			Timestamp:        s.now(),
		}

		if err := s.publisher.Publish(ctx, updatedEvent); err != nil {
			s.logger.ErrorContext(ctx, "failed to publish updated event",
				slog.String("transaction_id", event.TransactionID),
				slog.String("error", err.Error()),
			)
		}

		if newDecision == domain.DecisionBlocked {
			fraudEvent := domain.FraudDetectedEvent{
				TransactionID: event.TransactionID,
				RiskScore:     newScore,
				RuleResults:   results,
				Timestamp:     s.now(),
			}
			if err := s.publisher.Publish(ctx, fraudEvent); err != nil {
				s.logger.ErrorContext(ctx, "failed to publish fraud detected event",
					slog.String("transaction_id", event.TransactionID),
					slog.String("error", err.Error()),
				)
			}
		}

		// Update idempotency cache with new decision
		// This ensures subsequent reads see the slow path result
		s.idempotency.Set(ctx, event.TransactionID, []byte(newDecision))

		// Notify via webhook
		if err := s.notifier.Notify(ctx, event.TransactionID, newDecision, newScore); err != nil {
			s.logger.WarnContext(ctx, "webhook notification failed",
				slog.String("transaction_id", event.TransactionID),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

func (s *SlowPathAssessor) buildSlowRules(ctx context.Context) ([]domain.Rule, error) {
	maxDistKm, err := s.config.GetFloat(ctx, "rules.location.max_distance_km")
	if err != nil {
		return nil, fmt.Errorf("loading location max distance: %w", err)
	}

	locScore, err := s.config.GetInt(ctx, "rules.location.score")
	if err != nil {
		return nil, fmt.Errorf("loading location score: %w", err)
	}

	locFallback, err := s.config.GetInt(ctx, "rules.location.fallback_score")
	if err != nil {
		return nil, fmt.Errorf("loading location fallback score: %w", err)
	}

	patternScore, err := s.config.GetInt(ctx, "rules.pattern.score")
	if err != nil {
		return nil, fmt.Errorf("loading pattern score: %w", err)
	}

	patternFallback, err := s.config.GetInt(ctx, "rules.pattern.fallback_score")
	if err != nil {
		return nil, fmt.Errorf("loading pattern fallback score: %w", err)
	}

	return []domain.Rule{
		rules.NewLocationRule(s.locationRepo, maxDistKm, locScore, locFallback),
		rules.NewPatternRule(patternScore, patternFallback),
	}, nil
}

func deriveSlowPathDecision(score domain.RiskScore, results []domain.RuleResult) domain.Decision {
	for _, r := range results {
		if r.Triggered && r.Score >= 80 {
			return domain.DecisionBlocked
		}
	}
	if score.IsHighRisk() {
		return domain.DecisionBlocked
	}
	if score.IsReview() {
		return domain.DecisionReview
	}
	return domain.DecisionApproved
}
