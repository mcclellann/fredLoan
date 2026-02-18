# Session Context: FredLoan Ledger

## 1. System Architecture
- **Language:** Go 1.25.6
- **Persistence:** SQLite (WAL mode enabled, Foreign Keys enforced).
- **Math Engine:** `shopspring/decimal` (Arbitrary-precision for all money/rates).
- **API Framework:** `gorilla/mux` (RESTful JSON).

## 2. Component Breakdown
### Models (`pkg/models/models.go`)
- `Loan`: Core entity. Tracks `Principal`, `Balance`, `BaseInterestRate`, `InterestRateVariance`, `InterestRate` (effective), `StatementCycleDay`, and `AccruedInterest`.
- `Transaction`: Audit trail for `disbursement`, `payment`, and `interest`.

### Store (`pkg/store/sqlite.go`)
- Uses `TEXT` for decimals to prevent SQLite `REAL` precision loss.
- Implements `Storage` interface with transactional deletes.
- Schema includes `loans` and `transactions` tables.

### Ledger (`pkg/ledger/ledger.go`)
- **Daily Accrual:** Calculates `Balance * (Effective APR / 365)` and adds to `AccruedInterest`. Prevents duplicate accrual per calendar day.
- **Monthly Application:** Capitalizes `AccruedInterest` to `Balance` on the `StatementCycleDay`.
- **Pricing:** Implements `Base + Variance` logic.
- **Payments:** Deducts from balance and auto-closes loans when paid in full.

### API Layer (`cmd/api/main.go`)
- `POST /loans`: Create with risk-based pricing.
- `GET /loans`: List all.
- `GET /loans/{id}`: Fetch single.
- `PUT /loans/{id}`: Update parameters.
- `DELETE /loans/{id}`: Full deletion (with transactions).
- `POST /loans/{id}/payments`: Record payment.

## 3. Current State & Verification
- **Build Status:** Passing.
- **Test Coverage:**
    - `pkg/ledger`: Unit tests for interest logic and payments.
    - `pkg/store`: Integration tests for SQLite operations.
    - `cmd/api`: Integration tests for CRUD and Payment endpoints.
- **GitHub:** Initialized at [mcclellann/fredLoan](https://github.com/mcclellann/fredLoan).
- **Environment:** Resolved previous SQLite "disk I/O error" by simplifying connection strings.

## 4. Pending / Next Steps
- [ ] **Transactions API:** Add endpoint to fetch transaction history for a specific loan.
- [ ] **Robust Batching:** Currently running on a 10s ticker for simulation; needs a production-grade scheduler.
- [ ] **Rounding Strategy:** Define explicit rounding modes (e.g., Bankers' Rounding) for interest calculations.
- [ ] **Customer System Integration:** Implement the webhook/callback layer for the external customer system.
