package wallet

import (
	"github.com/zsmartex/multichain/pkg/transaction"
)

type GasPriceRate string

const (
	GasPriceRateStandard GasPriceRate = "standard"
	GasPriceRateFast     GasPriceRate = "fast"
)

type Wallet interface {
	// Get current balance of address
	GetBalance(address string)

	// Create new address from server
	CreateAddress(secret string)

	// Create a transaction and send it to the server
	CreateTransaction(transaction transaction.Transaction, secret string) error
}
