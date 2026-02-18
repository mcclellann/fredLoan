package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/mcclellann/fredLoan/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore manages the database connection and operations for SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore and initializes the database.
func NewSQLiteStore(dataSourceName string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// Manually enable foreign keys and WAL mode
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("could not initialize schema: %w", err)
	}
	log.Println("Database connection established and schema initialized.")
	return s, nil
}

// initSchema creates the database tables if they don't already exist and adds new columns if necessary.
// We use TEXT for decimal fields in SQLite to ensure no precision is lost.
func (s *SQLiteStore) initSchema() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS loans (
		id TEXT PRIMARY KEY,
		customer_key TEXT NOT NULL,
		principal TEXT NOT NULL,
		balance TEXT NOT NULL,
		interest_rate TEXT NOT NULL,
		base_interest_rate TEXT NOT NULL DEFAULT '0',
		interest_rate_variance TEXT NOT NULL DEFAULT '0',
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_interest_calculation_date DATETIME,
		statement_cycle_day INTEGER NOT NULL DEFAULT 1,
		accrued_interest TEXT NOT NULL DEFAULT '0'
	);
	CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		loan_id TEXT NOT NULL,
		amount TEXT NOT NULL,
		type TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY(loan_id) REFERENCES loans(id)
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration for columns that might have been REAL before
	// In a real scenario, we'd need more complex migrations if data existed.
	// For now, we'll just ensure the columns are present.
	columns := []string{
		"last_interest_calculation_date DATETIME",
		"statement_cycle_day INTEGER NOT NULL DEFAULT 1",
		"accrued_interest TEXT NOT NULL DEFAULT '0'",
		"base_interest_rate TEXT NOT NULL DEFAULT '0'",
		"interest_rate_variance TEXT NOT NULL DEFAULT '0'",
	}

	for _, col := range columns {
		_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE loans ADD COLUMN %s", col))
		if err != nil && !isDuplicateColumnError(err) {
			return fmt.Errorf("failed to add column %s: %w", col, err)
		}
	}

	return nil
}

// isDuplicateColumnError checks if the error indicates a duplicate column.
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "duplicate column name" || (len(err.Error()) > 21 && err.Error()[:21] == "duplicate column name")
}

// CreateLoan inserts a new loan into the database.
func (s *SQLiteStore) CreateLoan(loan *models.Loan) error {
	_, err := s.db.Exec(
		`INSERT INTO loans (id, customer_key, principal, balance, interest_rate, base_interest_rate, interest_rate_variance, status, created_at, updated_at, last_interest_calculation_date, statement_cycle_day, accrued_interest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		loan.ID.String(), loan.CustomerKey, loan.Principal, loan.Balance, loan.InterestRate, loan.BaseInterestRate, loan.InterestRateVariance, loan.Status, loan.CreatedAt, loan.UpdatedAt, loan.LastInterestCalculationDate, loan.StatementCycleDay, loan.AccruedInterest,
	)
	if err != nil {
		return fmt.Errorf("failed to create loan: %w", err)
	}
	return nil
}

// GetLoan retrieves a loan by its ID.
func (s *SQLiteStore) GetLoan(id uuid.UUID) (*models.Loan, error) {
	var loan models.Loan
	var created, updated time.Time
	var loanIDStr string
	var lastInterestCalcDate sql.NullTime

	row := s.db.QueryRow(`SELECT id, customer_key, principal, balance, interest_rate, base_interest_rate, interest_rate_variance, status, created_at, updated_at, last_interest_calculation_date, statement_cycle_day, accrued_interest FROM loans WHERE id = ?`, id.String())
	err := row.Scan(&loanIDStr, &loan.CustomerKey, &loan.Principal, &loan.Balance, &loan.InterestRate, &loan.BaseInterestRate, &loan.InterestRateVariance, &loan.Status, &created, &updated, &lastInterestCalcDate, &loan.StatementCycleDay, &loan.AccruedInterest)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("loan not found")
		}
		return nil, fmt.Errorf("failed to get loan: %w", err)
	}
	loan.ID = uuid.MustParse(loanIDStr)
	loan.CreatedAt = created
	loan.UpdatedAt = updated
	if lastInterestCalcDate.Valid {
		loan.LastInterestCalculationDate = &lastInterestCalcDate.Time
	}
	return &loan, nil
}

// UpdateLoan updates an existing loan in the database.
func (s *SQLiteStore) UpdateLoan(loan *models.Loan) error {
	result, err := s.db.Exec(
		`UPDATE loans SET customer_key = ?, principal = ?, balance = ?, interest_rate = ?, base_interest_rate = ?, interest_rate_variance = ?, status = ?, updated_at = ?, last_interest_calculation_date = ?, statement_cycle_day = ?, accrued_interest = ? WHERE id = ?`,
		loan.CustomerKey, loan.Principal, loan.Balance, loan.InterestRate, loan.BaseInterestRate, loan.InterestRateVariance, loan.Status, loan.UpdatedAt, loan.LastInterestCalculationDate, loan.StatementCycleDay, loan.AccruedInterest, loan.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update loan: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("loan not found")
	}
	return nil
}

// DeleteLoan removes a loan and its transactions from the database within a transaction.
func (s *SQLiteStore) DeleteLoan(id uuid.UUID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM transactions WHERE loan_id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete associated transactions: %w", err)
	}

	result, err := tx.Exec(`DELETE FROM loans WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete loan: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("loan not found")
	}

	return tx.Commit()
}

// GetAllLoans retrieves all loans.
func (s *SQLiteStore) GetAllLoans() ([]*models.Loan, error) {
	rows, err := s.db.Query(`SELECT id, customer_key, principal, balance, interest_rate, base_interest_rate, interest_rate_variance, status, created_at, updated_at, last_interest_calculation_date, statement_cycle_day, accrued_interest FROM loans`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all loans: %w", err)
	}
	defer rows.Close()

	return s.scanLoans(rows)
}

// GetAllActiveLoans retrieves all active loans.
func (s *SQLiteStore) GetAllActiveLoans() ([]*models.Loan, error) {
	rows, err := s.db.Query(`SELECT id, customer_key, principal, balance, interest_rate, base_interest_rate, interest_rate_variance, status, created_at, updated_at, last_interest_calculation_date, statement_cycle_day, accrued_interest FROM loans WHERE status = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all active loans: %w", err)
	}
	defer rows.Close()

	return s.scanLoans(rows)
}

func (s *SQLiteStore) scanLoans(rows *sql.Rows) ([]*models.Loan, error) {
	var loans []*models.Loan
	for rows.Next() {
		var loan models.Loan
		var created, updated time.Time
		var loanIDStr string
		var lastInterestCalcDate sql.NullTime
		if err := rows.Scan(&loanIDStr, &loan.CustomerKey, &loan.Principal, &loan.Balance, &loan.InterestRate, &loan.BaseInterestRate, &loan.InterestRateVariance, &loan.Status, &created, &updated, &lastInterestCalcDate, &loan.StatementCycleDay, &loan.AccruedInterest); err != nil {
			return nil, fmt.Errorf("failed to scan loan row: %w", err)
		}
		loan.ID = uuid.MustParse(loanIDStr)
		loan.CreatedAt = created
		loan.UpdatedAt = updated
		if lastInterestCalcDate.Valid {
			loan.LastInterestCalculationDate = &lastInterestCalcDate.Time
		}
		loans = append(loans, &loan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}
	return loans, nil
}

// CreateTransaction inserts a new transaction into the database.
func (s *SQLiteStore) CreateTransaction(transaction *models.Transaction) error {
	_, err := s.db.Exec(
		`INSERT INTO transactions (id, loan_id, amount, type, timestamp)
		VALUES (?, ?, ?, ?, ?)`,
		transaction.ID.String(), transaction.LoanID.String(), transaction.Amount, transaction.Type, transaction.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	return nil
}

// GetTransactionsForLoan retrieves all transactions for a given loan ID.
func (s *SQLiteStore) GetTransactionsForLoan(loanID uuid.UUID) ([]*models.Transaction, error) {
	rows, err := s.db.Query(`SELECT id, loan_id, amount, type, timestamp FROM transactions WHERE loan_id = ? ORDER BY timestamp ASC`, loanID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions for loan %s: %w", loanID, err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		var txIDStr, loanIDStr string
		var timestamp time.Time
		if err := rows.Scan(&txIDStr, &loanIDStr, &transaction.Amount, &transaction.Type, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		transaction.ID = uuid.MustParse(txIDStr)
		transaction.LoanID = uuid.MustParse(loanIDStr)
		transaction.Timestamp = timestamp
		transactions = append(transactions, &transaction)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for loan transactions: %w", err)
	}
	return transactions, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
