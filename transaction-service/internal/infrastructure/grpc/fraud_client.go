package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/urunsiyabend/distributed-fraud-detection/transaction-service/internal/domain"
	fraudv1 "github.com/urunsiyabend/distributed-fraud-detection/proto/fraud/v1"

	"github.com/sony/gobreaker/v2"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type FraudClient struct {
	client  fraudv1.FraudServiceClient
	breaker *gobreaker.CircuitBreaker[*fraudv1.AssessResponse]
}

func NewFraudClient(addr string) (*FraudClient, error) {
	conn, err := gogrpc.NewClient(addr,
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", addr, err)
	}

	cb := gobreaker.NewCircuitBreaker[*fraudv1.AssessResponse](gobreaker.Settings{
		Name:        "fraud-service",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %s: %s → %s", name, from.String(), to.String())
		},
	})

	return &FraudClient{
		client:  fraudv1.NewFraudServiceClient(conn),
		breaker: cb,
	}, nil
}

func (f *FraudClient) Check(ctx context.Context, tx *domain.Transaction) (string, int, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer cancel()

	resp, err := f.breaker.Execute(func() (*fraudv1.AssessResponse, error) {
		return f.client.Assess(ctx, &fraudv1.AssessRequest{
			TransactionId: tx.ID,
			Amount:        tx.Amount.Amount,
			Currency:      tx.Amount.Currency,
			SenderId:      tx.SenderID,
			ReceiverId:    tx.ReceiverID,
			DeviceId:      tx.DeviceID,
			Ip:            tx.IP,
			Lat:           tx.Location.Lat,
			Lng:           tx.Location.Lng,
			PaymentMethod: string(tx.PaymentMethod),
			Timestamp:     tx.CreatedAt.Format(time.RFC3339),
		})
	})
	if err != nil {
		// CB open or timeout → default to review + MFA
		return "review", 0, []string{"fraud service unavailable, requiring MFA"}, nil
	}

	return resp.Decision, int(resp.RiskScore), resp.Reasons, nil
}
