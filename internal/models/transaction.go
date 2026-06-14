package models

type Transaction struct {
    ID          int64   `json:"id"`
    EmailHash   string  `json:"email_hash"`
    Merchant    string  `json:"merchant"`
    Amount      float64 `json:"amount"`
    Category    string  `json:"category"`
    IsExpense   bool    `json:"is_expense"`
    Date        string  `json:"date"`
    Description string  `json:"description"`
    Confirmed   bool    `json:"confirmed"`
    UpdatedAt   string  `json:"updated_at"`
}
