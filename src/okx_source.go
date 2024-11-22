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
	Category       string
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
		Category:       "CEX",
	}
}

func (s *OKXSource) FetchRates() ([]Rate, error) {
	var rates []Rate
	for _, currencyID := range s.CurrencyIDs {
		estimatedRate, preRate, _, currencyName, err := s.fetchInterestRates(currencyID)
		if err != nil {
			return nil, fmt.Errorf("error fetching interest rates for currency ID %d: %v", currencyID, err)
		}

		rates = append(rates, Rate{
			Source:      "OKX",
			Token:       currencyName,
			BorrowRate:  preRate * 100,       // Assuming preRate is the borrow rate
			LendingRate: estimatedRate * 100, // Assuming estimatedRate is the lending rate
			Category:    s.Category,
		})
	}
	return rates, nil
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
