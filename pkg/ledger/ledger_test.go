package ledger

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mcclellann/fredLoan/pkg/models"
	"github.com/shopspring/decimal"
)

// MockStore is a simple in-memory implementation of the Storage interface for testing.
type MockStore struct {
	loans        map[uuid.UUID]*models.Loan
	transactions []*models.Transaction
}

func NewMockStore() *MockStore {
	return &MockStore{
		loans:        make(map[uuid.UUID]*models.Loan),
		transactions: []*models.Transaction{},
	}
}

func (m *MockStore) CreateLoan(loan *models.Loan) error {
	m.loans[loan.ID] = loan
	return nil
}

func (m *MockStore) GetLoan(id uuid.UUID) (*models.Loan, error) {
	loan, ok := m.loans[id]
	if !ok {
		return nil, fmt.Errorf("loan not found")
	}
	return loan, nil
}

func (m *MockStore) UpdateLoan(loan *models.Loan) error {
	m.loans[loan.ID] = loan
	return nil
}

func (m *MockStore) DeleteLoan(id uuid.UUID) error {
	delete(m.loans, id)
	return nil
}

func (m *MockStore) GetAllLoans() ([]*models.Loan, error) {
	loans := []*models.Loan{}
	for _, l := range m.loans {
		loans = append(loans, l)
	}
	return loans, nil
}

func (m *MockStore) GetAllActiveLoans() ([]*models.Loan, error) {
	loans := []*models.Loan{}
	for _, l := range m.loans {
		if l.Status == "active" {
			loans = append(loans, l)
		}
	}
	return loans, nil
}

func (m *MockStore) CreateTransaction(tx *models.Transaction) error {
	m.transactions = append(m.transactions, tx)
	return nil
}

func (m *MockStore) GetTransactionsForLoan(loanID uuid.UUID) ([]*models.Transaction, error) {
	txs := []*models.Transaction{}
	for _, tx := range m.transactions {
		if tx.LoanID == loanID {
			txs = append(txs, tx)
		}
	}
	return txs, nil
}

func (m *MockStore) Close() error {
	return nil
}

func TestCreateLoan(t *testing.T) {
	store := NewMockStore()
	l := NewLedger(store)

	principal := decimal.NewFromFloat(1000.0)
	baseRate := decimal.NewFromFloat(0.12)
	variance := decimal.NewFromFloat(-0.02)
	expectedRate := decimal.NewFromFloat(0.10)

	loan, err := l.CreateLoan("cust123", principal, baseRate, variance)
	if err != nil {
		t.Fatalf("Failed to create loan: %v", err)
	}

	if !loan.Principal.Equal(principal) {
		t.Errorf("Expected principal %s, got %s", principal, loan.Principal)
	}

	if !loan.InterestRate.Equal(expectedRate) {
		t.Errorf("Expected effective interest rate %s, got %s", expectedRate, loan.InterestRate)
	}

	if len(store.transactions) != 1 {
		t.Errorf("Expected 1 transaction (disbursement), got %d", len(store.transactions))
	}
}

func TestCalculateDailyInterest(t *testing.T) {
	store := NewMockStore()
	l := NewLedger(store)

	principal := decimal.NewFromFloat(1000.0)
	baseRate := decimal.NewFromFloat(0.10)
	loan, _ := l.CreateLoan("cust123", principal, baseRate, decimal.Zero)

	// Run interest calculation
	l.CalculateDailyInterest()

	if loan.AccruedInterest.Equal(decimal.Zero) {
		t.Error("Expected accrued interest to be greater than 0")
	}

	expectedDaily := principal.Mul(baseRate.Div(decimal.NewFromInt(365)))
	if !loan.AccruedInterest.Equal(expectedDaily) {
		t.Errorf("Expected accrued interest %s, got %s", expectedDaily, loan.AccruedInterest)
	}

	// Run again on same day (should skip)
	prevAccrued := loan.AccruedInterest
	l.CalculateDailyInterest()
	if !loan.AccruedInterest.Equal(prevAccrued) {
		t.Error("Interest should not be calculated twice on the same day")
	}
}

func TestApplyMonthlyInterest(t *testing.T) {
	store := NewMockStore()
	l := NewLedger(store)

	accrued := decimal.NewFromFloat(5.0)
	loan, _ := l.CreateLoan("cust123", decimal.NewFromFloat(1000.0), decimal.NewFromFloat(0.10), decimal.Zero)
	loan.AccruedInterest = accrued
	loan.StatementCycleDay = time.Now().Day() // Set to today

	l.ApplyMonthlyInterest()

	expectedBalance := decimal.NewFromFloat(1005.0)
	if !loan.Balance.Equal(expectedBalance) {
		t.Errorf("Expected balance %s, got %s", expectedBalance, loan.Balance)
	}

	if !loan.AccruedInterest.Equal(decimal.Zero) {
		t.Errorf("Expected accrued interest to be reset to 0, got %s", loan.AccruedInterest)
	}

	// Check if transaction was created
	found := false
	for _, tx := range store.transactions {
		if tx.Type == models.TransactionTypeInterest && tx.Amount.Equal(accrued) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Monthly interest transaction not found")
	}
}

func TestRecordPayment(t *testing.T) {
	store := NewMockStore()
	l := NewLedger(store)

	loan, _ := l.CreateLoan("cust123", decimal.NewFromFloat(1000.0), decimal.NewFromFloat(0.10), decimal.Zero)

	payment := decimal.NewFromFloat(400.0)
	_, err := l.RecordPayment(loan.ID, payment)
	if err != nil {
		t.Fatalf("Failed to record payment: %v", err)
	}

	expectedBalance := decimal.NewFromFloat(600.0)
	if !loan.Balance.Equal(expectedBalance) {
		t.Errorf("Expected balance %s, got %s", expectedBalance, loan.Balance)
	}

	// Pay off the loan
	l.RecordPayment(loan.ID, expectedBalance)
	if loan.Status != "closed" {
		t.Errorf("Expected status 'closed', got %s", loan.Status)
	}
	if !loan.Balance.Equal(decimal.Zero) {
		t.Errorf("Expected balance 0, got %s", loan.Balance)
	}
}
