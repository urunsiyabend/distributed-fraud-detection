package rules

import (
	"context"
	"errors"
	"testing"

	"github.com/urunsiyabend/distributed-fraud-detection/fraud-detection/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDeviceRepo struct {
	known bool
	err   error
}

func (m *mockDeviceRepo) IsKnownDevice(_ context.Context, _, _ string) (bool, error) {
	return m.known, m.err
}

func TestDeviceRule_Evaluate(t *testing.T) {
	tests := []struct {
		name          string
		deviceID      string
		known         bool
		repoErr       error
		wantTriggered bool
		wantScore     int
		wantErr       bool
	}{
		{"known device — not triggered", "dev-1", true, nil, false, 0, false},
		{"unknown device — triggered", "dev-1", false, nil, true, 35, false},
		{"empty device ID — triggered with missing score", "", false, nil, true, 30, false},
		{"repo error — returns error", "dev-1", false, errors.New("db down"), false, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockDeviceRepo{known: tt.known, err: tt.repoErr}
			rule := NewDeviceRule(repo, 30, 35, 15)

			assert.Equal(t, "device", rule.Name())
			assert.Equal(t, 15, rule.FallbackScore())

			tx := domain.Transaction{
				SenderID: "user-1",
				DeviceID: tt.deviceID,
			}

			result, err := rule.Evaluate(context.Background(), tx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTriggered, result.Triggered)
			if tt.wantTriggered {
				assert.Equal(t, tt.wantScore, result.Score)
				assert.NotEmpty(t, result.Reason)
			}
		})
	}
}
