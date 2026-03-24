package domain

import (
	"fmt"
	"time"
)

type Transaction struct {
	ID            string
	Amount        Money
	SenderID      string
	ReceiverID    string
	DeviceID      string
	IP            string
	Location      Coordinate
	Timestamp     time.Time
	PaymentMethod PaymentMethod
}

type TransactionInput struct {
	ID            string
	Amount        Money
	SenderID      string
	ReceiverID    string
	DeviceID      string
	IP            string
	Location      Coordinate
	Timestamp     time.Time
	PaymentMethod PaymentMethod
}

func NewTransaction(input TransactionInput) (Transaction, error) {
	if input.ID == "" {
		return Transaction{}, fmt.Errorf("transaction ID must not be empty")
	}
	if input.SenderID == "" {
		return Transaction{}, fmt.Errorf("sender ID must not be empty")
	}
	if input.ReceiverID == "" {
		return Transaction{}, fmt.Errorf("receiver ID must not be empty")
	}
	if input.SenderID == input.ReceiverID {
		return Transaction{}, fmt.Errorf("sender and receiver must be different")
	}
	if input.Timestamp.IsZero() {
		return Transaction{}, fmt.Errorf("timestamp must not be zero")
	}

	return Transaction{
		ID:            input.ID,
		Amount:        input.Amount,
		SenderID:      input.SenderID,
		ReceiverID:    input.ReceiverID,
		DeviceID:      input.DeviceID,
		IP:            input.IP,
		Location:      input.Location,
		Timestamp:     input.Timestamp,
		PaymentMethod: input.PaymentMethod,
	}, nil
}
