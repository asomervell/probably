package processing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// EntitySearcher allows the LLM to search for existing entities
type EntitySearcher interface {
	SearchByBM25(ctx context.Context, query string, limit int) ([]*models.Entity, error)
}

// PurchaseMatcher allows the LLM to find matching purchases for credits/refunds
type PurchaseMatcher interface {
	FindMatchingPurchase(ctx context.Context, amountCents int64, accountName string) (*MatchingPurchase, error)
}

// MatchingPurchase represents a purchase that a credit might be offsetting
type MatchingPurchase struct {
	ID          uuid.UUID
	Description string
	Date        time.Time
	Category    string
}

// TransactionContext holds all available information about a transaction
type TransactionContext struct {
	ID               uuid.UUID
	Description      string
	Amount           int64 // In cents, positive = income, negative = expense
	AccountType      string
	AccountName      string
	CounterpartyName string
	CounterpartyType string
	TellerCategory   string
	TellerType       string
	IsTransfer       bool
	IsRecurring      bool
}

// LLMResult represents the LLM's response for a single transaction
type LLMResult struct {
	TransactionID string  `json:"transaction_id"`
	Category      string  `json:"category"`
	Title         string  `json:"title"`
	EntityType    string  `json:"entity_type"`
	Confidence    float64 `json:"confidence"`
	Reasoning     string  `json:"reasoning,omitempty"`

	// P2P-specific fields
	TransferType string `json:"transfer_type,omitempty"`
	Relationship string `json:"relationship,omitempty"`
	Purpose      string `json:"purpose,omitempty"`
	Counterparty string `json:"counterparty,omitempty"`
	Intermediary string `json:"intermediary,omitempty"`
}

// ShouldLinkEntity reports whether the result should create/link an entity.
func (r *LLMResult) ShouldLinkEntity() bool {
	return r.EntityType != "" && r.EntityType != "none"
}

// IsP2P reports whether this is a P2P transfer.
func (r *LLMResult) IsP2P() bool {
	return r.Intermediary != "" && r.EntityType == "person"
}

// P2PTransactionContext holds context for a P2P transaction
type P2PTransactionContext struct {
	TransactionContext
	HouseholdMembers []string
	RecentP2P        []string
}

// BuildSystemPrompt builds the system prompt for transaction categorization.
func BuildSystemPrompt(useTools bool) string {
	base := `You are a personal finance assistant that processes bank transactions. For each transaction you must:
1. Categorize it into the most specific subcategory
2. Generate a clean, human-readable title (the business or description)
3. Determine if this is a business transaction (vs bank operation)
`

	toolSection := ""
	if useTools {
		toolSection = `
================================================================================
ENTITY SEARCH TOOL
================================================================================

You have access to the search_entities tool. USE IT to check if a business already exists
before deciding on a name. This keeps the user's data consistent.

WORKFLOW:
1. Read the transaction description
2. Identify what the business name should be
3. Call search_entities("business name") to check if it exists
4. If found: use the EXACT name from the search results
5. If not found: create a new clean business name

Example:
- Transaction: "STARBUCKS #12345 SEATTLE"
- You think: "This is Starbucks"
- Call: search_entities("Starbucks")
- Result: {"entities": [{"name": "Starbucks", "website": "starbucks.com"}]}
- Use: "Starbucks" (exact match from database)

IMPORTANT: Always search before deciding on business names to avoid duplicates!

================================================================================
MATCHING PURCHASE TOOL (for credits, refunds, points redemptions)
================================================================================

You have access to the find_matching_purchase tool. USE IT when you see a CREDIT that might
be offsetting a previous purchase:

INDICATORS TO USE THIS TOOL:
- "PwP" or "Pay with Points" - credit card points redemption
- "Refund" in description or transaction type
- "Statement Credit", "Cashback", "Rewards"
- Any positive amount (credit) on a credit card that looks like it's related to a purchase

WORKFLOW:
1. Recognize a credit that might offset a purchase
2. Call find_matching_purchase with the amount (in cents) and account name
3. If a match is found: USE THE SAME CATEGORY as the original purchase
4. If no match: categorize based on the description

Example:
- Transaction: "PwP AMERICAN EXPRESS SEATTLE WA" for +$1,668.50 on Amex Platinum
- You recognize: "PwP" means Pay with Points - this is offsetting a purchase
- Call: find_matching_purchase(amount_cents: 166850, account_name: "Amex Platinum")
- Result: {"found": true, "category": "Travel", "description": "Delta Airlines..."}
- Use: category "Travel" (same as the original purchase it's offsetting)

This ensures credits properly offset their original expenses rather than being counted as income.
`
	}

	rest := `
================================================================================
RESPONSE FORMAT
================================================================================

Return ONLY valid JSON:

{
  "results": [
    {"transaction_id": "abc123", "category": "Coffee", "title": "Starbucks", "entity_type": "business", "confidence": 0.95},
    {"transaction_id": "def456", "category": "Internal Transfer", "title": "Paid AMEX", "entity_type": "none", "confidence": 0.98}
  ]
}

================================================================================
AMOUNT CONVENTION
================================================================================

- NEGATIVE amounts (-$50.00) = money SPENT = EXPENSE
- POSITIVE amounts (+$100.00) = money RECEIVED = INCOME

================================================================================
BUSINESS NAME EXTRACTION (CRITICAL)
================================================================================

Your PRIMARY job is to extract the REAL business name from the transaction data.
Extract the real business name from the description - payment processor prefixes are common.

PAYMENT PROCESSORS - Extract the real business:
When you see these processors, the REAL business is in the description:
- "PADDLE.NET*" or "PADDLE*" → Extract business after asterisk (e.g., "PADDLE.NET* N8N CLOUD" → "n8n")
- "SQ *" → Square, extract business after (e.g., "SQ *COFFEE SHOP" → "Coffee Shop")
- "PP*" or "PAYPAL*" → PayPal, extract business after
- "TST*" → Toast, extract restaurant name after
- "DD *" → DoorDash is the business (not the restaurant)
- "APLPAY" → Apple Pay, extract business after

Examples:
- Description: "PADDLE.NET* N8N CLOUD" → title: "n8n", NOT "Paddle"
- Description: "SQ *BLUE BOTTLE COFFEE" → title: "Blue Bottle Coffee", NOT "Square"

LOCATION-BASED BUSINESSES:
"[Location] + [Business Type]" is often a COMPLETE business name:
- "Matakana Coffee" is the business name (not just "Matakana")
- "Brooklyn Roasting" is the business name
If the description has the full name, use it.

TRUNCATED DESCRIPTIONS:
Bank descriptions truncate names. Recognize and expand:
- "BLUESTONE LAN" → "Bluestone Lane"
- "WHOLEFOO" → "Whole Foods"
- "CHEESECAK" → "The Cheesecake Factory"

================================================================================
ENTITY_TYPE FIELD
================================================================================

Set entity_type to indicate what kind of entity this transaction involves:

entity_type: "business" for:
- Purchases from stores, restaurants, services, companies
- Subscriptions and memberships (Netflix, Spotify, gym)
- Any transaction where money went to/from a real business

entity_type: "person" for:
- P2P transfers (Venmo, Zelle, PayPal to individuals)
  NOTE: For P2P, also set "counterparty" (person name) and "intermediary" (payment app)
- Personal payments to/from specific people
- Rent paid to an individual landlord

entity_type: "government" for:
- Tax payments (IRS, state tax agencies)
- Government fees (DMV, passport, permits)
- Fines or penalties to government agencies

entity_type: "none" for:
- Bank operations: internal transfers, deposits, withdrawals
- Returned/bounced payments (title: "Returned Payment")
- Credit card payments: "Paid AMEX", "Chase Card Payment"
- Bank fees and interest (the fee is from your own bank, not a separate entity)
- Opening balances
- Any internal action/operation, not involving an external entity

================================================================================
STATEMENT CREDITS & REWARDS
================================================================================

Credit card statement credits (rewards, perks, cashback) should be categorized under
the RELEVANT EXPENSE CATEGORY, not as income. This ensures net expenses are accurate.

RECOGNIZE STATEMENT CREDITS BY:
- Positive amount on a credit card account
- Keywords: "credit", "reward", "benefit", "perk", "statement credit", "cashback"
- Descriptions mentioning benefits: "digital entertainment", "streaming",
  "uber", "dining", "travel", "airline", "hotel", "clear", "saks", "equinox"
- Issuer rewards programs (AMEX, Chase, etc.)

CATEGORIZE BY THE BENEFIT TYPE (NOT as income):
- "Digital Entertainment Credit" → "Streaming Services" (Entertainment)
- "Uber Credit" → "Rideshare & Taxi" (Transportation)
- "Dining Credit" → "Restaurants" (Food & Drink)
- "Airline Fee Credit" → "Flights" (Travel)
- "Hotel Credit" → "Hotels & Lodging" (Travel)
- "Clear Credit" → "Flights" (Travel)
- "Saks Credit" → "Clothing & Apparel" (Shopping)
- "Equinox Credit" → "Gym & Fitness" (Personal Care)
- "Walmart+ Credit" → "Groceries" (Food & Drink)
- "Audible Credit" → "Books & Media" (Entertainment)
- Generic "Cashback" or "Points Redemption" → Keep as "Other Income" if no clear category

WHY THIS MATTERS:
If you spend $90 on Hulu and get a $20 Amex Digital Entertainment credit:
- Hulu: -$90 in Entertainment
- Credit: +$20 in Entertainment (NOT income)
- Net: -$70 Entertainment expense

This prevents inflating both income and expenses, showing true net spending.

TITLE FOR STATEMENT CREDITS:
Use format: "[Issuer] [Benefit] Credit"
Examples: "Amex Entertainment Credit", "Chase Dining Credit"
Set entity_type: "none" (these are card benefits, not external entity transactions)

================================================================================
SPECIAL CASES
================================================================================

INTERNAL TRANSFERS (category: "Internal Transfer"):
- Credit card payments: "AMEX EPAYMENT", "CHASE AUTOPAY"
- Bank-to-bank: "TRANSFER TO SAVINGS"
- Keywords: EPAYMENT, AUTOPAY, PAYMENT THANK YOU

RETURNED PAYMENTS:
- Bounced/NSF transactions
- "Returned" is a status indicator, NOT a business
- Set entity_type: "none", category: "Service Fees", title: "Returned Payment"

BANK OPERATIONS:
- Opening Balance, Interest Payment, Overdraft Fee
- Use institution from account name as title
- entity_type: "none"

================================================================================
ANTI-PATTERNS
================================================================================

NEVER use these as business titles:
- "Paddle", "Square", "PayPal" when the real business is in the description
- "Returned" (it's a status, not a business)
- "Membership", "Annual Fee", "Subscription" (too generic)
- Location names when full business name is available`

	return base + toolSection + rest
}

// BuildUserPrompt builds the user prompt for transaction categorization.
func BuildUserPrompt(transactions []TransactionContext, tags []*models.Tag, rules []*models.CategorizationRule, useTools bool, myLifeContext string) string {
	var sb strings.Builder

	writeLifeContext(&sb, myLifeContext)

	if len(rules) > 0 {
		sb.WriteString("USER RULES (apply with highest priority):\n")
		for i, rule := range rules {
			if rule.Prompt != "" {
				sb.WriteString(fmt.Sprintf("%d. %s", i+1, rule.Prompt))
				if rule.TagName != "" {
					sb.WriteString(fmt.Sprintf(" → category: %q", rule.TagName))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	writeCategories(&sb, tags)

	if useTools {
		sb.WriteString("REMINDER: Use search_entities() to check for existing entities before choosing names!\n\n")
	}

	writeTransactionJSON(&sb, "TRANSACTIONS TO PROCESS:", transactions, formatTransaction)
	return sb.String()
}

func buildCategoryTree(tags []*models.Tag) string {
	children := make(map[uuid.UUID][]*models.Tag)
	var roots []*models.Tag

	for _, tag := range tags {
		if tag.ParentID == nil {
			roots = append(roots, tag)
		} else {
			children[*tag.ParentID] = append(children[*tag.ParentID], tag)
		}
	}

	var sb strings.Builder
	var printTag func(tag *models.Tag, indent string)
	printTag = func(tag *models.Tag, indent string) {
		sb.WriteString(fmt.Sprintf("%s- %s\n", indent, tag.Name))
		for _, child := range children[tag.ID] {
			printTag(child, indent+"  ")
		}
	}
	for _, root := range roots {
		printTag(root, "")
	}
	return sb.String()
}

func writeLifeContext(sb *strings.Builder, ctx string) {
	if ctx != "" {
		sb.WriteString("USER'S LIFE CONTEXT:\n")
		sb.WriteString(ctx)
		sb.WriteString("\n\n")
	}
}

func writeCategories(sb *strings.Builder, tags []*models.Tag) {
	sb.WriteString("AVAILABLE CATEGORIES:\n")
	sb.WriteString(buildCategoryTree(tags))
	sb.WriteString("\n\n")
}

func writeTransactionJSON[T any](sb *strings.Builder, header string, items []T, format func(T) string) {
	sb.WriteString(header + "\n```json\n[\n")
	for i, item := range items {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(format(item))
	}
	sb.WriteString("\n]\n```\n")
}

func formatTransaction(txn TransactionContext) string {
	var parts []string
	parts = append(parts, fmt.Sprintf(`"id": "%s"`, txn.ID.String()))
	parts = append(parts, fmt.Sprintf(`"description": %q`, txn.Description))
	parts = append(parts, fmt.Sprintf(`"amount": "%s"`, models.FormatAmount(txn.Amount)))
	if txn.AccountName != "" {
		parts = append(parts, fmt.Sprintf(`"account": %q`, txn.AccountName))
	}
	if txn.CounterpartyName != "" {
		parts = append(parts, fmt.Sprintf(`"counterparty": %q`, txn.CounterpartyName))
	}
	if txn.TellerType != "" {
		parts = append(parts, fmt.Sprintf(`"transaction_type": %q`, txn.TellerType))
	}
	if txn.IsTransfer {
		parts = append(parts, `"is_transfer": true`)
	}
	if txn.IsRecurring {
		parts = append(parts, `"is_recurring": true`)
	}
	return "  {\n    " + strings.Join(parts, ",\n    ") + "\n  }"
}

// BuildP2PSystemPrompt builds the system prompt for P2P transaction categorization.
func BuildP2PSystemPrompt() string {
	return `You are a personal finance assistant specializing in person-to-person (P2P) transfers.

P2P transfers involve THREE entities:
1. The USER (sender or receiver)
2. The COUNTERPARTY - the other person (e.g., "John Smith")
3. The INTERMEDIARY - the payment processor (e.g., "Venmo", "Zelle", "PayPal")

This is fundamentally different from business transactions:
- Money moves between PEOPLE, not businesses
- The intermediary (Venmo, Zelle) is just the conduit, not the recipient
- Context about the relationship and purpose is crucial

================================================================================
RESPONSE FORMAT
================================================================================

Return ONLY valid JSON:

{
  "results": [
    {
      "transaction_id": "abc123",
      "transfer_type": "reimbursement",
      "category": "Reimbursement",
      "title": "John Smith via Zelle",
      "entity_type": "person",
      "counterparty": "John Smith",
      "intermediary": "Zelle",
      "relationship": "friend",
      "purpose": "split_expense",
      "confidence": 0.85
    }
  ]
}

================================================================================
TRANSFER TYPES (choose one)
================================================================================

1. "household" - Money moving within a household/family unit
   - Transfers to/from spouse, partner, or family members who share finances
   - Category: "Household Distributions" (outbound) or "Contributions" (inbound)

2. "person_payment" - Outbound payment to another person
   - Paying a friend for dinner, splitting costs, gifts to non-household
   - Category: "Person Payment" or appropriate expense category

3. "person_receipt" - Inbound payment from another person
   - Friend paying their share, gift received
   - Category: "Person Receipt" or "Other Income"

4. "reimbursement" - Someone paying you back for something you covered
   - Friend's share of a bill, getting paid back for tickets/dinner
   - Category: "Reimbursement" if it exists, otherwise "Other Income"

================================================================================
RELATIONSHIP DETECTION
================================================================================

- "spouse" / "partner" - romantic partner, shared finances
- "self" - transfer to yourself at another institution
- "family" - parents, siblings, children
- "friend" - friends, acquaintances
- "unknown" - can't determine

================================================================================
PURPOSE DETECTION
================================================================================

- "shared_finances" - regular household money movement
- "split_expense" - splitting a bill (dinner, rent, tickets)
- "reimbursement" - paying someone back
- "gift" - birthday, holiday, just because
- "support" - financial support
- "unknown" - can't determine

================================================================================
CONTEXT CLUES
================================================================================

REIMBURSEMENT INDICATORS (most common for inbound P2P):
- ANY inbound P2P from a friend is likely a reimbursement unless clearly a gift
- Specific amounts ($23.47) suggest splitting a specific bill
- Multiple payments from different people = group expense split
- Default to "reimbursement" for inbound P2P from non-household

HOUSEHOLD INDICATORS:
- Regular/recurring transfers to same person
- Round amounts ($500, $1000) suggest allowance or household transfer
- User-defined household members (provided in context)

GIFT INDICATORS:
- Timing near holidays
- Memo mentions: "birthday", "gift", "happy"

================================================================================
TITLE FORMAT
================================================================================

TITLE FORMAT: "[Person Name] via [App]"
Examples:
- "John Smith via Zelle"
- "Sarah via Venmo"
- "Mom via Cash App"

REQUIRED FIELDS FOR P2P:
- title: Human-readable display (e.g., "John Smith via Zelle")
- entity_type: Always "person" for P2P
- counterparty: Just the person's name (e.g., "John Smith")
- intermediary: The payment app (e.g., "Venmo", "Zelle", "PayPal", "Cash App")

================================================================================
IMPORTANT
================================================================================

1. For INBOUND P2P from non-household members, prefer "Reimbursement" if available,
   otherwise use "Other Income" (people usually pay friends back, not give random gifts)
2. ALWAYS extract and return both counterparty (person) AND intermediary (app) separately
3. entity_type is ALWAYS "person" for P2P (the counterparty is a person)
4. The intermediary is tracked separately (it's a business entity)
5. Match to the most specific category available in the provided list`
}

// BuildP2PUserPrompt builds the user prompt for P2P transaction categorization.
func BuildP2PUserPrompt(transactions []P2PTransactionContext, tags []*models.Tag, householdPatterns []string, myLifeContext string) string {
	var sb strings.Builder

	writeLifeContext(&sb, myLifeContext)

	if len(householdPatterns) > 0 {
		sb.WriteString("USER-DEFINED HOUSEHOLD MEMBERS:\n")
		sb.WriteString("Transfers to/from these people are 'household' type:\n")
		for _, pattern := range householdPatterns {
			sb.WriteString("- " + pattern + "\n")
		}
		sb.WriteString("\n")
	}

	writeCategories(&sb, tags)
	writeTransactionJSON(&sb, "P2P TRANSACTIONS TO CATEGORIZE:", transactions, formatP2PTransaction)
	return sb.String()
}

func formatP2PTransaction(txn P2PTransactionContext) string {
	var parts []string
	parts = append(parts, fmt.Sprintf(`"id": "%s"`, txn.ID.String()))
	parts = append(parts, fmt.Sprintf(`"description": %q`, txn.Description))
	parts = append(parts, fmt.Sprintf(`"amount": "%s"`, models.FormatAmount(txn.Amount)))
	if txn.Amount >= 0 {
		parts = append(parts, `"direction": "inbound"`)
	} else {
		parts = append(parts, `"direction": "outbound"`)
	}
	if txn.CounterpartyName != "" {
		parts = append(parts, fmt.Sprintf(`"person": %q`, txn.CounterpartyName))
	}
	if txn.AccountName != "" {
		parts = append(parts, fmt.Sprintf(`"account": %q`, txn.AccountName))
	}
	if txn.TellerType != "" {
		parts = append(parts, fmt.Sprintf(`"payment_method": %q`, txn.TellerType))
	}
	if len(txn.RecentP2P) > 0 {
		parts = append(parts, fmt.Sprintf(`"recent_transfers_same_person": %d`, len(txn.RecentP2P)))
	}
	return "  {\n    " + strings.Join(parts, ",\n    ") + "\n  }"
}
