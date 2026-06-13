package handlers

import (
	"github.com/asomervell/probably/internal/models"
)

func formatMoney(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}

	dollars := cents / 100
	remainder := cents % 100

	result := "$"
	if negative {
		result = "-$"
	}

	// Add commas for thousands
	dollarStr := formatWithCommas(dollars)
	return result + dollarStr + "." + padZero(remainder)
}

func formatMoneyWithSign(cents int64) string {
	if cents >= 0 {
		return "+" + formatMoney(cents)
	}
	return formatMoney(cents)
}

// displayBalance returns the balance formatted for display.
// For liabilities, we flip the sign so debt shows as negative.
func displayBalance(balance int64, accountType models.AccountType) string {
	if accountType == models.AccountTypeLiability {
		return formatMoney(-balance) // Flip sign: stored positive debt → display negative
	}
	return formatMoney(balance)
}

// displayBalanceWithSign returns the balance with +/- sign for display.
func displayBalanceWithSign(balance int64, accountType models.AccountType) string {
	if accountType == models.AccountTypeLiability {
		return formatMoneyWithSign(-balance) // Flip sign for liabilities
	}
	return formatMoneyWithSign(balance)
}

func formatWithCommas(n int64) string {
	if n < 1000 {
		return itoa64(n)
	}
	return formatWithCommas(n/1000) + "," + padZeroThree(n%1000)
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func padZero(n int64) string {
	if n < 10 {
		return "0" + itoa64(n)
	}
	return itoa64(n)
}

func padZeroThree(n int64) string {
	if n < 10 {
		return "00" + itoa64(n)
	}
	if n < 100 {
		return "0" + itoa64(n)
	}
	return itoa64(n)
}

// amountColorClass returns the CSS class for balance display.
// For liabilities, positive stored value = debt = bad (red)
// For assets, positive stored value = money = good (green)
func amountColorClass(amount int64, accountType models.AccountType) string {
	switch accountType {
	case models.AccountTypeAsset:
		if amount >= 0 {
			return "amount-positive"
		}
		return "amount-negative"
	case models.AccountTypeLiability:
		// Liabilities: stored positive = debt = bad (show as red)
		// stored negative = credit/overpayment = good (show as green)
		if amount > 0 {
			return "amount-negative" // Debt is red
		}
		return "amount-positive"
	default:
		return "text-foreground"
	}
}

func simpleAmountColorClass(amount int64) string {
	if amount > 0 {
		return "amount-positive"
	}
	if amount < 0 {
		return "amount-negative"
	}
	return "amount-neutral"
}

// transactionAmountColorClass returns the color class for a transaction amount.
// Amounts are stored as Teller reports:
// - For assets: positive = deposit (good), negative = withdrawal (neutral/bad)
// - For liabilities: positive = purchase/debt increase (bad), negative = payment (good)
func transactionAmountColorClass(amount int64, accountType models.AccountType) string {
	switch accountType {
	case models.AccountTypeLiability:
		// For liabilities, flip the interpretation:
		// positive (debt increase) = bad, negative (payment) = good
		return simpleAmountColorClass(-amount)
	default:
		return simpleAmountColorClass(amount)
	}
}

// formatAccountType returns a human-readable label for account types
func formatAccountType(accountType models.AccountType) string {
	switch accountType {
	case models.AccountTypeAsset:
		return "Asset"
	case models.AccountTypeLiability:
		return "Liability"
	case models.AccountTypeIncome:
		return "Income"
	case models.AccountTypeExpense:
		return "Expense"
	case models.AccountTypeEquity:
		return "Equity"
	default:
		return string(accountType)
	}
}
