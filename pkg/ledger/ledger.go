package ledger

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/mcclellann/fredLoan/pkg/models"
	"github.com/mcclellann/fredLoan/pkg/store"
	"github.com/shopspring/decimal"
)

const (
	minStatementDay = 1
	maxStatementDay = 28
)

var (
	daysInYear = decimal.NewFromInt(365)
)

// Ledger handles the business logic for loans and transactions.
type Ledger struct {
	storage store.Storage // Use the Storage interface
	randSrc rand.Source   // Random source for assigning statement cycle day
}

// NewLedger creates a new Ledger with a given Storage implementation.
func NewLedger(s store.Storage) *Ledger {
	return &Ledger{
		storage: s,
		randSrc: rand.NewSource(time.Now().UnixNano()), // Initialize with a changing seed
	}
}

// assignStatementCycleDay assigns a day of the month (1-28) for the statement cycle.
func (l *Ledger) assignStatementCycleDay() int {
	r := rand.New(l.randSrc)
	return r.Intn(maxStatementDay-minStatementDay+1) + minStatementDay
}

// CreateLoan initializes a new loan for a customer.
func (l *Ledger) CreateLoan(customerKey string, principal decimal.Decimal, baseRate decimal.Decimal, variance decimal.Decimal) (*models.Loan, error) {
	loan := &models.Loan{
		ID:                          uuid.New(),
		CustomerKey:                 customerKey,
		Principal:                   principal,
		Balance:                     principal,
		BaseInterestRate:            baseRate,
		InterestRateVariance:        variance,
		InterestRate:                baseRate.Add(variance), // Effective rate
		Status:                      "active",
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
		LastInterestCalculationDate: nil,                         // Initially nil
		StatementCycleDay:           l.assignStatementCycleDay(), // Assign statement cycle day
		AccruedInterest:             decimal.Zero,
	}

	if err := l.storage.CreateLoan(loan); err != nil {
		return nil, fmt.Errorf("failed to store loan: %w", err)
	}

	// Record disbursement
	transaction := models.Transaction{
		ID:        uuid.New(),
		LoanID:    loan.ID,
		Amount:    principal,
		Type:      models.TransactionTypeDisbursement,
		Timestamp: time.Now(),
	}
	if err := l.storage.CreateTransaction(&transaction); err != nil {
		return nil, fmt.Errorf("failed to store disbursement transaction: %w", err)
	}

	return loan, nil
}

// CalculateDailyInterest iterates through all active loans and accrues daily interest.
func (l *Ledger) CalculateDailyInterest() {
	loans, err := l.storage.GetAllActiveLoans()
	if err != nil {
		fmt.Printf("Error getting active loans for daily interest calculation: %v\n", err)
		return
	}

	today := time.Now().UTC().Truncate(24 * time.Hour) // Truncate to get just the date

	for _, loan := range loans {
		// Check if interest has already been calculated for today
		if loan.LastInterestCalculationDate != nil && loan.LastInterestCalculationDate.UTC().Truncate(24*time.Hour).Equal(today) {
			fmt.Printf("Daily interest for Loan %s already calculated for today. Skipping.\n", loan.ID)
			continue
		}

		// Daily interest = Balance * (APR / 365)
		dailyRate := loan.InterestRate.Div(daysInYear)
		interestAmount := loan.Balance.Mul(dailyRate)

		if interestAmount.GreaterThan(decimal.Zero) {
			loan.AccruedInterest = loan.AccruedInterest.Add(interestAmount)
			loan.UpdatedAt = time.Now()
			// Update LastInterestCalculationDate
			loan.LastInterestCalculationDate = &today

			if err := l.storage.UpdateLoan(loan); err != nil {
				fmt.Printf("Error updating loan %s during daily interest calculation: %v\n", loan.ID, err)
				continue
			}

			fmt.Printf("Accrued %s daily interest for Loan %s (Total Accrued: %s)\n", interestAmount.StringFixed(2), loan.ID, loan.AccruedInterest.StringFixed(2))
		}
	}
}

// ApplyMonthlyInterest checks if today is the statement cycle day for any loans
// and applies accrued interest to the balance.
func (l *Ledger) ApplyMonthlyInterest() {
	loans, err := l.storage.GetAllActiveLoans()
	if err != nil {
		fmt.Printf("Error getting active loans for monthly interest application: %v\n", err)
		return
	}

	todayDay := time.Now().Day()

	for _, loan := range loans {
		if loan.StatementCycleDay == todayDay {
			if loan.AccruedInterest.GreaterThan(decimal.Zero) {
				loan.Balance = loan.Balance.Add(loan.AccruedInterest)
				loan.UpdatedAt = time.Now()

				transaction := models.Transaction{
					ID:        uuid.New(),
					LoanID:    loan.ID,
					Amount:    loan.AccruedInterest,
					Type:      models.TransactionTypeInterest,
					Timestamp: time.Now(),
				}
				if err := l.storage.CreateTransaction(&transaction); err != nil {
					fmt.Printf("Error creating monthly interest transaction for loan %s: %v\n", loan.ID, err)
					continue
				}

				fmt.Printf("Applied %s accrued interest to Loan %s on statement day (New Balance: %s)\n", loan.AccruedInterest.StringFixed(2), loan.ID, loan.Balance.StringFixed(2))
				loan.AccruedInterest = decimal.Zero // Reset accrued interest after application

				if err := l.storage.UpdateLoan(loan); err != nil {
					fmt.Printf("Error updating loan %s after monthly interest application: %v\n", loan.ID, err)
					continue
				}
			} else {
				fmt.Printf("No accrued interest to apply for Loan %s on statement day.\n", loan.ID)
			}
		}
	}
}

// GetLoan retrieves a loan by its ID.
func (l *Ledger) GetLoan(id uuid.UUID) (*models.Loan, error) {
	return l.storage.GetLoan(id)
}

// GetAllLoans retrieves all loans.
func (l *Ledger) GetAllLoans() ([]*models.Loan, error) {
	return l.storage.GetAllLoans()
}

// UpdateLoan updates an existing loan.
func (l *Ledger) UpdateLoan(loan *models.Loan) error {
	loan.UpdatedAt = time.Now()
	return l.storage.UpdateLoan(loan)
}

// DeleteLoan deletes a loan.
func (l *Ledger) DeleteLoan(id uuid.UUID) error {
	return l.storage.DeleteLoan(id)
}

// RecordPayment processes a payment for a loan.
func (l *Ledger) RecordPayment(loanID uuid.UUID, amount decimal.Decimal) (*models.Transaction, error) {
	loan, err := l.storage.GetLoan(loanID)
	if err != nil {
		return nil, err
	}

	if loan.Status != "active" {
		return nil, fmt.Errorf("loan is not active")
	}

	loan.Balance = loan.Balance.Sub(amount)
	loan.UpdatedAt = time.Now()

	// If balance is 0 or negative, close the loan
	if loan.Balance.LessThanOrEqual(decimal.Zero) {
		loan.Status = "closed"
		loan.Balance = decimal.Zero // Ensure balance is not negative
	}

	if err := l.storage.UpdateLoan(loan); err != nil {
		return nil, fmt.Errorf("failed to update loan balance: %w", err)
	}

	transaction := &models.Transaction{
		ID:        uuid.New(),
		LoanID:    loan.ID,
		Amount:    amount,
		Type:      models.TransactionTypePayment,
		Timestamp: time.Now(),
	}

	if err := l.storage.CreateTransaction(transaction); err != nil {
		return nil, fmt.Errorf("failed to store payment transaction: %w", err)
	}

	return transaction, nil
}
