package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/mcclellann/fredLoan/pkg/models"
	"github.com/mcclellann/fredLoan/pkg/store"
	"github.com/shopspring/decimal"
)

func setupTestServer(t *testing.T) (*Server, string) {
	dbFile := "test_api_dec.db"
	os.Remove(dbFile)

	s, err := store.NewSQLiteStore(dbFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	return NewServer(s), dbFile
}

func TestAPI_CreateAndGetLoan(t *testing.T) {
	server, dbFile := setupTestServer(t)
	defer os.Remove(dbFile)
	defer server.storage.Close()

	router := mux.NewRouter()
	router.HandleFunc("/loans", server.createLoanHandler).Methods("POST")
	router.HandleFunc("/loans/{id}", server.getLoanHandler).Methods("GET")

	// Create Loan
	loanReq := map[string]interface{}{
		"customer_key":           "test_cust",
		"principal":              5000.0,
		"base_interest_rate":     0.08,
		"interest_rate_variance": 0.01,
	}
	body, _ := json.Marshal(loanReq)
	req := httptest.NewRequest("POST", "/loans", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}

	var createdLoan models.Loan
	json.Unmarshal(rr.Body.Bytes(), &createdLoan)

	expectedRate := decimal.NewFromFloat(0.09)
	if !createdLoan.InterestRate.Equal(expectedRate) {
		t.Errorf("Expected effective rate %s, got %s", expectedRate, createdLoan.InterestRate)
	}

	// Get Loan
	req = httptest.NewRequest("GET", "/loans/"+createdLoan.ID.String(), nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var fetchedLoan models.Loan
	json.Unmarshal(rr.Body.Bytes(), &fetchedLoan)

	if fetchedLoan.ID != createdLoan.ID {
		t.Errorf("Expected ID %s, got %s", createdLoan.ID, fetchedLoan.ID)
	}
}

func TestAPI_RecordPayment(t *testing.T) {
	server, dbFile := setupTestServer(t)
	defer os.Remove(dbFile)
	defer server.storage.Close()

	router := mux.NewRouter()
	router.HandleFunc("/loans", server.createLoanHandler).Methods("POST")
	router.HandleFunc("/loans/{id}/payments", server.recordPaymentHandler).Methods("POST")

	// Create Loan
	loanReq := map[string]interface{}{
		"customer_key":           "test_cust",
		"principal":              1000.0,
		"base_interest_rate":     0.10,
		"interest_rate_variance": 0.0,
	}
	body, _ := json.Marshal(loanReq)
	req := httptest.NewRequest("POST", "/loans", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	var createdLoan models.Loan
	json.Unmarshal(rr.Body.Bytes(), &createdLoan)

	// Record Payment
	paymentAmount := 200.0
	payReq := map[string]interface{}{
		"amount": paymentAmount,
	}
	body, _ = json.Marshal(payReq)
	req = httptest.NewRequest("POST", "/loans/"+createdLoan.ID.String()+"/payments", bytes.NewBuffer(body))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var tx models.Transaction
	json.Unmarshal(rr.Body.Bytes(), &tx)
	if !tx.Amount.Equal(decimal.NewFromFloat(paymentAmount)) {
		t.Errorf("Expected amount %f, got %s", paymentAmount, tx.Amount)
	}
}
