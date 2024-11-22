package main

type Rate struct {
	Source      string  `json:"source"`
	Token       string  `json:"token"`
	BorrowRate  float64 `json:"borrow_rate"`
	LendingRate float64 `json:"lending_rate"`
	Category    string  `json:"category"`
}

type Source interface {
	FetchRates() ([]Rate, error)
}

func GetSources() []Source {
	return []Source{
		NewOKXSource(),
		NewNeptuneSource(),
		NewInjeraSource(),
		NewBinanceSimpleEarnSource(),
	}
}
