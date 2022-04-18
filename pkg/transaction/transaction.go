package transaction

import (
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
)

type TransactionStatus string

const (
	TransactionStatusPending TransactionStatus = "pending"
	TransactionStatusSuccess TransactionStatus = "success"
	TransactionStatusFailed  TransactionStatus = "failed"
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
	Status      TransactionStatus
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
