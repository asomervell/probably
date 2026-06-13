package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/demo"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/pages"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DemoPulse renders the Pulse dashboard component with mock data
func (hdl *Handlers) DemoPulse(w http.ResponseWriter, r *http.Request) {
	leftToSpend, upcomingBills, spendingPace := demo.GeneratePulseData()

	// Render just the pulse components (no layout)
	content := h.Div(
		h.Class("space-y-6"),
		// Quick stats row - only Net Worth and Left to Spend
		h.Div(
			h.Class("grid grid-cols-2 gap-4"),
			shadcn.Stat(shadcn.StatProps{Label: "Net Worth", Value: formatMoney(450000), Trend: "Current", Positive: true}),
			hdl.renderLeftToSpend(leftToSpend),
		),
		// Single column layout - Upcoming Bills with Spending Pace below
		h.Div(
			h.Class("space-y-6"),
			hdl.renderUpcomingBills(upcomingBills),
			hdl.renderSpendingPace(spendingPace),
		),
	)

	renderHTML(w, content)
}

// DemoTransactions renders the transactions list component with mock data
func (hdl *Handlers) DemoTransactions(w http.ResponseWriter, r *http.Request) {
	transactions := demo.GenerateTransactions()
	search := r.URL.Query().Get("search")

	// Filter by search if provided
	filtered := transactions
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered = []*models.Transaction{}
		for _, txn := range transactions {
			if strings.Contains(strings.ToLower(txn.Description), searchLower) ||
				strings.Contains(strings.ToLower(txn.DisplayTitle), searchLower) {
				filtered = append(filtered, txn)
			}
		}
	}

	// Limit to 10 for demo
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	// Ensure transactions have proper entry structure with account info and entities with logos
	for _, txn := range filtered {
		if len(txn.Entries) > 0 {
			for _, entry := range txn.Entries {
				if entry.AccountName == "" {
					entry.AccountName = "Checking"
					entry.AccountType = models.AccountTypeAsset
				}
			}
		}
		// Ensure entity has logo URL if entity exists
		if txn.Entity != nil && txn.Entity.LogoURL == "" {
			// Try to get logo from entity name lookup
			// For demo, we'll set it from the transaction description/merchant name
			if txn.Entity.Name != "" {
				// Logo should already be set in GenerateTransactions, but ensure it's there
				// The demo data generator should handle this
			}
		}
	}

	// Create mock accounts list for renderTransactionListItemWithCheckbox
	// Use the account ID from the first transaction's entry
	var mockAccounts []*models.Account
	if len(filtered) > 0 && len(filtered[0].Entries) > 0 {
		accountID := filtered[0].Entries[0].AccountID
		mockAccounts = []*models.Account{
			{
				ID:   accountID,
				Name: "Checking",
				Type: models.AccountTypeAsset,
			},
		}
	}

	// Render transaction list with scrollable container
	content := shadcn.Card(shadcn.CardProps{Class: "overflow-hidden flex flex-col"},
		// Header
		h.Div(
			h.Class("flex items-center px-3 sm:px-4 py-2.5 border-b border-border flex-none"),
			h.Span(h.Class("text-sm font-medium text-foreground"), g.Text("Recent Transactions")),
		),
		// Scrollable transactions container
		h.Div(
			h.Class("overflow-y-auto max-h-[500px]"),
			g.If(len(filtered) == 0,
				shadcn.EmptyNoData("No transactions found", "", nil),
			),
			g.Group(g.Map(filtered, func(txn *models.Transaction) g.Node {
				return renderTransactionListItemWithCheckbox(txn, mockAccounts, "", search, hdl.getLogoURL)
			})),
		),
	)

	renderHTML(w, content)
}

// DemoChat renders the chat interface component with mock data
func (hdl *Handlers) DemoChat(w http.ResponseWriter, r *http.Request) {
	messages := demo.GenerateChatMessages()

	// Convert to ChatMessage format and render markdown for assistant messages
	chatMessages := make([]pages.ChatMessage, len(messages))
	for i, msg := range messages {
		content := msg.Content
		// Render markdown for assistant messages
		if msg.Role == "assistant" {
			content = renderMarkdownToHTML(content)
		}
		chatMessages[i] = pages.ChatMessage{
			ID:      "demo-" + string(rune(i)),
			Role:    msg.Role,
			Content: content,
		}
	}

	// Render chat messages
	content := h.Div(
		h.ID("demo-chat-messages"),
		h.Class("space-y-4"),
		g.Group(g.Map(chatMessages, func(msg pages.ChatMessage) g.Node {
			return pages.RenderMessage(msg)
		})),
	)

	renderHTML(w, content)
}

// DemoChatMessage handles POST requests to add a new message to the chat
func (hdl *Handlers) DemoChatMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	question := r.FormValue("message")
	if question == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	// Generate a mock AI response based on the question
	response := generateMockChatResponse(question)
	// Render markdown for assistant response
	responseHTML := renderMarkdownToHTML(response)

	// Return both user message and AI response
	content := h.Div(
		h.Class("space-y-4"),
		// User message
		pages.RenderMessage(pages.ChatMessage{
			ID:      "demo-user-" + string(rune(time.Now().Unix())),
			Role:    "user",
			Content: question,
		}),
		// AI response (with markdown rendered)
		pages.RenderMessage(pages.ChatMessage{
			ID:      "demo-assistant-" + string(rune(time.Now().Unix())),
			Role:    "assistant",
			Content: responseHTML,
		}),
	)

	renderHTML(w, content)
}

// generateMockChatResponse generates a realistic AI response based on the question
func generateMockChatResponse(question string) string {
	questionLower := strings.ToLower(question)

	if strings.Contains(questionLower, "grocer") {
		return "You've spent **$387.50** on groceries this month so far. That's across 12 transactions, with your largest purchase being $125.00 at Whole Foods on January 15th.\n\nCompared to last month, you're spending about 8% more on groceries. Your average grocery transaction is $32.29."
	}

	if strings.Contains(questionLower, "top") || strings.Contains(questionLower, "category") || strings.Contains(questionLower, "spending") {
		return "Here are your top spending categories this month:\n\n1. **Groceries**: $387.50 (28% of total)\n2. **Transportation**: $245.20 (18%)\n3. **Shopping**: $189.99 (14%)\n4. **Coffee & Tea**: $87.50 (6%)\n5. **Entertainment**: $42.97 (3%)\n\nYour total spending this month is $1,382.16."
	}

	if strings.Contains(questionLower, "net worth") || strings.Contains(questionLower, "worth") {
		return "Your current net worth is **$450,000**. This includes:\n\n- **Assets**: $550,000 (checking, savings, investments)\n- **Liabilities**: $100,000 (credit cards, loans)\n\nYour net worth has increased by $12,500 this month."
	}

	// Default response
	return "I can help you understand your finances! Try asking about:\n\n- Your spending by category\n- Net worth and account balances\n- Upcoming bills\n- Spending trends\n\nWhat would you like to know?"
}
