package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name      string
		amount    float64
		currency  string
		wantErr   bool
		wantMoney Money
	}{
		{"valid USD", 100.50, "USD", false, Money{Amount: 100.50, Currency: "USD"}},
		{"valid EUR", 0.01, "EUR", false, Money{Amount: 0.01, Currency: "EUR"}},
		{"valid GBP", 999999.99, "GBP", false, Money{Amount: 999999.99, Currency: "GBP"}},
		{"valid JPY", 1, "JPY", false, Money{Amount: 1, Currency: "JPY"}},
		{"valid CAD", 50, "CAD", false, Money{Amount: 50, Currency: "CAD"}},
		{"valid AUD", 50, "AUD", false, Money{Amount: 50, Currency: "AUD"}},
		{"valid CHF", 50, "CHF", false, Money{Amount: 50, Currency: "CHF"}},
		{"valid CNY", 50, "CNY", false, Money{Amount: 50, Currency: "CNY"}},
		{"valid INR", 50, "INR", false, Money{Amount: 50, Currency: "INR"}},
		{"valid BRL", 50, "BRL", false, Money{Amount: 50, Currency: "BRL"}},
		{"zero amount", 0, "USD", true, Money{}},
		{"negative amount", -10, "USD", true, Money{}},
		{"invalid currency", 100, "XYZ", true, Money{}},
		{"empty currency", 100, "", true, Money{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMoney(tt.amount, tt.currency)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMoney, got)
		})
	}
}
