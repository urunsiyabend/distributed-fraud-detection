package fraudv1

type AssessRequest struct {
	TransactionId string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	SenderId      string  `json:"sender_id"`
	ReceiverId    string  `json:"receiver_id"`
	DeviceId      string  `json:"device_id"`
	Ip            string  `json:"ip"`
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	PaymentMethod string  `json:"payment_method"`
	Timestamp     string  `json:"timestamp"`
}

type AssessResponse struct {
	TransactionId string   `json:"transaction_id"`
	Decision      string   `json:"decision"`
	RiskScore     int32    `json:"risk_score"`
	Reasons       []string `json:"reasons"`
}
