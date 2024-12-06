package main

import (
	"testing"
)

func TestBybitSource_FetchRates(t *testing.T) {
	source := NewBybitSource()
	rates, err := source.FetchRates()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(rates) == 0 {
		t.Error("Expected some rates to be returned")
	}

	expectedTokens := map[string]bool{
		"USDT": false,
		"USDC": false,
	}

	for _, rate := range rates {
		if rate.Source != "Bybit" {
			t.Errorf("Expected source to be Bybit, got %s", rate.Source)
		}
		if rate.Category != "CEX" {
			t.Errorf("Expected category to be CEX, got %s", rate.Category)
		}
		if rate.Token == "" {
			t.Error("Expected token to not be empty")
		}
		if rate.LendingRate <= 0 {
			t.Errorf("Expected lending rate to be positive, got %f", rate.LendingRate)
		}
		expectedTokens[rate.Token] = true
	}

	// Check if we got rates for both expected tokens
	for token, found := range expectedTokens {
		if !found {
			t.Errorf("Expected to find rates for %s", token)
		}
	}
}
