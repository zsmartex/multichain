package wallet

import (
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type GasPriceRate string

const (
	GasPriceRateStandard GasPriceRate = "standard"
	GasPriceRateFast     GasPriceRate = "fast"
)

type SettingWallet struct {
	URI     string
	Secret  string
	Address string
}

type Setting struct {
	Currency *blockchain.Currency
	Wallet   *SettingWallet
}

type Wallet interface {
	Configure(settings *Setting) error

	// Create new address from server
	CreateAddress() (address string, secret string, err error)

	// Create a transaction and send it to the server
	CreateTransaction(transaction *transaction.Transaction) (*transaction.Transaction, error)
}
