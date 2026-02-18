package store

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mcclellann/fredLoan/pkg/models"
	"github.com/shopspring/decimal"
)

func TestSQLiteStore_CreateAndGetLoan(t *testing.T) {
	dbFile := "test_store_dec.db"
	os.Remove(dbFile)
	defer os.Remove(dbFile)

	s, err := NewSQLiteStore(dbFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	loan := &models.Loan{
		ID:                   uuid.New(),
		CustomerKey:          "cust_test",
		Principal:            decimal.NewFromFloat(2000.0),
		Balance:              decimal.NewFromFloat(2000.0),
		BaseInterestRate:     decimal.NewFromFloat(0.05),
		InterestRateVariance: decimal.Zero,
		InterestRate:         decimal.NewFromFloat(0.05),
		Status:               "active",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		StatementCycleDay:    15,
		AccruedInterest:      decimal.Zero,
	}

	err = s.CreateLoan(loan)
	if err != nil {
		t.Fatalf("Failed to create loan: %v", err)
	}

	fetched, err := s.GetLoan(loan.ID)
	if err != nil {
		t.Fatalf("Failed to get loan: %v", err)
	}

	if fetched.CustomerKey != loan.CustomerKey {
		t.Errorf("Expected CustomerKey %s, got %s", loan.CustomerKey, fetched.CustomerKey)
	}

	if !fetched.Principal.Equal(loan.Principal) {
		t.Errorf("Expected Principal %s, got %s", loan.Principal, fetched.Principal)
	}

	if fetched.StatementCycleDay != 15 {
		t.Errorf("Expected StatementCycleDay 15, got %d", fetched.StatementCycleDay)
	}
}

func TestSQLiteStore_Transactions(t *testing.T) {
	dbFile := "test_tx_dec.db"
	os.Remove(dbFile)
	defer os.Remove(dbFile)

	s, err := NewSQLiteStore(dbFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	loanID := uuid.New()
	// Must create loan first due to foreign key
	s.CreateLoan(&models.Loan{
		ID:                   loanID,
		CustomerKey:          "test",
		Principal:            decimal.NewFromInt(100),
		Balance:              decimal.NewFromInt(100),
		BaseInterestRate:     decimal.NewFromFloat(0.1),
		InterestRateVariance: decimal.Zero,
		InterestRate:         decimal.NewFromFloat(0.1),
		Status:               "active",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		StatementCycleDay:    1,
		AccruedInterest:      decimal.Zero,
	})

	amount := decimal.NewFromFloat(50.0)
	tx := &models.Transaction{
		ID:        uuid.New(),
		LoanID:    loanID,
		Amount:    amount,
		Type:      models.TransactionTypePayment,
		Timestamp: time.Now(),
	}

	err = s.CreateTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	txs, err := s.GetTransactionsForLoan(loanID)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(txs) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(txs))
	}

	if !txs[0].Amount.Equal(amount) {
		t.Errorf("Expected amount %s, got %s", amount, txs[0].Amount)
	}
}
