package transaction

import (
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusSucceed  Status = "succeed"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped"
	StatusRejected Status = "rejected"
)

type Transaction struct {
	Currency    string
	CurrencyFee string
	FromAddress string
	ToAddress   string
	Fee         decimal.Decimal
	Amount      decimal.Decimal
	BlockNumber int64
	TxHash      null.String
	Status      Status
	Options     map[string]interface{}
}

func New(currency string, fromAddress, toAddress string, amount decimal.Decimal, txHash null.String) Transaction {
	return Transaction{
		Currency:    currency,
		FromAddress: fromAddress,
		ToAddress:   toAddress,
		Amount:      amount,
		TxHash:      txHash,
	}
}
