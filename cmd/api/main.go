package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/mcclellann/fredLoan/pkg/ledger"
	"github.com/mcclellann/fredLoan/pkg/models"
	"github.com/mcclellann/fredLoan/pkg/store"
	"github.com/shopspring/decimal"
)

// Server holds the ledger instance.
type Server struct {
	ledger *ledger.Ledger
	storage store.Storage // Keep a reference to the storage to close it
}

func NewServer(s store.Storage) *Server {
	return &Server{
		ledger: ledger.NewLedger(s),
		storage: s,
	}
}

func (s *Server) createLoanHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerKey          string          `json:"customer_key"`
		Principal            decimal.Decimal `json:"principal"`
		BaseInterestRate     decimal.Decimal `json:"base_interest_rate"`
		InterestRateVariance decimal.Decimal `json:"interest_rate_variance"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	loan, err := s.ledger.CreateLoan(req.CustomerKey, req.Principal, req.BaseInterestRate, req.InterestRateVariance)
	if err != nil {
		log.Printf("Error creating loan: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to create loan: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loan)
}

func (s *Server) getLoanHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	loanID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid loan ID", http.StatusBadRequest)
		return
	}

	loan, err := s.ledger.GetLoan(loanID)
	if err != nil {
		if err.Error() == "loan not found" {
			http.Error(w, "Loan not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loan)
}

func (s *Server) listLoansHandler(w http.ResponseWriter, r *http.Request) {
	loans, err := s.ledger.GetAllLoans()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loans)
}

func (s *Server) updateLoanHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	loanID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid loan ID", http.StatusBadRequest)
		return
	}

	var loan models.Loan
	if err := json.NewDecoder(r.Body).Decode(&loan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	loan.ID = loanID // Ensure ID from URL is used

	if err := s.ledger.UpdateLoan(&loan); err != nil {
		if err.Error() == "loan not found" {
			http.Error(w, "Loan not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loan)
}

func (s *Server) deleteLoanHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	loanID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid loan ID", http.StatusBadRequest)
		return
	}

	if err := s.ledger.DeleteLoan(loanID); err != nil {
		if err.Error() == "loan not found" {
			http.Error(w, "Loan not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) recordPaymentHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	loanID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid loan ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Amount decimal.Decimal `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Amount.LessThanOrEqual(decimal.Zero) {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	tx, err := s.ledger.RecordPayment(loanID, req.Amount)
	if err != nil {
		if err.Error() == "loan not found" {
			http.Error(w, "Loan not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(tx)
}

func main() {
	// Initialize SQLite Store
	sqliteStore, err := store.NewSQLiteStore("fredloan.db")
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer sqliteStore.Close()

	server := NewServer(sqliteStore)
	router := mux.NewRouter()

	router.HandleFunc("/loans", server.listLoansHandler).Methods("GET")
	router.HandleFunc("/loans", server.createLoanHandler).Methods("POST")
	router.HandleFunc("/loans/{id}", server.getLoanHandler).Methods("GET")
	router.HandleFunc("/loans/{id}", server.updateLoanHandler).Methods("PUT")
	router.HandleFunc("/loans/{id}", server.deleteLoanHandler).Methods("DELETE")
	router.HandleFunc("/loans/{id}/payments", server.recordPaymentHandler).Methods("POST")

	// Start a goroutine for daily and monthly batch processing
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Simulate daily for testing
		defer ticker.Stop()

		for range ticker.C {
			log.Println("Running daily interest calculation...")
			server.ledger.CalculateDailyInterest()
			log.Println("Daily interest calculation complete.")

			log.Println("Running monthly interest application...")
			server.ledger.ApplyMonthlyInterest()
			log.Println("Monthly interest application complete.")
		}
	}()

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
