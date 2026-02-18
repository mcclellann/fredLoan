package store

import (
	"github.com/google/uuid"
	"github.com/mcclellann/fredLoan/pkg/models"
)

// Storage defines the interface for database operations related to loans and transactions.
type Storage interface {
	CreateLoan(loan *models.Loan) error
	GetLoan(id uuid.UUID) (*models.Loan, error)
	UpdateLoan(loan *models.Loan) error
	DeleteLoan(id uuid.UUID) error
	GetAllLoans() ([]*models.Loan, error)
	GetAllActiveLoans() ([]*models.Loan, error)

	CreateTransaction(transaction *models.Transaction) error
	GetTransactionsForLoan(loanID uuid.UUID) ([]*models.Transaction, error)

	Close() error
}
