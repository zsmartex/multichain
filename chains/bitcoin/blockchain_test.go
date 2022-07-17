package bitcoin

import (
	"context"
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/currency"
)

func newBlockchain() blockchain.Blockchain {
	bl := NewBlockchain()
	bl.Configure(&blockchain.Setting{
		URI: "https://young-fragrant-sunset.btc.quiknode.pro",
		Currencies: []*currency.Currency{
			{
				ID:       "BTC",
				Subunits: 8,
			},
		},
	})

	return bl
}

func TestBlockchain_GetBlockByNumber(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByNumber(context.Background(), 100001)
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestBlockchain_GetTransaction(t *testing.T) {
	bl := newBlockchain()

	tx, err := bl.GetTransaction(context.Background(), "f22279937740708024052bd8c3decf01a406a6f33760e3e10b8e4436826f0e59")
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}
