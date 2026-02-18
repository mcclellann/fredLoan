package models

import (
	"time"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Loan struct {
	ID                        uuid.UUID       `json:"id"`
	CustomerKey               string          `json:"customer_key"` // Link to external customer system
	Principal                 decimal.Decimal `json:"principal"`
	Balance                   decimal.Decimal `json:"balance"`
	BaseInterestRate          decimal.Decimal `json:"base_interest_rate"`     // Standard rate for the product
	InterestRateVariance      decimal.Decimal `json:"interest_rate_variance"` // Adjustment (positive or negative)
	InterestRate              decimal.Decimal `json:"interest_rate"`          // Resulting effective APR
	Status                    string          `json:"status"`                 // e.g., "active", "closed"
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	LastInterestCalculationDate *time.Time      `json:"last_interest_calculation_date,omitempty"` // To prevent duplicate daily calculations
	StatementCycleDay         int             `json:"statement_cycle_day"`                       // Day of the month (1-28) for statement generation and interest application
	AccruedInterest           decimal.Decimal `json:"accrued_interest"`                          // Interest accrued since last statement
}

type TransactionType string

const (
	TransactionTypeDisbursement TransactionType = "disbursement"
	TransactionTypePayment      TransactionType = "payment"
	TransactionTypeInterest     TransactionType = "interest"
)

type Transaction struct {
	ID        uuid.UUID       `json:"id"`
	LoanID    uuid.UUID       `json:"loan_id"`
	Amount    decimal.Decimal `json:"amount"`
	Type      TransactionType `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
}
