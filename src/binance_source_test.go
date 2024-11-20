package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBinanceSimpleEarnSource_FetchRates(t *testing.T) {
	source := NewBinanceSimpleEarnSource()
	rates, err := source.FetchRates()

	if err != nil {
		t.Logf("Error fetching rates: %v", err)
		t.FailNow()
	}

	assert.NotEmpty(t, rates, "Should get at least one rate")

	for _, rate := range rates {
		t.Logf("Got rate: Source=%s, Token=%s, LendingRate=%f",
			rate.Source, rate.Token, rate.LendingRate)

		assert.Equal(t, "binance_simple_earn", rate.Source)
		assert.Contains(t, []string{"USDT", "FDUSD"}, rate.Token)
		assert.GreaterOrEqual(t, rate.LendingRate, 0.0)
		assert.Equal(t, 0.0, rate.BorrowRate)
	}
}
