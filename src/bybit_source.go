package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type BybitSource struct {
	Category string
}

type BybitResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		FlexibleSavingProductDetail struct {
			TieredApyList []struct {
				ApyE8 string `json:"apy_e8"`
			} `json:"tiered_apy_list"`
			Coin int    `json:"coin"`
			Name string `json:"name"`
		} `json:"flexible_saving_product_detail"`
	} `json:"result"`
}

func NewBybitSource() *BybitSource {
	return &BybitSource{
		Category: "CEX",
	}
}

func (s *BybitSource) FetchRates() ([]Rate, error) {
	url := "https://api2.bybit.com/s1/byfi/get-product-detail"
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// We'll fetch both USDT and USDC
	productIDs := []string{"1", "2"} // 1 for USDT, 2 for USDC
	var rates []Rate

	for _, productID := range productIDs {
		payload := fmt.Sprintf(`{"product_type":4,"product_id":"%s"}`, productID)

		req, err := http.NewRequest("POST", url, strings.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Add required headers
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Accept", "*/*")
		req.Header.Add("Referer", "https://www.bybit.com/")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch bybit rates: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read bybit response: %v", err)
		}

		var response BybitResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("failed to parse bybit response: %v, body: %s", err, string(body))
		}

		if response.RetCode != 0 {
			return nil, fmt.Errorf("bybit API error: %s (code: %d)", response.RetMsg, response.RetCode)
		}

		if len(response.Result.FlexibleSavingProductDetail.TieredApyList) > 1 {
			apyE8, err := strconv.ParseInt(response.Result.FlexibleSavingProductDetail.TieredApyList[1].ApyE8, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse apy value: %v", err)
			}
			apy := float64(apyE8) / 1000000
			rates = append(rates, Rate{
				Token:       response.Result.FlexibleSavingProductDetail.Name,
				LendingRate: apy,
				BorrowRate:  0,
				Source:      "Bybit",
				Category:    s.Category,
			})
		}
	}

	return rates, nil
}
