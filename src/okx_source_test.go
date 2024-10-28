package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOKXSource_FetchRates(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		expectedCount  int
		expectError    bool
		expectedValues map[string]float64 // currency -> rate
	}{
		{
			name: "successful response",
			responseBody: `{
				"data": {
					"list": [
						{
							"currencyName": "TIA",
							"estimatedRate": 0.0525,
							"preRate": 0.0520,
							"avgRate": 0.0522
						}
					]
				}
			}`,
			expectedCount: 1,
			expectError:   false,
			expectedValues: map[string]float64{
				"TIA": 5.25, // 0.0525 * 100
			},
		},
		{
			name:           "empty response",
			responseBody:   `{"data": {"list": []}}`,
			expectedCount:  0,
			expectError:    true,
			expectedValues: nil,
		},
		{
			name:           "invalid JSON",
			responseBody:   `invalid json`,
			expectedCount:  0,
			expectError:    true,
			expectedValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(tt.responseBody)) // Check the error
				if err != nil {
					t.Errorf("Failed to write test response: %v", err)
				}
			}))
			defer server.Close()

			source := &OKXSource{
				APIURLTemplate: server.URL + "?currencyId=%d",
				CurrencyIDs:    []int{2854},
			}

			updates, err := source.FetchRates()

			if (err != nil) != tt.expectError {
				t.Errorf("FetchRates() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && len(updates) != tt.expectedCount {
				t.Errorf("FetchRates() got %v updates, want %v", len(updates), tt.expectedCount)
			}

			if tt.expectedValues != nil {
				for currency, expectedRate := range tt.expectedValues {
					found := false
					for _, update := range updates {
						if strings.Contains(update, currency) {
							t.Logf("Update string: %s", update)

							var actualRate float64
							n, err := fmt.Sscanf(update, fmt.Sprintf("OKX %s: %%f", currency), &actualRate)
							if err != nil {
								t.Errorf("Failed to parse rate: %v, matched %d items", err, n)
								continue
							}
							found = true
							if actualRate != expectedRate {
								t.Errorf("FetchRates() got rate %.2f for %s, want %.2f", actualRate, currency, expectedRate)
							}
							break
						}
					}
					if !found {
						t.Errorf("FetchRates() did not find update for currency %s", currency)
					}
				}
			}
		})
	}
}
