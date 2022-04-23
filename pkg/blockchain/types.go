package blockchain

import (
	"github.com/shopspring/decimal"
	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type Currency struct {
	ID         string
	BaseFactor int32 // 8 -> 18
	Options    map[string]string
}

type Settings struct {
	Currencies           []*Currency
	WhitelistedAddresses []string
	URI                  string
}

type Blockchain interface {
	Configure(settings *Settings) error
	GetLatestBlockNumber() (int64, error)
	GetBlockByHash(hash string) (*block.Block, error)
	GetBlockByNumber(block_number int64) (*block.Block, error)
	GetTransaction(transaction_hash string) (*transaction.Transaction, error)
	GetBalanceOfAddress(address string, currency_id string) (decimal.Decimal, error)
}
