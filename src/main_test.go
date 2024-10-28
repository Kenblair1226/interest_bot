package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
		setEnv       bool
	}{
		{
			name:         "existing environment variable",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "test_value",
			want:         "test_value",
			setEnv:       true,
		},
		{
			name:         "non-existing environment variable",
			key:          "TEST_VAR",
			defaultValue: "default",
			want:         "default",
			setEnv:       false,
		},
		{
			name:         "empty environment variable",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "",
			setEnv:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			if got := getEnv(tt.key, tt.defaultValue); got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name      string
		strings   []string
		separator string
		want      string
	}{
		{
			name:      "empty slice",
			strings:   []string{},
			separator: ",",
			want:      "",
		},
		{
			name:      "single string",
			strings:   []string{"test"},
			separator: ",",
			want:      "test",
		},
		{
			name:      "multiple strings",
			strings:   []string{"test1", "test2", "test3"},
			separator: ",",
			want:      "test1,test2,test3",
		},
		{
			name:      "multiple strings with newline separator",
			strings:   []string{"test1", "test2", "test3"},
			separator: "\n",
			want:      "test1\ntest2\ntest3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinStrings(tt.strings, tt.separator); got != tt.want {
				t.Errorf("joinStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock implementation of rate source for testing
type MockRateSource struct {
	rates []string
	err   error
}

func (m *MockRateSource) FetchRates() ([]string, error) {
	return m.rates, m.err
}

func TestCombineUpdates(t *testing.T) {
	tests := []struct {
		name          string
		okxRates      []string
		neptuneRates  []string
		expectedCount int
	}{
		{
			name:          "both sources have updates",
			okxRates:      []string{"OKX BTC: 1.2%", "OKX ETH: 2.3%"},
			neptuneRates:  []string{"Neptune BTC: 1.3%", "Neptune ETH: 2.4%"},
			expectedCount: 4,
		},
		{
			name:          "only OKX has updates",
			okxRates:      []string{"OKX BTC: 1.2%"},
			neptuneRates:  []string{},
			expectedCount: 1,
		},
		{
			name:          "only Neptune has updates",
			okxRates:      []string{},
			neptuneRates:  []string{"Neptune BTC: 1.3%"},
			expectedCount: 1,
		},
		{
			name:          "no updates",
			okxRates:      []string{},
			neptuneRates:  []string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okxSource := &MockRateSource{rates: tt.okxRates}
			neptuneSource := &MockRateSource{rates: tt.neptuneRates}

			okxUpdates, _ := okxSource.FetchRates()
			neptuneUpdates, _ := neptuneSource.FetchRates()

			allUpdates := append(okxUpdates, neptuneUpdates...)
			if len(allUpdates) != tt.expectedCount {
				t.Errorf("Combined updates count = %v, want %v", len(allUpdates), tt.expectedCount)
			}
		})
	}
}
