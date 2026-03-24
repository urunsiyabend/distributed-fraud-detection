package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/application"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"
	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/infrastructure/postgres"
	fraudv1 "github.com/urunsiyabend/distributed-fraud-detection/proto/fraud/v1"
)

type FraudServer struct {
	fraudv1.UnimplementedFraudServiceServer
	assessor    *application.FraudAssessor
	uow         domain.UnitOfWork
	assessments *postgres.AssessmentRepository
	outbox      *postgres.OutboxRepository
	now         func() time.Time
}

type FraudServerDeps struct {
	Assessor    *application.FraudAssessor
	UoW         domain.UnitOfWork
	Assessments *postgres.AssessmentRepository
	Outbox      *postgres.OutboxRepository
	Now         func() time.Time
}

func NewFraudServer(deps FraudServerDeps) *FraudServer {
	return &FraudServer{
		assessor:    deps.Assessor,
		uow:        deps.UoW,
		assessments: deps.Assessments,
		outbox:      deps.Outbox,
		now:         deps.Now,
	}
}

func (s *FraudServer) Assess(ctx context.Context, req *fraudv1.AssessRequest) (*fraudv1.AssessResponse, error) {
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		ts = s.now()
	}

	money, err := domain.NewMoney(req.Amount, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid money: %w", err)
	}

	coord, _ := domain.NewCoordinate(req.Lat, req.Lng)
	pm, err := domain.NewPaymentMethod(req.PaymentMethod)
	if err != nil {
		return nil, fmt.Errorf("invalid payment method: %w", err)
	}

	tx, err := domain.NewTransaction(domain.TransactionInput{
		ID:            req.TransactionId,
		Amount:        money,
		SenderID:      req.SenderId,
		ReceiverID:    req.ReceiverId,
		DeviceID:      req.DeviceId,
		IP:            req.Ip,
		Location:      coord,
		Timestamp:     ts,
		PaymentMethod: pm,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	assessCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	assessment, err := s.assessor.Assess(assessCtx, tx)
	if err != nil {
		return nil, fmt.Errorf("assessment failed: %w", err)
	}

	// Atomic save
	if err := s.saveAtomically(ctx, assessment); err != nil {
		return nil, fmt.Errorf("persist failed: %w", err)
	}

	var reasons []string
	for _, rr := range assessment.RuleResults {
		if rr.Triggered {
			reasons = append(reasons, rr.Reason)
		}
	}

	return &fraudv1.AssessResponse{
		TransactionId: assessment.TransactionID,
		Decision:      string(assessment.Decision),
		RiskScore:     int32(assessment.RiskScore.Value),
		Reasons:       reasons,
	}, nil
}

func (s *FraudServer) saveAtomically(ctx context.Context, assessment domain.FraudAssessment) error {
	dbTx, err := s.uow.Begin(ctx)
	if err != nil {
		return err
	}

	if err := s.assessments.SaveWithTx(ctx, dbTx, assessment); err != nil {
		s.uow.Rollback(dbTx)
		return err
	}

	for _, event := range assessment.Events() {
		if err := s.outbox.SaveWithinTx(ctx, dbTx, event); err != nil {
			s.uow.Rollback(dbTx)
			return err
		}
	}

	return s.uow.Commit(dbTx)
}
