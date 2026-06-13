package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

func TestAPITransfers_Pending(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-pending")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	// Create two transactions that look like a transfer
	txn1 := env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Transfer to Savings", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -10000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Transfer from Checking", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 10000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -10000, Currency: "USD"},
	})

	// Create pending match
	pendingMatch := &models.PendingTransferMatch{
		ID:                     uuid.New(),
		TransactionID:          txn1.ID,
		CandidateTransactionID: txn2.ID,
		ConfidenceScore:        0.95,
		MatchReasons:           []string{"same_amount", "same_date"},
		Status:                 models.MatchStatusPending,
	}
	_, _ = env.Pool.Exec(context.Background(), `
		INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id, confidence_score, match_reasons, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, pendingMatch.ID, pendingMatch.TransactionID, pendingMatch.CandidateTransactionID,
		pendingMatch.ConfidenceScore, pendingMatch.MatchReasons, pendingMatch.Status)

	// List pending transfers
	resp := client.Get("/api/v1/transfers/pending")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct {
			ID              string   `json:"id"`
			ConfidenceScore float64  `json:"confidence_score"`
			MatchReasons    []string `json:"match_reasons"`
			Status          string   `json:"status"`
			Transaction     struct {
				Description string `json:"description"`
			} `json:"transaction"`
			Candidate struct {
				Description string `json:"description"`
			} `json:"candidate"`
		} `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("Expected 1 pending match, got %d", len(result.Data))
		return
	}

	if result.Data[0].ConfidenceScore != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Data[0].ConfidenceScore)
	}
	if result.Data[0].Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", result.Data[0].Status)
	}
}

func TestAPITransfers_ManualMatch(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-manual")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	// Create accounts
	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	// Create two transactions
	txn1 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Send to Savings", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Receive from Checking", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Manually match them
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn1.ID.String(),
		"transaction_id_2": txn2.ID.String(),
	})
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Matched      bool `json:"matched"`
		Transaction1 struct {
			ID         string `json:"id"`
			IsTransfer bool   `json:"is_transfer"`
		} `json:"transaction1"`
		Transaction2 struct {
			ID         string `json:"id"`
			IsTransfer bool   `json:"is_transfer"`
		} `json:"transaction2"`
	}
	ParseJSON(t, resp, &result)

	if !result.Matched {
		t.Error("Expected matched to be true")
	}
	if !result.Transaction1.IsTransfer {
		t.Error("Expected transaction1 to be marked as transfer")
	}
	if !result.Transaction2.IsTransfer {
		t.Error("Expected transaction2 to be marked as transfer")
	}
}

func TestAPITransfers_ManualMatchValidation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-manual-validation")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Test", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Try to match with itself
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn.ID.String(),
		"transaction_id_2": txn.ID.String(),
	})
	AssertStatus(t, resp, http.StatusBadRequest)

	// Try to match with non-existent transaction
	resp = client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn.ID.String(),
		"transaction_id_2": uuid.New().String(),
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPITransfers_Confirm(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-confirm")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Send", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Receive", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Create pending match
	matchID := uuid.New()
	_, _ = env.Pool.Exec(context.Background(), `
		INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id, confidence_score, match_reasons, status, created_at)
		VALUES ($1, $2, $3, 0.90, ARRAY['same_amount'], 'pending', NOW())
	`, matchID, txn1.ID, txn2.ID)

	// Confirm the match
	resp := client.Post("/api/v1/transfers/"+matchID.String()+"/confirm", nil)
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Confirmed    bool `json:"confirmed"`
		Transaction1 struct {
			IsTransfer bool `json:"is_transfer"`
		} `json:"transaction1"`
		Transaction2 struct {
			IsTransfer bool `json:"is_transfer"`
		} `json:"transaction2"`
	}
	ParseJSON(t, resp, &result)

	if !result.Confirmed {
		t.Error("Expected confirmed to be true")
	}
	if !result.Transaction1.IsTransfer || !result.Transaction2.IsTransfer {
		t.Error("Expected both transactions to be marked as transfers")
	}
}

func TestAPITransfers_Reject(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-reject")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Transaction 1", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Transaction 2", []*models.Entry{
		{AccountID: checking.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: expense.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Create pending match
	matchID := uuid.New()
	_, _ = env.Pool.Exec(context.Background(), `
		INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id, confidence_score, match_reasons, status, created_at)
		VALUES ($1, $2, $3, 0.50, ARRAY['same_amount'], 'pending', NOW())
	`, matchID, txn1.ID, txn2.ID)

	// Reject the match
	resp := client.Post("/api/v1/transfers/"+matchID.String()+"/reject", nil)
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Rejected bool `json:"rejected"`
	}
	ParseJSON(t, resp, &result)

	if !result.Rejected {
		t.Error("Expected rejected to be true")
	}

	// Verify match status changed
	var status string
	_ = env.Pool.QueryRow(context.Background(), `SELECT status FROM pending_transfer_matches WHERE id = $1`, matchID).Scan(&status)
	if status != "rejected" {
		t.Errorf("Expected status 'rejected', got %s", status)
	}
}

func TestAPITransfers_Unlink(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-unlink")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Uncategorized Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Uncategorized Expenses", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Send", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Receive", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -5000, Currency: "USD"},
	})

	// First, manually match them
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn1.ID.String(),
		"transaction_id_2": txn2.ID.String(),
	})
	AssertStatus(t, resp, http.StatusOK)

	// Then unlink
	resp = client.Post("/api/v1/transfers/"+txn1.ID.String()+"/unlink", nil)
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Unlinked    bool `json:"unlinked"`
		Transaction struct {
			IsTransfer bool `json:"is_transfer"`
		} `json:"transaction"`
	}
	ParseJSON(t, resp, &result)

	if !result.Unlinked {
		t.Error("Expected unlinked to be true")
	}
	if result.Transaction.IsTransfer {
		t.Error("Expected transaction to no longer be a transfer")
	}

	// Verify both transactions are unlinked
	resp = client.Get("/api/v1/transactions/" + txn2.ID.String())
	AssertStatus(t, resp, http.StatusOK)

	var txnResult struct {
		IsTransfer bool `json:"is_transfer"`
	}
	ParseJSON(t, resp, &txnResult)

	if txnResult.IsTransfer {
		t.Error("Expected paired transaction to also be unlinked")
	}
}

func TestAPITransfers_UnlinkNonTransfer(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-unlink-nontransfer")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	// Create a regular transaction (not a transfer)
	txn := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Regular Purchase", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Try to unlink (should fail)
	resp := client.Post("/api/v1/transfers/"+txn.ID.String()+"/unlink", nil)
	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestAPITransfers_CrossUserIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	user1 := env.CreateTestUser("transfers-iso-1")
	defer env.CleanupTestUser(user1)
	user2 := env.CreateTestUser("transfers-iso-2")
	defer env.CleanupTestUser(user2)

	client1 := env.NewAPIClient(user1)
	client2 := env.NewAPIClient(user2)

	// User 1 creates accounts and transactions
	checking1 := env.CreateAccount(user1.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings1 := env.CreateAccount(user1.Ledger.ID, "Savings", models.AccountTypeAsset)
	income1 := env.CreateAccount(user1.Ledger.ID, "Income", models.AccountTypeIncome)
	expense1 := env.CreateAccount(user1.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(user1.Ledger.ID, timeNow(), "User1 Txn1", []*models.Entry{
		{AccountID: expense1.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking1.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(user1.Ledger.ID, timeNow(), "User1 Txn2", []*models.Entry{
		{AccountID: savings1.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income1.ID, AmountCents: -5000, Currency: "USD"},
	})

	// Create pending match for user1
	matchID := uuid.New()
	_, _ = env.Pool.Exec(context.Background(), `
		INSERT INTO pending_transfer_matches (id, transaction_id, candidate_transaction_id, confidence_score, match_reasons, status, created_at)
		VALUES ($1, $2, $3, 0.90, ARRAY['same_amount'], 'pending', NOW())
	`, matchID, txn1.ID, txn2.ID)

	// User 2 should see empty pending list
	resp := client2.Get("/api/v1/transfers/pending")
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Data []struct{ ID string } `json:"data"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("User 2 should see 0 pending matches, got %d", len(result.Data))
	}

	// User 2 should not be able to confirm user1's match
	resp = client2.Post("/api/v1/transfers/"+matchID.String()+"/confirm", nil)
	AssertStatus(t, resp, http.StatusNotFound)

	// User 1 should be able to see and confirm
	resp = client1.Get("/api/v1/transfers/pending")
	AssertStatus(t, resp, http.StatusOK)
	ParseJSON(t, resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("User 1 should see 1 pending match, got %d", len(result.Data))
	}
}

func TestAPITransfers_MatchAlreadyTransfer(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	tu := env.CreateTestUser("transfers-already-transfer")
	defer env.CleanupTestUser(tu)
	client := env.NewAPIClient(tu)

	checking := env.CreateAccount(tu.Ledger.ID, "Checking", models.AccountTypeAsset)
	savings := env.CreateAccount(tu.Ledger.ID, "Savings", models.AccountTypeAsset)
	income := env.CreateAccount(tu.Ledger.ID, "Income", models.AccountTypeIncome)
	expense := env.CreateAccount(tu.Ledger.ID, "Expense", models.AccountTypeExpense)

	txn1 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Txn1", []*models.Entry{
		{AccountID: expense.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: checking.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn2 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Txn2", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -5000, Currency: "USD"},
	})
	txn3 := env.CreateTransaction(tu.Ledger.ID, timeNow(), "Txn3", []*models.Entry{
		{AccountID: savings.ID, AmountCents: 5000, Currency: "USD"},
		{AccountID: income.ID, AmountCents: -5000, Currency: "USD"},
	})

	// First match txn1 and txn2
	resp := client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn1.ID.String(),
		"transaction_id_2": txn2.ID.String(),
	})
	AssertStatus(t, resp, http.StatusOK)

	// Try to match txn1 (already a transfer) with txn3
	resp = client.Post("/api/v1/transfers/match", map[string]any{
		"transaction_id_1": txn1.ID.String(),
		"transaction_id_2": txn3.ID.String(),
	})
	AssertStatus(t, resp, http.StatusBadRequest)
}
