package blockchain

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type Settings struct {
	Currencies           []*currency.Currency
	WhitelistedAddresses []string
	URI                  string
}

type Blockchain interface {
	Configure(settings *Settings)
	GetLatestBlockNumber(ctx context.Context) (int64, error)
	GetBlockByHash(ctx context.Context, hash string) (*block.Block, error)
	GetBlockByNumber(ctx context.Context, blockNumber int64) (*block.Block, error)
	GetTransaction(ctx context.Context, transactionHash string) (*transaction.Transaction, error)
	GetBalanceOfAddress(ctx context.Context, address string, currencyID string) (decimal.Decimal, error)
}
