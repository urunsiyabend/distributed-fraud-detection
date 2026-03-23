package domain

import "fmt"

type PaymentMethod string

const (
	PaymentMethodCard   PaymentMethod = "card"
	PaymentMethodWire   PaymentMethod = "wire"
	PaymentMethodCrypto PaymentMethod = "crypto"
)

func NewPaymentMethod(value string) (PaymentMethod, error) {
	pm := PaymentMethod(value)
	switch pm {
	case PaymentMethodCard, PaymentMethodWire, PaymentMethodCrypto:
		return pm, nil
	default:
		return "", fmt.Errorf("invalid payment method: %s (must be card, wire, or crypto)", value)
	}
}
