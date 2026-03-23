package api

import "time"

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type AssessTransactionRequest struct {
	ID            string  `json:"id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	DeviceID      string  `json:"device_id"`
	IP            string  `json:"ip"`
	Location      LatLng  `json:"location"`
	Timestamp     time.Time `json:"timestamp"`
	PaymentMethod string  `json:"payment_method"`
}

type AssessTransactionResponse struct {
	TransactionID string   `json:"transaction_id"`
	Decision      string   `json:"decision"`
	RiskScore     int      `json:"risk_score"`
	Reasons       []string `json:"reasons"`
	AssessedAt    time.Time `json:"assessed_at"`
	FastPath      bool     `json:"fast_path"`
}

type ErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

func (r AssessTransactionRequest) Validate() []string {
	var errs []string

	if r.ID == "" {
		errs = append(errs, "id is required")
	}
	if r.Amount <= 0 {
		errs = append(errs, "amount must be positive")
	}
	if r.Currency == "" {
		errs = append(errs, "currency is required")
	}
	if r.SenderID == "" {
		errs = append(errs, "sender_id is required")
	}
	if r.ReceiverID == "" {
		errs = append(errs, "receiver_id is required")
	}
	if r.SenderID != "" && r.ReceiverID != "" && r.SenderID == r.ReceiverID {
		errs = append(errs, "sender_id and receiver_id must be different")
	}
	if r.Timestamp.IsZero() {
		errs = append(errs, "timestamp is required")
	}

	return errs
}
