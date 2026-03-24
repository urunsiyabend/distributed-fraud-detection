package domain

import (
	"fmt"
	"time"
)

type TransactionStatus string

const (
	StatusCreated           TransactionStatus = "created"
	StatusPendingFraudCheck TransactionStatus = "pending_fraud_check"
	StatusApproved          TransactionStatus = "approved"
	StatusBlocked           TransactionStatus = "blocked"
	StatusReview            TransactionStatus = "review"
	StatusPendingMFA        TransactionStatus = "pending_mfa"
	StatusCompleted         TransactionStatus = "completed"
	StatusFailed            TransactionStatus = "failed"
)

type Money struct {
	Amount   float64
	Currency string
}

type Coordinate struct {
	Lat float64
	Lng float64
}

type PaymentMethod string

type Transaction struct {
	ID            string
	SenderID      string
	ReceiverID    string
	Amount        Money
	Status        TransactionStatus
	DeviceID      string
	IP            string
	Location      Coordinate
	PaymentMethod PaymentMethod
	FraudDecision *string
	FraudScore    *int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// State machine transitions
var validTransitions = map[TransactionStatus][]TransactionStatus{
	StatusCreated:           {StatusPendingFraudCheck, StatusFailed},
	StatusPendingFraudCheck: {StatusApproved, StatusBlocked, StatusReview},
	StatusApproved:          {StatusCompleted, StatusFailed},
	StatusBlocked:           {},
	StatusReview:            {StatusPendingMFA, StatusBlocked, StatusApproved},
	StatusPendingMFA:        {StatusApproved, StatusBlocked, StatusFailed},
	StatusCompleted:         {},
	StatusFailed:            {},
}

func NewTransaction(
	id, senderID, receiverID string,
	amount Money,
	deviceID, ip string,
	location Coordinate,
	paymentMethod PaymentMethod,
) (*Transaction, error) {
	if id == "" {
		return nil, fmt.Errorf("transaction ID required")
	}
	if senderID == "" || receiverID == "" {
		return nil, fmt.Errorf("sender and receiver required")
	}
	if senderID == receiverID {
		return nil, fmt.Errorf("sender and receiver must differ")
	}
	if amount.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	now := time.Now()
	return &Transaction{
		ID:            id,
		SenderID:      senderID,
		ReceiverID:    receiverID,
		Amount:        amount,
		Status:        StatusCreated,
		DeviceID:      deviceID,
		IP:            ip,
		Location:      location,
		PaymentMethod: paymentMethod,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (t *Transaction) TransitionTo(newStatus TransactionStatus) error {
	allowed := validTransitions[t.Status]
	for _, s := range allowed {
		if s == newStatus {
			t.Status = newStatus
			t.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", t.Status, newStatus)
}

func (t *Transaction) ApplyFraudDecision(decision string, score int) error {
	t.FraudDecision = &decision
	t.FraudScore = &score

	switch decision {
	case "approved":
		return t.TransitionTo(StatusApproved)
	case "blocked":
		return t.TransitionTo(StatusBlocked)
	case "review":
		return t.TransitionTo(StatusReview)
	default:
		return fmt.Errorf("unknown fraud decision: %s", decision)
	}
}
