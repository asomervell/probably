package categorize

import (
	"context"
	"log/slog"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CategoryDef defines a category with optional subcategories
type CategoryDef struct {
	Name          string
	Color         string
	Subcategories []SubcategoryDef
}

// SubcategoryDef defines a subcategory with aliases for matching
type SubcategoryDef struct {
	Name    string
	Aliases []string // Keywords that help identify this category
}

// DefaultTaxonomy returns the hierarchical category structure
// Based on Plaid's personal finance category taxonomy
var DefaultTaxonomy = []CategoryDef{
	// Income
	{
		Name:  "Income",
		Color: "#22c55e", // Green
		Subcategories: []SubcategoryDef{
			{Name: "Salary & Wages", Aliases: []string{"payroll", "salary", "wage", "paycheck", "direct deposit"}},
			{Name: "Dividends", Aliases: []string{"dividend", "stock income"}},
			{Name: "Interest", Aliases: []string{"interest earned", "bank interest", "savings interest"}},
			{Name: "Tax Refund", Aliases: []string{"tax refund", "irs", "state refund"}},
			{Name: "Retirement & Pension", Aliases: []string{"pension", "retirement", "401k", "ira"}},
			{Name: "Reimbursement", Aliases: []string{"reimbursement", "payback", "paid back", "owes", "split"}},
			{Name: "Hobby Income", Aliases: []string{"winnings", "poker", "gambling", "craft sale", "side hustle", "sold"}},
			{Name: "Other Income", Aliases: []string{"income", "deposit", "credit"}},
		},
	},
	// Food & Drink
	{
		Name:  "Food & Drink",
		Color: "#f97316", // Orange
		Subcategories: []SubcategoryDef{
			{Name: "Groceries", Aliases: []string{"grocery", "supermarket", "whole foods", "trader joe", "safeway", "kroger", "walmart", "costco", "target"}},
			{Name: "Restaurants", Aliases: []string{"restaurant", "dining", "dine", "grill", "bistro", "cafe", "eatery"}},
			{Name: "Coffee", Aliases: []string{"coffee", "starbucks", "dunkin", "peet", "cafe", "espresso"}},
			{Name: "Fast Food", Aliases: []string{"mcdonald", "burger", "wendy", "taco bell", "chipotle", "subway", "chick-fil-a", "kfc", "pizza"}},
			{Name: "Alcohol & Bars", Aliases: []string{"bar", "pub", "liquor", "wine", "beer", "brewery", "tavern"}},
			{Name: "Food Delivery", Aliases: []string{"doordash", "uber eats", "grubhub", "postmates", "instacart"}},
		},
	},
	// Transportation
	{
		Name:  "Transportation",
		Color: "#8b5cf6", // Purple
		Subcategories: []SubcategoryDef{
			{Name: "Gas & Fuel", Aliases: []string{"gas", "fuel", "shell", "chevron", "exxon", "mobil", "bp", "arco", "76"}},
			{Name: "Public Transit", Aliases: []string{"transit", "metro", "subway", "bus", "rail", "train", "mta", "bart"}},
			{Name: "Rideshare & Taxi", Aliases: []string{"uber", "lyft", "taxi", "cab", "rideshare"}},
			{Name: "Parking", Aliases: []string{"parking", "meter", "garage"}},
			{Name: "Tolls", Aliases: []string{"toll", "fastrak", "ezpass", "sunpass"}},
			{Name: "Auto Maintenance", Aliases: []string{"mechanic", "auto repair", "oil change", "tire", "jiffy lube", "autozone"}},
		},
	},
	// Shopping
	{
		Name:  "Shopping",
		Color: "#ec4899", // Pink
		Subcategories: []SubcategoryDef{
			{Name: "Clothing & Apparel", Aliases: []string{"clothing", "apparel", "fashion", "nike", "adidas", "gap", "h&m", "zara", "nordstrom"}},
			{Name: "Electronics", Aliases: []string{"electronics", "apple", "best buy", "computer", "phone", "tech"}},
			{Name: "Home & Garden", Aliases: []string{"home depot", "lowe's", "ikea", "furniture", "garden", "bed bath"}},
			{Name: "General Merchandise", Aliases: []string{"walmart", "target", "costco", "amazon", "merchandise"}},
			{Name: "Online Shopping", Aliases: []string{"amazon", "ebay", "etsy", "shopify", "online"}},
			{Name: "Pet Supplies", Aliases: []string{"pet", "petco", "petsmart", "chewy", "vet"}},
		},
	},
	// Entertainment
	{
		Name:  "Entertainment",
		Color: "#f43f5e", // Rose
		Subcategories: []SubcategoryDef{
			{Name: "Streaming Services", Aliases: []string{"netflix", "spotify", "hulu", "disney", "hbo", "apple tv", "youtube", "amazon prime"}},
			{Name: "Movies & Events", Aliases: []string{"movie", "cinema", "amc", "regal", "theater", "concert", "ticketmaster", "eventbrite"}},
			{Name: "Gaming", Aliases: []string{"game", "steam", "playstation", "xbox", "nintendo", "twitch"}},
			{Name: "Books & Media", Aliases: []string{"book", "kindle", "audible", "barnes", "library"}},
			{Name: "Hobbies", Aliases: []string{"hobby", "craft", "michaels", "joann"}},
		},
	},
	// Travel
	{
		Name:  "Travel",
		Color: "#06b6d4", // Cyan
		Subcategories: []SubcategoryDef{
			{Name: "Flights", Aliases: []string{"airline", "flight", "united", "delta", "american", "southwest", "jetblue", "air"}},
			{Name: "Hotels & Lodging", Aliases: []string{"hotel", "motel", "airbnb", "vrbo", "marriott", "hilton", "hyatt", "inn"}},
			{Name: "Rental Cars", Aliases: []string{"rental car", "hertz", "enterprise", "avis", "budget", "national"}},
			{Name: "Vacation", Aliases: []string{"vacation", "resort", "cruise", "travel"}},
		},
	},
	// Healthcare
	{
		Name:  "Healthcare",
		Color: "#14b8a6", // Teal
		Subcategories: []SubcategoryDef{
			{Name: "Doctor & Medical", Aliases: []string{"doctor", "medical", "clinic", "hospital", "physician", "health"}},
			{Name: "Pharmacy", Aliases: []string{"pharmacy", "cvs", "walgreens", "rite aid", "prescription", "rx"}},
			{Name: "Dental", Aliases: []string{"dental", "dentist", "orthodont"}},
			{Name: "Vision", Aliases: []string{"vision", "eye", "optical", "lenscrafters", "glasses", "contacts"}},
			{Name: "Mental Health", Aliases: []string{"therapy", "counseling", "psycholog", "psychiatr", "mental"}},
		},
	},
	// Personal Care
	{
		Name:  "Personal Care",
		Color: "#a855f7", // Violet
		Subcategories: []SubcategoryDef{
			{Name: "Gym & Fitness", Aliases: []string{"gym", "fitness", "yoga", "pilates", "crossfit", "planet fitness", "equinox", "orangetheory"}},
			{Name: "Hair & Beauty", Aliases: []string{"salon", "haircut", "spa", "nail", "beauty", "barber", "massage"}},
			{Name: "Laundry & Dry Cleaning", Aliases: []string{"laundry", "dry clean", "cleaners"}},
		},
	},
	// Home & Utilities
	{
		Name:  "Home & Utilities",
		Color: "#eab308", // Yellow
		Subcategories: []SubcategoryDef{
			{Name: "Rent", Aliases: []string{"rent", "lease", "landlord", "apartment"}},
			{Name: "Mortgage", Aliases: []string{"mortgage", "home loan"}},
			{Name: "Electric & Gas", Aliases: []string{"electric", "gas", "utility", "pge", "con ed", "duke energy"}},
			{Name: "Water", Aliases: []string{"water", "sewer", "municipal"}},
			{Name: "Internet & Cable", Aliases: []string{"internet", "cable", "comcast", "xfinity", "spectrum", "att", "verizon fios"}},
			{Name: "Phone", Aliases: []string{"phone", "mobile", "wireless", "verizon", "at&t", "t-mobile", "sprint"}},
			{Name: "Home Insurance", Aliases: []string{"home insurance", "renters insurance", "property insurance"}},
			{Name: "Home Repairs", Aliases: []string{"repair", "maintenance", "plumber", "electrician", "hvac", "handyman"}},
		},
	},
	// Services
	{
		Name:  "Services",
		Color: "#64748b", // Slate
		Subcategories: []SubcategoryDef{
			{Name: "Education", Aliases: []string{"education", "tuition", "school", "university", "college", "course", "udemy", "coursera"}},
			{Name: "Childcare", Aliases: []string{"childcare", "daycare", "babysit", "nanny"}},
			{Name: "Legal", Aliases: []string{"legal", "attorney", "lawyer", "law firm"}},
			{Name: "Accounting", Aliases: []string{"accounting", "accountant", "cpa", "tax prep", "turbotax", "h&r block"}},
			{Name: "Insurance", Aliases: []string{"insurance", "geico", "state farm", "allstate", "progressive", "premium"}},
			{Name: "Subscriptions", Aliases: []string{"subscription", "membership", "monthly", "annual"}},
		},
	},
	// Bank Fees
	{
		Name:  "Bank Fees",
		Color: "#ef4444", // Red
		Subcategories: []SubcategoryDef{
			{Name: "ATM Fees", Aliases: []string{"atm fee", "atm charge", "withdrawal fee"}},
			{Name: "Overdraft Fees", Aliases: []string{"overdraft", "nsf", "insufficient funds"}},
			{Name: "Interest Charges", Aliases: []string{"interest charge", "finance charge", "apr"}},
			{Name: "Service Fees", Aliases: []string{"service fee", "monthly fee", "maintenance fee", "account fee", "returned payment", "returned autopay", "return item", "nsf return", "bounced"}},
			{Name: "Wire & Transfer Fees", Aliases: []string{"wire fee", "transfer fee", "foreign transaction"}},
		},
	},
	// Gifts & Donations
	{
		Name:  "Gifts & Donations",
		Color: "#10b981", // Emerald
		Subcategories: []SubcategoryDef{
			{Name: "Charitable Donations", Aliases: []string{"donation", "charity", "nonprofit", "red cross", "united way", "gofundme"}},
			{Name: "Gifts", Aliases: []string{"gift", "present", "birthday", "holiday"}},
		},
	},
	// Loan Payments
	{
		Name:  "Loan Payments",
		Color: "#0ea5e9", // Sky
		Subcategories: []SubcategoryDef{
			{Name: "Credit Card Payment", Aliases: []string{"credit card payment", "card payment", "pay credit"}},
			{Name: "Auto Loan", Aliases: []string{"auto loan", "car payment", "car loan", "vehicle loan"}},
			{Name: "Student Loan", Aliases: []string{"student loan", "education loan", "navient", "sallie mae", "nelnet"}},
			{Name: "Personal Loan", Aliases: []string{"personal loan", "loan payment"}},
		},
	},
	// Transfer (special category for internal transfers)
	{
		Name:  "Transfers",
		Color: "#6366f1", // Indigo
		Subcategories: []SubcategoryDef{
			{Name: "Internal Transfer", Aliases: []string{"transfer", "moving money"}},
			{Name: "Investment Transfer", Aliases: []string{"brokerage", "investment", "fidelity", "vanguard", "schwab", "robinhood"}},
			{Name: "Person Payment", Aliases: []string{"venmo", "zelle", "cash app", "paypal", "apple cash", "p2p"}},
			{Name: "Person Receipt", Aliases: []string{"received from", "payment from"}},
			{Name: "Household Transfer", Aliases: []string{"spouse", "family", "household"}},
		},
	},
	// Equity (for household/personal transfers that shouldn't affect income/expense)
	{
		Name:  "Equity",
		Color: "#78716c", // Stone
		Subcategories: []SubcategoryDef{
			{Name: "Household Distributions", Aliases: []string{"household", "family transfer", "spouse transfer"}},
			{Name: "Personal Draw", Aliases: []string{"personal", "draw", "owner draw"}},
			{Name: "Contributions", Aliases: []string{"contribution", "deposit from family"}},
		},
	},
}

// TaxonomyService handles category taxonomy operations
type TaxonomyService struct {
	pool *pgxpool.Pool
	tags *models.TagStore
}

// NewTaxonomyService creates a new taxonomy service
func NewTaxonomyService(pool *pgxpool.Pool) *TaxonomyService {
	return &TaxonomyService{
		pool: pool,
		tags: models.NewTagStore(pool),
	}
}

// SeedDefaultTags creates the default category hierarchy for a ledger.
// Idempotent: if a parent already exists its ID is looked up so subcategory
// creation can still proceed even on a partial/re-run seed.
func (s *TaxonomyService) SeedDefaultTags(ctx context.Context, ledgerID uuid.UUID) error {
	existing, err := s.tags.GetByLedgerID(ctx, ledgerID)
	if err != nil {
		return err
	}
	existingByName := make(map[string]uuid.UUID, len(existing))
	for _, t := range existing {
		existingByName[t.Name] = t.ID
	}

	for _, category := range DefaultTaxonomy {
		parent := &models.Tag{
			LedgerID: ledgerID,
			Name:     category.Name,
			Color:    category.Color,
		}

		if err := s.tags.Create(ctx, parent); err != nil {
			slog.WarnContext(ctx, "failed to create parent tag", "err", err, "name", category.Name)
			id, ok := existingByName[category.Name]
			if !ok {
				continue
			}
			parent.ID = id
		}

		for _, sub := range category.Subcategories {
			child := &models.Tag{
				LedgerID: ledgerID,
				ParentID: &parent.ID,
				Name:     sub.Name,
				Color:    category.Color,
			}
			if err := s.tags.Create(ctx, child); err != nil {
				slog.WarnContext(ctx, "failed to create subcategory tag", "err", err, "name", sub.Name)
				continue
			}
		}
	}

	return nil
}

// GetCategoryTreeForPrompt returns a formatted string of the category tree for LLM prompts
func GetCategoryTreeForPrompt(tags []*models.Tag) string {
	// Build a map of parent -> children
	childrenMap := make(map[uuid.UUID][]*models.Tag)
	var roots []*models.Tag

	for _, tag := range tags {
		if tag.ParentID == nil {
			roots = append(roots, tag)
		} else {
			childrenMap[*tag.ParentID] = append(childrenMap[*tag.ParentID], tag)
		}
	}

	var result string
	for _, root := range roots {
		result += "- " + root.Name + "\n"
		for _, child := range childrenMap[root.ID] {
			result += "  - " + child.Name + "\n"
		}
	}

	return result
}

