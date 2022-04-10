package block

import (
	"github.com/zsmartex/multichain/pkg/transaction"
)

type Block struct {
	Hash         string
	Number       int64
	Transactions []*transaction.Transaction
}
