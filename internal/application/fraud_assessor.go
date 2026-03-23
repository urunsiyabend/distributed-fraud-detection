package application

import (
	"context"
	"fmt"
	"time"

	"distributed-fraud-detection/internal/domain"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type FraudAssessor struct {
	factory     domain.RuleFactory
	ruleMetrics domain.RuleMetrics
	assessment  domain.AssessmentMetrics
	tracer      trace.Tracer
	now         func() time.Time
}

func NewFraudAssessor(
	factory domain.RuleFactory,
	ruleMetrics domain.RuleMetrics,
	assessment domain.AssessmentMetrics,
	tracer trace.Tracer,
	now func() time.Time,
) *FraudAssessor {
	return &FraudAssessor{
		factory:     factory,
		ruleMetrics: ruleMetrics,
		assessment:  assessment,
		tracer:      tracer,
		now:         now,
	}
}

func (s *FraudAssessor) Assess(ctx context.Context, tx domain.Transaction) (domain.FraudAssessment, error) {
	ctx, span := s.tracer.Start(ctx, "fraud.assess",
		trace.WithAttributes(
			attribute.String("transaction.id", tx.ID),
			attribute.String("transaction.sender", tx.SenderID),
			attribute.Float64("transaction.amount", tx.Amount.Amount),
			attribute.String("transaction.currency", tx.Amount.Currency),
		),
	)
	defer span.End()

	start := s.now()

	ruleSet, err := s.factory.Build(ctx, tx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "building rules failed")
		return domain.FraudAssessment{}, fmt.Errorf("building rules: %w", err)
	}

	var results []domain.RuleResult
	for _, rule := range ruleSet {
		result, err := s.evaluateRule(ctx, rule, tx)
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

	fa, err := domain.NewFraudAssessment(tx.ID, results, s.now())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "creating assessment failed")
		return domain.FraudAssessment{}, fmt.Errorf("creating assessment: %w", err)
	}

	duration := s.now().Sub(start)

	span.SetAttributes(
		attribute.String("assessment.decision", string(fa.Decision)),
		attribute.Int("assessment.risk_score", fa.RiskScore.Value),
		attribute.Float64("assessment.duration_ms", float64(duration.Milliseconds())),
	)

	s.assessment.AssessmentDuration(duration.Seconds())
	s.assessment.DecisionMade(fa.Decision)

	return fa, nil
}

func (s *FraudAssessor) evaluateRule(ctx context.Context, rule domain.Rule, tx domain.Transaction) (domain.RuleResult, error) {
	ctx, span := s.tracer.Start(ctx, "fraud.rule."+rule.Name(),
		trace.WithAttributes(attribute.String("rule.name", rule.Name())),
	)
	defer span.End()

	result, err := rule.Evaluate(ctx, tx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rule evaluation failed")
		return domain.RuleResult{}, err
	}

	span.SetAttributes(
		attribute.Bool("rule.triggered", result.Triggered),
		attribute.Int("rule.score", result.Score),
	)

	return result, nil
}
