package blockchain

import (
	"github.com/shopspring/decimal"
	"github.com/zsmartex/go-peatio/pkg/block"
	"github.com/zsmartex/go-peatio/pkg/transaction"
)

type BlockchainSettings struct {
	Currencies           []*BlockchainSettingsCurrency
	WhitelistedAddresses []string
}

type BlockchainSettingsCurrency struct {
	ID         string
	BaseFactor int32 // 8 -> 18
	Options    map[string]interface{}
}

type BlockchainConfig struct {
	URI string
}

type Blockchain interface {
	Configure(settings *BlockchainSettings)
	GetLatestBlockNumber() (int64, error)
	GetBlockByHash(hash string) (*block.Block, error)
	GetBlockByNumber(block_number int64) (*block.Block, error)
	GetTransaction(transaction_hash string) ([]*transaction.Transaction, error)
	GetBalanceOfAddress(address string, currency_id string) (decimal.Decimal, error)
}
