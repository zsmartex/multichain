package evm

import (
	"context"
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
)

func TestBlockchain_GetBlockByNumber(t *testing.T) {
	bl := NewBlockchain()

	bl.Configure(&blockchain.Settings{
		URI: "http://localhost:8545",
	})

	block, err := bl.GetBlockByNumber(context.Background(), 1)
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}
