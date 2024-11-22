package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BinanceSimpleEarnSource struct {
	client   *http.Client
	Category string
}

type BinanceSimpleEarnFullResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List []struct {
			Asset    string   `json:"asset"`
			ApyRange []string `json:"apyRange"`
		} `json:"list"`
	} `json:"data"`
	Success bool `json:"success"`
}

func NewBinanceSimpleEarnSource() *BinanceSimpleEarnSource {
	return &BinanceSimpleEarnSource{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		Category: "CEX",
	}
}

func (b *BinanceSimpleEarnSource) FetchRates() ([]Rate, error) {
	url := "https://www.binance.com/bapi/earn/v1/friendly/finance-earn/simple-earn/homepage/details"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching binance simple earn data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var response BinanceSimpleEarnFullResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshaling response (body: %s): %w", string(body), err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API returned unsuccessful response: %s", response.Message)
	}

	var rates []Rate
	for _, product := range response.Data.List {
		if product.Asset != "USDT" && product.Asset != "FDUSD" {
			continue
		}

		if len(product.ApyRange) == 0 {
			continue
		}

		// Use the highest APY from the range
		var maxApy float64
		for _, apyStr := range product.ApyRange {
			var apy float64
			if _, err := fmt.Sscanf(apyStr, "%f", &apy); err == nil {
				if apy > maxApy {
					maxApy = apy
				}
			}
		}

		rates = append(rates, Rate{
			Source:      "Binance",
			Token:       product.Asset,
			BorrowRate:  0,
			LendingRate: maxApy * 100,
			Category:    b.Category,
		})
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("no valid rates found in response")
	}

	return rates, nil
}
