package model

import ( 
    "fmt"
    "regexp"
    "strings"
)

type Trade struct {
    ID      int64   `json:"id,omitempty"`
    Account string  `json:"account"`
    Symbol  string  `json:"symbol"`
    Volume  float64 `json:"volume"`
    Open    float64 `json:"open"`
    Close   float64 `json:"close"`
    Side    string  `json:"side"` // "buy" или "sell"
}

type Stats struct {
    Account string  `json:"account"`
    Trades  int     `json:"trades"`
    Profit  float64 `json:"profit"`
}

// Validate checks if the trade data is valid
func (t *Trade) Validate() error {
    if t.Account == "" {
        return fmt.Errorf("account must not be empty")
    }

    symbolRegex := regexp.MustCompile(`^[A-Z]{6}$`)
    if !symbolRegex.MatchString(t.Symbol) {
        return fmt.Errorf("symbol must match the format ^[A-Z]{6}$")
    }

    if t.Volume <= 0 {
        return fmt.Errorf("volume must be > 0")
    }

    if t.Open <= 0 {
        return fmt.Errorf("open must be > 0")
    }

    if t.Close <= 0 {
        return fmt.Errorf("close must be > 0")
    }

    t.Side = strings.ToLower(t.Side)
    if t.Side != "buy" && t.Side != "sell" {
        return fmt.Errorf("side must be either 'buy' or 'sell'")
    }

    return nil
}

func (t *Trade) CalculateProfit() float64 {
    lot := 100000.0
    profit := (t.Close - t.Open) * t.Volume * lot
    if t.Side == "sell" {
        profit = -profit
    }
    return profit
}
