package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type NeptuneSource struct {
	APIURL   string
	Tokens   map[string]string
	Category string
}

type NeptuneResponse struct {
	BorrowAPRs [][]interface{} `json:"borrow_aprs"`
	LendAPRs   [][]interface{} `json:"lend_aprs"`
}

func NewNeptuneSource() *NeptuneSource {
	return &NeptuneSource{
		APIURL: "https://neptune-api-production-6ojz3.ondigitalocean.app/v1/aprs?refresh=false",
		Tokens: map[string]string{
			"ibc/2CBC2EA121AE42563B08028466F37B600F2D7D4282342DE938283CC3FB2BC00E": "USDC",
			"ibc/F51BB221BAA275F2EBF654F70B005627D7E713AFFD6D86AFD1E43CAA886149F4": "TIA",
			"peggy0xdAC17F958D2ee523a2206206994597C13D831ec7":                      "USDT",
		},
		Category: "DEX",
	}
}

func (s *NeptuneSource) FetchRates() ([]Rate, error) {
	resp, err := http.Get(s.APIURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching Neptune data: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Neptune response body: %v", err)
	}

	var neptuneResp NeptuneResponse
	err = json.Unmarshal(body, &neptuneResp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling Neptune response: %v", err)
	}

	// Create a map to store rates by token
	ratesByToken := make(map[string]*Rate)

	processRates := func(ratesData [][]interface{}, rateType string) {
		for _, rate := range ratesData {
			if len(rate) != 2 {
				continue
			}
			tokenInfo, ok := rate[0].(map[string]interface{})
			if !ok {
				continue
			}
			nativeToken, ok := tokenInfo["native_token"].(map[string]interface{})
			if !ok {
				continue
			}
			denom, ok := nativeToken["denom"].(string)
			if !ok {
				continue
			}
			tokenName, ok := s.Tokens[denom]
			if !ok {
				continue // Skip tokens not in the mapping
			}
			rateStr, ok := rate[1].(string)
			if !ok {
				continue
			}
			rateFloat, err := strconv.ParseFloat(rateStr, 64)
			if err != nil {
				continue
			}
			ratePercent := rateFloat * 100

			// Get or create rate struct for this token
			rateStruct, exists := ratesByToken[tokenName]
			if !exists {
				rateStruct = &Rate{
					Source:   "Neptune",
					Token:    tokenName,
					Category: s.Category,
				}
				ratesByToken[tokenName] = rateStruct
			}

			// Update the appropriate rate
			if rateType == "Borrow" {
				rateStruct.BorrowRate = ratePercent
			} else if rateType == "Lend" {
				rateStruct.LendingRate = ratePercent
			}
		}
	}

	// Process both types of rates
	processRates(neptuneResp.LendAPRs, "Lend")
	processRates(neptuneResp.BorrowAPRs, "Borrow")

	// Convert map to slice
	var rates []Rate
	for _, rate := range ratesByToken {
		rates = append(rates, *rate)
	}

	return rates, nil
}
