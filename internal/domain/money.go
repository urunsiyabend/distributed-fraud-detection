package domain

import "fmt"

var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "JPY": true,
	"CAD": true, "AUD": true, "CHF": true, "CNY": true,
	"INR": true, "BRL": true,
}

type Money struct {
	Amount   float64
	Currency string
}

func NewMoney(amount float64, currency string) (Money, error) {
	if amount <= 0 {
		return Money{}, fmt.Errorf("amount must be non-negative, got %f", amount)
	}
	if !validCurrencies[currency] {
		return Money{}, fmt.Errorf("invalid currency: %s", currency)
	}
	return Money{Amount: amount, Currency: currency}, nil
}
