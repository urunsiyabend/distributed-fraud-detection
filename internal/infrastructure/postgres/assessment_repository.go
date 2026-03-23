package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"distributed-fraud-detection/internal/domain"
)

type AssessmentRepository struct {
	db *sql.DB
}

func NewAssessmentRepository(db *sql.DB) *AssessmentRepository {
	return &AssessmentRepository{db: db}
}

func (r *AssessmentRepository) SaveWithTx(ctx context.Context, tx domain.TxHandle, assessment domain.FraudAssessment) error {
	ruleResultsJSON, err := json.Marshal(assessment.RuleResults)
	if err != nil {
		return fmt.Errorf("marshaling rule results: %w", err)
	}

	query := `
		INSERT INTO assessments (transaction_id, decision, risk_score, rule_results)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (transaction_id) DO UPDATE SET
			decision = EXCLUDED.decision,
			risk_score = EXCLUDED.risk_score,
			rule_results = EXCLUDED.rule_results`

	_, err = tx.ExecContext(ctx, query,
		assessment.TransactionID,
		string(assessment.Decision),
		assessment.RiskScore.Value,
		ruleResultsJSON,
	)
	if err != nil {
		return fmt.Errorf("saving assessment %s: %w", assessment.TransactionID, err)
	}

	return nil
}
