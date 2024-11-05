package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OKXSource struct {
	APIURLTemplate string
	CurrencyIDs    []int
}

type OKXResponse struct {
	Data struct {
		List []struct {
			CurrencyName  string  `json:"currencyName"`
			EstimatedRate float64 `json:"estimatedRate"`
			PreRate       float64 `json:"preRate"`
			AvgRate       float64 `json:"avgRate"`
		} `json:"list"`
	} `json:"data"`
}

func NewOKXSource() *OKXSource {
	return &OKXSource{
		APIURLTemplate: "https://www.okx.com/priapi/v2/financial/market-lending-info?currencyId=%d",
		CurrencyIDs:    []int{2854, 7, 283}, // TIA, USDT, USDC
	}
}

func (s *OKXSource) FetchRates() ([]string, error) {
	var updates []string
	for _, currencyID := range s.CurrencyIDs {
		estimatedRate, preRate, avgRate, currencyName, err := s.fetchInterestRates(currencyID)
		if err != nil {
			return nil, fmt.Errorf("error fetching interest rates for currency ID %d: %v", currencyID, err)
		}

		update := fmt.Sprintf("OKX %s: %.2f%% (prev: %.2f%%, avg: %.2f%%)",
			currencyName, estimatedRate*100, preRate*100, avgRate*100)
		updates = append(updates, update)
	}
	return updates, nil
}

func (s *OKXSource) fetchInterestRates(currencyID int) (float64, float64, float64, string, error) {
	url := fmt.Sprintf(s.APIURLTemplate, currencyID)
	resp, err := http.Get(url)
	if err != nil {
		return 0, 0, 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, "", err
	}

	var okxResp OKXResponse
	err = json.Unmarshal(body, &okxResp)
	if err != nil {
		return 0, 0, 0, "", err
	}

	if len(okxResp.Data.List) == 0 {
		return 0, 0, 0, "", fmt.Errorf("no data found in OKX response for currency ID %d", currencyID)
	}

	data := okxResp.Data.List[0]
	return data.EstimatedRate, data.PreRate, data.AvgRate, data.CurrencyName, nil
}
