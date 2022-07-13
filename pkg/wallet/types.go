package wallet

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zsmartex/multichain/pkg/currency"
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
	Currency *currency.Currency
	Wallet   *SettingWallet
}

type Wallet interface {
	Configure(settings *Setting) error

	// CreateAddress Create new address from server
	CreateAddress() (ctx context.Context, address string, secret string, err error)

	// CreateTransaction Create a transaction and send it to the server
	CreateTransaction(ctx context.Context, transaction *transaction.Transaction) (*transaction.Transaction, error)

	// LoadBalance Load balance from wallet address
	LoadBalance(ctx context.Context) (balance decimal.Decimal, err error)

	// PrepareDepositCollection Prepare deposit collection fee for deposit
	// WARN: this func don't execute create transaction just return transaction was built
	PrepareDepositCollection(ctx context.Context, depositTransaction *transaction.Transaction, depositSpreads []*transaction.Transaction, depositCurrency *currency.Currency) (*transaction.Transaction, error)
}
