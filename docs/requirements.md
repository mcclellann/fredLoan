# FredLoan Ledger Requirements & Specification

## 1. Project Overview
FredLoan is a high-precision personal loan ledger system designed to handle the core lifecycle of a loan, including disbursement, daily interest accrual, monthly interest application, and payment processing.

## 2. Business Requirements

### 2.1 Loan Management
- **Customer Linking:** Every loan must be linked to an external system via a unique `CustomerKey`.
- **Pricing:**
    - **Base Rate:** A standard interest rate (APR) is defined for the product.
    - **Risk Variance:** Support for risk-based pricing where individual customers receive a positive or negative variance to the base rate.
    - **Effective Rate:** The effective interest rate is the sum of the base rate and the variance.
- **Statement Cycles:**
    - Each loan is assigned a `StatementCycleDay` between the 1st and 28th.
    - An algorithm distributes these days randomly during loan creation to balance system load across the month.

### 2.2 Interest Calculation & Application
- **Precision:** All financial calculations (Principal, Balance, Interest) MUST use arbitrary-precision decimal math. Binary floating-point math is strictly prohibited.
- **Accrual:** Interest is calculated daily based on the current balance: `Daily Interest = Balance * (Effective APR / 365)`.
- **Application:** Accrued interest is not added to the principal balance daily. It is applied (capitalized) once per month on the loan's specific `StatementCycleDay`.
- **Idempotency:** The system must ensure daily interest is only accrued once per calendar day, even if the batch process is triggered multiple times.

### 2.3 Payments
- **Balance Reduction:** Payments immediately reduce the principal balance.
- **Loan Closure:** If a payment reduces the balance to zero or less, the loan status must automatically transition to `closed`.

## 3. Technical Specifications

### 3.1 Data Persistence
- **Engine:** SQLite 3.
- **Schema Integrity:**
    - Use `TEXT` types for all decimal values to prevent precision loss in the database layer.
    - Foreign key constraints must be enforced between Loans and Transactions.
    - WAL (Write-Ahead Logging) mode enabled for improved concurrency.

### 3.2 API Layer
- **Protocol:** RESTful JSON API.
- **Endpoints:**
    - `POST /loans`: Create a new loan.
    - `GET /loans`: List all loans.
    - `GET /loans/{id}`: Retrieve specific loan details.
    - `PUT /loans/{id}`: Update loan parameters.
    - `DELETE /loans/{id}`: Remove a loan (cascading delete for transactions).
    - `POST /loans/{id}/payments`: Record a payment.

### 3.3 Batch Processing
- **Simulation:** For development, the "daily" batch process runs on a high-frequency ticker (e.g., every 10 seconds), but adheres to calendar-day logic for accruals.
- **Routine Tasks:**
    1. Scan for all `active` loans.
    2. Calculate and update `AccruedInterest`.
    3. If `today == StatementCycleDay`, apply `AccruedInterest` to `Balance` and generate an interest transaction.

## 4. Safety & Standards
- **Concurrency:** Uses SQLite WAL mode and Go's standard library `database/sql` for safe concurrent access.
- **Accuracy:** Employs `shopspring/decimal` for all arithmetic.
- **Traceability:** Every major balance change (Disbursement, Interest Application, Payment) must generate a corresponding entry in the `transactions` table.
