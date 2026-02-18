# FredLoan - Personal Loan Ledger

FredLoan is a high-precision personal loan ledger system built in Go. It provides a robust API for managing loans, recording payments, and automatically handling interest accruals and applications with financial-grade precision.

## Features

*   **Financial Precision:** Uses `shopspring/decimal` for all monetary calculations to avoid floating-point rounding errors.
*   **Risk-Based Pricing:** Supports standard product interest rates with per-customer variances (positive or negative).
*   **Monthly Statement Cycles:** Automatically assigns a statement cycle day (1st-28th) to new loans to distribute processing load.
*   **Batch Processing:** Includes a background worker that calculates interest daily (accrued) and applies it to the balance monthly on the statement cycle date.
*   **Full CRUD API:** Comprehensive endpoints for creating, retrieving, updating, and deleting loans.
*   **Payment Processing:** Dedicated endpoint for recording customer payments.
*   **SQLite Persistence:** Robust data storage using SQLite with WAL (Write-Ahead Logging) mode enabled for concurrency.
*   **Transactional Integrity:** Uses database transactions for critical operations like loan deletion to ensure data consistency.

## Prerequisites

*   **Go:** Version 1.21 or higher.
*   **GCC:** Required for the SQLite driver (CGO).
*   **Git:** To clone the repository.

## Installation

### 1. Clone the repository
```bash
git clone https://github.com/mcclellann/fredLoan.git
cd fredLoan
```

### 2. Download dependencies
```bash
go mod download
```

## Build and Run

### 1. Build the application
This will compile the API server into a binary named `fredloan`.
```bash
go build -o fredloan ./cmd/api
```

### 2. Run the server
```bash
./fredloan
```
The server will start on `http://localhost:8080`. A SQLite database file named `fredloan.db` will be created automatically in the root directory.

*Note: For testing purposes, the "daily" interest calculation is currently set to run every 10 seconds. You can change this in `cmd/api/main.go`.*

## API Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/loans` | List all loans |
| `POST` | `/loans` | Create a new loan |
| `GET` | `/loans/{id}` | Get details of a specific loan |
| `PUT` | `/loans/{id}` | Update an existing loan |
| `DELETE` | `/loans/{id}` | Delete a loan and its transactions |
| `POST` | `/loans/{id}/payments` | Record a payment for a loan |

### Example: Create a Loan
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "customer_key": "cust_123",
  "principal": "5000.00",
  "base_interest_rate": "0.12",
  "interest_rate_variance": "-0.02"
}' http://localhost:8080/loans
```

### Example: Record a Payment
```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "amount": "250.00"
}' http://localhost:8080/loans/{loan_id}/payments
```

## Testing

Run the full suite of unit and integration tests:
```bash
go test ./...
```

## Project Structure

*   `cmd/api/`: Application entry point and API handlers.
*   `pkg/ledger/`: Core business logic for interest calculation and payments.
*   `pkg/models/`: Data models for Loans and Transactions.
*   `pkg/store/`: Database persistence layer (SQLite).

## License
MIT
