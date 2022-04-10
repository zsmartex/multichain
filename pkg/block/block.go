package block

import (
	"github.com/zsmartex/go-peatio/pkg/transaction"
)

type Block struct {
	Hash         string
	Number       int64
	Transactions []*transaction.Transaction
}
