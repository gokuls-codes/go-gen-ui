package server

import (
	"testing"
)

func TestParseAmountAndDescription(t *testing.T) {
	tests := []struct {
		input       string
		expectedAmt string
		expectedDesc string
	}{
		{
			input:       "Dinner with team - $45",
			expectedAmt: "45",
			expectedDesc: "Dinner with team",
		},
		{
			input:       "Lunch for 15.50 dollars",
			expectedAmt: "15.50",
			expectedDesc: "Lunch",
		},
		{
			input:       "Coffee 5 bucks",
			expectedAmt: "5",
			expectedDesc: "Coffee",
		},
		{
			input:       "Flight ticket - 350.00",
			expectedAmt: "350.00",
			expectedDesc: "Flight ticket",
		},
		{
			input:       "Bought groceries for 32.50 at Trader Joe's",
			expectedAmt: "32.50",
			expectedDesc: "Bought groceries at Trader Joe's",
		},
		{
			input:       "Grocery shopping at Trader Joe's",
			expectedAmt: "",
			expectedDesc: "Grocery shopping at Trader Joe's",
		},
	}

	for _, tt := range tests {
		amt, desc := parseAmountAndDescription(tt.input)
		if amt != tt.expectedAmt {
			t.Errorf("for %q, expected amount %q, got %q", tt.input, tt.expectedAmt, amt)
		}
		if desc != tt.expectedDesc {
			t.Errorf("for %q, expected desc %q, got %q", tt.input, tt.expectedDesc, desc)
		}
	}
}
