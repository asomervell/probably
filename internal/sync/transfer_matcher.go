package sync

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var incomeRegexps = []*regexp.Regexp{
	regexp.MustCompile(`(?i)payroll`),
	regexp.MustCompile(`(?i)salary`),
	regexp.MustCompile(`(?i)direct\s+deposit`),
	regexp.MustCompile(`(?i)wages`),
	regexp.MustCompile(`(?i)bonus`),
	regexp.MustCompile(`(?i)commission`),
	regexp.MustCompile(`(?i)unemployment`),
	regexp.MustCompile(`(?i)tax\s+refund`),
	regexp.MustCompile(`(?i)irs`),
	regexp.MustCompile(`(?i)social\s+security`),
	regexp.MustCompile(`(?i)pension`),
	regexp.MustCompile(`(?i)dividend`),
	regexp.MustCompile(`(?i)interest\s+(payment|income)`),
}

var transferRegexps = []*regexp.Regexp{
	regexp.MustCompile(`(?i)transfer\s+(to|from)`),
	regexp.MustCompile(`(?i)payment\s+(to|from)`),
	regexp.MustCompile(`(?i)xfer`),
	regexp.MustCompile(`(?i)internal\s+transfer`),
	regexp.MustCompile(`(?i)mobile\s+transfer`),
	regexp.MustCompile(`(?i)online\s+transfer`),
}

// TransferMatcher handles automatic detection and matching of transfers between accounts
type TransferMatcher struct {
	pool           *pgxpool.Pool
	transactions   *models.TransactionStore
	accounts       *models.AccountStore
	pendingMatches *models.PendingMatchStore
}

// NewTransferMatcher creates a new TransferMatcher
func NewTransferMatcher(pool *pgxpool.Pool) *TransferMatcher {
	return &TransferMatcher{
		pool:           pool,
		transactions:   models.NewTransactionStore(pool),
		accounts:       models.NewAccountStore(pool),
		pendingMatches: models.NewPendingMatchStore(pool),
	}
}

// MatchResult represents the result of transfer matching
type MatchResult struct {
	IsMatch          bool
	IsHighConfidence bool
	ConfidenceScore  float64
	Reasons          []string
	CandidateID      uuid.UUID
}

// Confidence thresholds
const (
	HighConfidenceThreshold   = 0.85
	MediumConfidenceThreshold = 0.50
)

// ProcessNewTransaction analyzes a new transaction for potential transfer matches
// High-confidence matches are automatically linked, others are queued for review
func (m *TransferMatcher) ProcessNewTransaction(ctx context.Context, txn *models.Transaction, entry *models.Entry) error {
	// Skip if already marked as transfer
	if txn.IsTransfer {
		return nil
	}

	// Skip income/expense accounts - transfers are between asset/liability accounts
	account, err := m.accounts.GetByID(ctx, entry.AccountID)
	if err != nil {
		return err
	}
	if account.Type == models.AccountTypeIncome || account.Type == models.AccountTypeExpense || account.Type == models.AccountTypeEquity {
		return nil
	}

	// Find candidates within 3-day window
	candidates, err := m.transactions.FindTransferCandidates(ctx, txn, entry.AmountCents, entry.AccountID, 3)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		return nil
	}

	// Score each candidate
	var bestMatch *MatchResult
	var bestCandidate *models.TransferCandidate

	for _, candidate := range candidates {
		result := m.scoreMatch(ctx, txn, entry, candidate)
		if result.ConfidenceScore >= MediumConfidenceThreshold {
			if bestMatch == nil || result.ConfidenceScore > bestMatch.ConfidenceScore {
				bestMatch = result
				bestCandidate = candidate
			}
		}
	}

	if bestMatch == nil {
		return nil
	}

	// High confidence: auto-match
	if bestMatch.ConfidenceScore >= HighConfidenceThreshold {
		return m.transactions.SetTransferPair(ctx, txn.ID, bestCandidate.Transaction.ID)
	}

	// Medium confidence: queue for review
	// First check if match already exists
	exists, err := m.pendingMatches.Exists(ctx, txn.ID, bestCandidate.Transaction.ID)
	if err != nil || exists {
		return err
	}

	pendingMatch := &models.PendingTransferMatch{
		TransactionID:          txn.ID,
		CandidateTransactionID: bestCandidate.Transaction.ID,
		ConfidenceScore:        bestMatch.ConfidenceScore,
		MatchReasons:           bestMatch.Reasons,
		Status:                 models.MatchStatusPending,
	}

	return m.pendingMatches.Create(ctx, pendingMatch)
}

// scoreMatch calculates the confidence score for a potential transfer match
func (m *TransferMatcher) scoreMatch(ctx context.Context, txn *models.Transaction, entry *models.Entry, candidate *models.TransferCandidate) *MatchResult {
	result := &MatchResult{
		CandidateID: candidate.Transaction.ID,
		Reasons:     make([]string, 0),
	}

	var score float64

	// Base score: amounts match (already filtered in query)
	score += 0.30
	result.Reasons = append(result.Reasons, "Matching amounts")

	// Date proximity scoring
	daysDiff := absDays(txn.Date, candidate.Transaction.Date)
	if daysDiff == 0 {
		score += 0.25
		result.Reasons = append(result.Reasons, "Same date")
	} else if daysDiff == 1 {
		score += 0.15
		result.Reasons = append(result.Reasons, "1 day apart")
	} else if daysDiff <= 3 {
		score += 0.05
		result.Reasons = append(result.Reasons, "Within 3 days")
	}

	// Counterparty signals
	if strings.EqualFold(txn.CounterpartyName, "YOURSELF") {
		score += 0.30
		result.Reasons = append(result.Reasons, "Counterparty is YOURSELF")
	}
	if strings.EqualFold(candidate.Transaction.CounterpartyName, "YOURSELF") {
		score += 0.15
		result.Reasons = append(result.Reasons, "Candidate counterparty is YOURSELF")
	}

	// Negative patterns - descriptions that indicate this is NOT a transfer
	for _, re := range incomeRegexps {
		if re.MatchString(txn.Description) {
			score -= 0.50
			result.Reasons = append(result.Reasons, "Income keyword in description - not a transfer")
			break
		}
	}

	for _, re := range incomeRegexps {
		if re.MatchString(candidate.Transaction.Description) {
			score -= 0.50
			result.Reasons = append(result.Reasons, "Income keyword in candidate - not a transfer")
			break
		}
	}

	// Description pattern matching - positive transfer signals
	for _, re := range transferRegexps {
		if re.MatchString(txn.Description) {
			score += 0.10
			result.Reasons = append(result.Reasons, "Transfer keyword in description")
			break
		}
	}

	for _, re := range transferRegexps {
		if re.MatchString(candidate.Transaction.Description) {
			score += 0.05
			result.Reasons = append(result.Reasons, "Transfer keyword in candidate")
			break
		}
	}

	// Fetch accounts for additional scoring signals
	sourceAcc, _ := m.accounts.GetByID(ctx, entry.AccountID)
	destAcc, _ := m.accounts.GetByID(ctx, candidate.AccountID)

	// Last four digits matching - strong signal when descriptions reference account numbers
	// Check if candidate's description contains the source account's last four
	// e.g., "Payment to Chase card ending in 5331" where 5331 is the source account
	if sourceAcc != nil && sourceAcc.LastFour != "" {
		if containsLastFour(candidate.Transaction.Description, sourceAcc.LastFour) {
			score += 0.30
			result.Reasons = append(result.Reasons, "Candidate mentions source account number")
		}
	}

	// Check if txn's description contains the candidate/destination account's last four
	// e.g., "Transfer to account ending 1234" where 1234 is the destination
	if destAcc != nil && destAcc.LastFour != "" {
		if containsLastFour(txn.Description, destAcc.LastFour) {
			score += 0.30
			result.Reasons = append(result.Reasons, "Description mentions destination account number")
		}
	}

	// Teller type signals
	transferTypes := []string{"transfer", "wire", "ach"}
	for _, t := range transferTypes {
		if strings.EqualFold(txn.TellerType, t) {
			score += 0.15
			result.Reasons = append(result.Reasons, "Transaction type is "+t)
			break
		}
	}

	// Account type pairing and DIRECTION validation
	// For a valid transfer, the money flow direction must make sense:
	//
	// All transfers have OPPOSITE signs from the bank's reporting perspective:
	//
	// Asset <-> Asset: OPPOSITE signs (money leaves one, arrives at another)
	//   - Checking (-100) -> Savings (+100)  ✓
	//
	// Asset <-> Liability: OPPOSITE signs (from bank's perspective)
	//   - Credit card payment: Checking (-100), Credit Card (+100 payment received)  ✓
	//   - Cash advance: Checking (+100), Credit Card (-100 debt added)  ✓
	//
	// Liability <-> Liability: INVALID! Two credit cards being paid simultaneously
	//   are two SEPARATE payments from a checking account, NOT a transfer between cards.
	if sourceAcc != nil && destAcc != nil {
		isAssetToAsset := sourceAcc.Type == models.AccountTypeAsset && destAcc.Type == models.AccountTypeAsset

		isLiabilityToLiability := sourceAcc.Type == models.AccountTypeLiability && destAcc.Type == models.AccountTypeLiability

		isAssetLiabilityPair := (sourceAcc.Type == models.AccountTypeAsset && destAcc.Type == models.AccountTypeLiability) ||
			(sourceAcc.Type == models.AccountTypeLiability && destAcc.Type == models.AccountTypeAsset)

		if isAssetToAsset {
			// Asset<->Asset with opposite amounts is valid (money moves from one to another)
			score += 0.10
			result.Reasons = append(result.Reasons, "Valid asset-to-asset transfer")
		} else if isLiabilityToLiability {
			// Liability<->Liability is INVALID!
			// Two credit cards being paid down are two separate payments from checking,
			// NOT a transfer between the cards. Reject this match entirely.
			result.Reasons = append(result.Reasons, "Invalid: two liabilities cannot transfer to each other")
			result.ConfidenceScore = 0
			result.IsMatch = false
			result.IsHighConfidence = false
			return result
		} else if isAssetLiabilityPair {
			// Asset<->Liability is valid for credit card payments and cash advances
			// The bank reports these with opposite signs:
			//   - CC payment: asset negative (outflow), liability positive (payment received)
			//   - Cash advance: asset positive (inflow), liability negative (debt added)
			score += 0.15
			result.Reasons = append(result.Reasons, "Valid asset-liability transfer (credit card payment)")
		}
	}

	// Floor score at 0 (income patterns can push it negative)
	if score < 0 {
		score = 0
	}
	// Cap score at 1.0
	if score > 1.0 {
		score = 1.0
	}

	result.ConfidenceScore = score
	result.IsMatch = score >= MediumConfidenceThreshold
	result.IsHighConfidence = score >= HighConfidenceThreshold

	return result
}

// ConfirmMatch confirms a pending match and links the transactions
func (m *TransferMatcher) ConfirmMatch(ctx context.Context, matchID uuid.UUID) error {
	match, err := m.pendingMatches.GetByID(ctx, matchID)
	if err != nil {
		return err
	}

	// Link the transactions
	if err := m.transactions.SetTransferPair(ctx, match.TransactionID, match.CandidateTransactionID); err != nil {
		return err
	}

	// Update match status
	return m.pendingMatches.UpdateStatus(ctx, matchID, models.MatchStatusConfirmed)
}

// RejectMatch marks a pending match as rejected
func (m *TransferMatcher) RejectMatch(ctx context.Context, matchID uuid.UUID) error {
	return m.pendingMatches.UpdateStatus(ctx, matchID, models.MatchStatusRejected)
}

// ManualMatch allows manual linking of two transactions as a transfer
func (m *TransferMatcher) ManualMatch(ctx context.Context, txn1ID, txn2ID uuid.UUID) error {
	// Remove any pending matches involving these transactions
	if err := m.pendingMatches.DeleteByTransactionID(ctx, txn1ID); err != nil {
		slog.WarnContext(ctx, "failed to clear pending matches for txn1", "txn", txn1ID, "error", err)
	}
	if err := m.pendingMatches.DeleteByTransactionID(ctx, txn2ID); err != nil {
		slog.WarnContext(ctx, "failed to clear pending matches for txn2", "txn", txn2ID, "error", err)
	}

	// Link the transactions
	return m.transactions.SetTransferPair(ctx, txn1ID, txn2ID)
}

// UnlinkTransfer unlinks a transfer pair
func (m *TransferMatcher) UnlinkTransfer(ctx context.Context, txnID uuid.UUID) error {
	return m.transactions.UnlinkTransferPair(ctx, txnID)
}

// MatchAllForAccountWithStats runs transfer matching and returns statistics
// Returns (autoLinked, pendingCreated, error)
func (m *TransferMatcher) MatchAllForAccountWithStats(ctx context.Context, account *models.Account) (int, int, error) {
	var autoLinked, pendingCreated int

	// Get all transactions for this account that aren't already transfers
	transactions, err := m.transactions.GetUnmatchedByAccountID(ctx, account.ID)
	if err != nil {
		return 0, 0, err
	}

	slog.DebugContext(ctx, "transfer matching", "unmatched_count", len(transactions))

	for _, txn := range transactions {
		// Get the entry for this transaction/account
		entry, err := m.transactions.GetEntryForAccount(ctx, txn.ID, account.ID)
		if err != nil {
			continue
		}

		// Find inter-account transfer candidates
		candidates, err := m.transactions.FindTransferCandidates(ctx, txn, entry.AmountCents, entry.AccountID, 3)
		if err != nil {
			continue
		}

		if len(candidates) == 0 {
			continue
		}

		// Score each candidate
		var bestMatch *MatchResult
		var bestCandidate *models.TransferCandidate

		for _, candidate := range candidates {
			result := m.scoreMatch(ctx, txn, entry, candidate)
			if result.ConfidenceScore >= MediumConfidenceThreshold {
				if bestMatch == nil || result.ConfidenceScore > bestMatch.ConfidenceScore {
					bestMatch = result
					bestCandidate = candidate
				}
			}
		}

		if bestMatch == nil {
			continue
		}

		// High confidence: auto-match
		if bestMatch.ConfidenceScore >= HighConfidenceThreshold {
			if err := m.transactions.SetTransferPair(ctx, txn.ID, bestCandidate.Transaction.ID); err != nil {
				slog.WarnContext(ctx, "failed to auto-link transfer pair",
					"from", txn.ID, "to", bestCandidate.Transaction.ID, "error", err)
			} else {
				autoLinked++
				slog.DebugContext(ctx, "transfer auto-linked",
					"from", txn.Description,
					"to", bestCandidate.Transaction.Description,
					"confidence", bestMatch.ConfidenceScore,
					"reasons", strings.Join(bestMatch.Reasons, ", "))
			}
			continue
		}

		// Medium confidence: queue for review
		exists, err := m.pendingMatches.Exists(ctx, txn.ID, bestCandidate.Transaction.ID)
		if err != nil || exists {
			continue
		}

		pendingMatch := &models.PendingTransferMatch{
			TransactionID:          txn.ID,
			CandidateTransactionID: bestCandidate.Transaction.ID,
			ConfidenceScore:        bestMatch.ConfidenceScore,
			MatchReasons:           bestMatch.Reasons,
			Status:                 models.MatchStatusPending,
		}

		if err := m.pendingMatches.Create(ctx, pendingMatch); err != nil {
			slog.WarnContext(ctx, "failed to create pending transfer match",
				"from", txn.ID, "to", bestCandidate.Transaction.ID, "error", err)
		} else {
			pendingCreated++
			slog.DebugContext(ctx, "transfer pending review",
				"from", txn.Description,
				"to", bestCandidate.Transaction.Description,
				"confidence", bestMatch.ConfidenceScore)
		}
	}

	return autoLinked, pendingCreated, nil
}

// Helper function to calculate absolute days difference
func absDays(t1, t2 time.Time) int {
	diff := t1.Sub(t2)
	if diff < 0 {
		diff = -diff
	}
	return int(diff.Hours() / 24)
}

// containsLastFour checks if a description contains an account's last four digits
// in a meaningful context (after "ending", as part of account reference, masked, or at a word boundary).
func containsLastFour(description, lastFour string) bool {
	if lastFour == "" || len(lastFour) != 4 {
		return false
	}
	lf := regexp.QuoteMeta(strings.ToLower(lastFour))
	pattern := regexp.MustCompile(
		`ending\s*(in\s*)?\s*` + lf +
			`|(card|account|acct)\s*#?\s*` + lf +
			`|[x*]+\s*` + lf +
			`|\b` + lf + `\b`,
	)
	return pattern.MatchString(strings.ToLower(description))
}
