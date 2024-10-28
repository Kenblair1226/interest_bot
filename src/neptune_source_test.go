package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNeptuneSource_FetchRates(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		expectedCount  int
		expectError    bool
		expectedValues map[string][]float64 // currency -> [lend rate, borrow rate]
	}{
		{
			name: "successful response",
			responseBody: `{
				"lend_aprs": [
					[{"native_token": {"denom": "ibc/F51BB221BAA275F2EBF654F70B005627D7E713AFFD6D86AFD1E43CAA886149F4"}}, "0.0525"],
					[{"native_token": {"denom": "ibc/2CBC2EA121AE42563B08028466F37B600F2D7D4282342DE938283CC3FB2BC00E"}}, "0.0320"]
				],
				"borrow_aprs": [
					[{"native_token": {"denom": "ibc/F51BB221BAA275F2EBF654F70B005627D7E713AFFD6D86AFD1E43CAA886149F4"}}, "0.0625"],
					[{"native_token": {"denom": "ibc/2CBC2EA121AE42563B08028466F37B600F2D7D4282342DE938283CC3FB2BC00E"}}, "0.0420"]
				]
			}`,
			expectedCount: 2,
			expectError:   false,
			expectedValues: map[string][]float64{
				"TIA":  {5.25, 6.25},
				"USDC": {3.20, 4.20},
			},
		},
		{
			name:           "empty response",
			responseBody:   `{"lend_aprs": [], "borrow_aprs": []}`,
			expectedCount:  0,
			expectError:    false,
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

			source := NewNeptuneSource()
			source.APIURL = server.URL

			updates, err := source.FetchRates()

			if (err != nil) != tt.expectError {
				t.Errorf("FetchRates() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && len(updates) != tt.expectedCount {
				t.Errorf("FetchRates() got %v updates, want %v", len(updates), tt.expectedCount)
			}

			if tt.expectedValues != nil {
				for currency, expectedRates := range tt.expectedValues {
					found := false
					for _, update := range updates {
						if strings.Contains(update, currency) {
							var lendRate, borrowRate float64
							fmt.Sscanf(update, fmt.Sprintf("Neptune %s: Lend: %s, Borrow: %s", currency, "%f%%", "%f%%"), &lendRate, &borrowRate)
							found = true
							if lendRate != expectedRates[0] || borrowRate != expectedRates[1] {
								t.Errorf("FetchRates() got rates %.2f/%.2f for %s, want %.2f/%.2f",
									lendRate, borrowRate, currency, expectedRates[0], expectedRates[1])
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
