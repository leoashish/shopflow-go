// Package money only formats money. Amounts are passed around as int64 cents.
package money

import "fmt"

// Format turns cents into a human string: 12345 -> "$123.45", -50 -> "-$0.50".
func Format(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}
