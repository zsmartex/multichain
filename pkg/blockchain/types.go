package blockchain

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type Setting struct {
	Currencies           []*currency.Currency
	WhitelistedAddresses []string
	URI                  string
}

type Blockchain interface {
	Configure(setting *Setting)
	GetLatestBlockNumber(ctx context.Context) (int64, error)
	GetBlockByHash(ctx context.Context, hash string) (*block.Block, error)
	GetBlockByNumber(ctx context.Context, blockNumber int64) (*block.Block, error)
	GetTransaction(ctx context.Context, transactionHash string) (*transaction.Transaction, error)
	GetBalanceOfAddress(ctx context.Context, address string, currencyID string) (decimal.Decimal, error)
}
