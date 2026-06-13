package models

import "fmt"

// FormatAmount formats an amount in cents as a signed dollar string, e.g. +$12.34 or -$5.00.
// Used in LLM prompt construction across packages.
func FormatAmount(cents int64) string {
	sign := "+"
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

// FormatCents formats an amount in cents as a dollar string, e.g. $12.34 or -$5.00.
// Used in LLM prompt construction across packages.
func FormatCents(cents int64) string {
	if cents < 0 {
		cents = -cents
		return fmt.Sprintf("-$%d.%02d", cents/100, cents%100)
	}
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
