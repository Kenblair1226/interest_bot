package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

type InjeraSource struct {
	APIURL string
}

type InjeraResponse struct {
	Data struct {
		ReturnData []struct {
			Success bool   `json:"success"`
			Data    string `json:"data"`
		} `json:"return_data"`
	} `json:"data"`
}

type InjeraRates struct {
	BorrowRate    string `json:"borrow_rate"`
	LiquidityRate string `json:"liquidity_rate"`
	// Add other fields if necessary
}

func NewInjeraSource() *InjeraSource {
	return &InjeraSource{
		APIURL: "https://inj24984.allnodes.me:1317/iAeAChGmajFpOeRk/cosmwasm/wasm/v1/contract/inj1578zx2zmp46l554zlw5jqq3nslth6ss04dv0ee/smart/ewogICJhZ2dyZWdhdGUiOiB7CiAgICAicXVlcmllcyI6IFsKICAgICAgewogICAgICAgICJhZGRyZXNzIjogImluajFkZmZ1ajR1ZDJmbjd2aGh3N2RlYzZhcng3dHV5eGQ1NnNyandrNCIsCiAgICAgICAgImRhdGEiOiAiZXlKdFlYSnJaWFFpT25zaVpHVnViMjBpT2lKd1pXZG5lVEI0WkVGRE1UZEdPVFU0UkRKbFpUVXlNMkV5TWpBMk1qQTJPVGswTlRrM1F6RXpSRGd6TVdWak55SjlmUT09IgogICAgICB9LAogICAgICB7CiAgICAgICAgImFkZHJlc3MiOiAiaW5qMXE1ZTZwbGVoMmg5ZDJxcjNtN2RybXhqN2tsZ3VnZzdweHZ1a3d0IiwKICAgICAgICAiZGF0YSI6ICJleUpoWTNScGRtVmZaVzFwYzNOcGIyNXpJanA3SW1OdmJHeGhkR1Z5WVd4ZlpHVnViMjBpT2lKd1pXZG5lVEI0WkVGRE1UZEdPVFU0UkRKbFpUVXlNMkV5TWpBMk1qQTJPVGswTlRrM1F6RXpSRGd6TVdWak55SjlmUT09IgogICAgICB9CiAgICBdCiAgfQp9",
	}
}

func (s *InjeraSource) FetchRates() ([]Rate, error) {
	resp, err := http.Get(s.APIURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching Injera data: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Injera response body: %v", err)
	}

	var injeraResp InjeraResponse
	err = json.Unmarshal(body, &injeraResp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling Injera response: %v", err)
	}

	var rates []Rate
	for _, item := range injeraResp.Data.ReturnData {
		if item.Success && item.Data != "" {
			decodedData, err := base64.StdEncoding.DecodeString(item.Data)
			if err != nil {
				log.Printf("Error decoding Injera data: %v", err)
				continue
			}

			var ratesData InjeraRates
			err = json.Unmarshal(decodedData, &ratesData)
			if err != nil {
				// Ignore the error and skip this item
				continue
			}

			// Convert rate strings to float64
			borrowRate, err := strconv.ParseFloat(ratesData.BorrowRate, 64)
			if err != nil {
				log.Printf("Error parsing borrow_rate for Injera: %v", err)
				continue
			}

			liquidityRate, err := strconv.ParseFloat(ratesData.LiquidityRate, 64)
			if err != nil {
				log.Printf("Error parsing liquidity_rate for Injera: %v", err)
				continue
			}

			// Format the update as a Rate struct for USDT
			rate := Rate{
				Source:      "Injera",
				Token:       "USDT",
				BorrowRate:  borrowRate * 100,
				LendingRate: liquidityRate * 100,
			}
			rates = append(rates, rate)
		}
	}

	return rates, nil
}
