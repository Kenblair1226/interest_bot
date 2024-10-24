package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type NeptuneSource struct {
	APIURL string
	Tokens map[string]string
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
	}
}

func (s *NeptuneSource) FetchRates() ([]string, error) {
	resp, err := http.Get(s.APIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var neptuneResp NeptuneResponse
	err = json.Unmarshal(body, &neptuneResp)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]string)

	processRates := func(rates [][]interface{}, rateType string) {
		for _, rate := range rates {
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

			if existingUpdate, ok := updates[tokenName]; ok {
				updates[tokenName] = fmt.Sprintf("%s, %s: %.2f%%", existingUpdate, rateType, ratePercent)
			} else {
				updates[tokenName] = fmt.Sprintf("%s: %.2f%%", rateType, ratePercent)
			}
		}
	}

	processRates(neptuneResp.LendAPRs, "Lend")
	processRates(neptuneResp.BorrowAPRs, "Borrow")

	var result []string
	for token, rates := range updates {
		result = append(result, fmt.Sprintf("Neptune %s: %s", token, rates))
	}

	return result, nil
}
