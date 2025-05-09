package model

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
