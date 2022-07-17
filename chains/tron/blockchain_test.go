package tron

import (
	"context"
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/currency"
)

func newBlockchain() blockchain.Blockchain {
	bl := NewBlockchain()
	bl.Configure(&blockchain.Setting{
		URI: "https://api.shasta.trongrid.io",
		Currencies: []*currency.Currency{
			{
				ID:       "TRX",
				Subunits: 6,
			},
			{
				ID:       "USDT",
				Subunits: 6,
				Options: map[string]interface{}{
					"trc20_contract_address": "TB5NSkyzxkzi3eHW87NwFE6TmtTmnZw61y",
				},
			},
		},
	})

	return bl
}

func TestBlockchain_GetLatestBlockNumber(t *testing.T) {
	bl := newBlockchain()

	blockNumber, err := bl.GetLatestBlockNumber(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(blockNumber)
}

func TestBlockchain_GetBlockByNumber(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByNumber(context.Background(), 25652403)
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestBlockchain_GetBlockByHash(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByHash(context.Background(), "0000000001876cb35a0f2774d2471bfe497d6c08b2857d663d2118262e585814")
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestBlockchain_GetTrxTransaction(t *testing.T) {
	bl := newBlockchain()

	tx, err := bl.GetTransaction(context.Background(), "aefdf111f53d74a3ec6bcb0b601a11feb80c74623ffa508cc13a4d42228a7b74")
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}

func TestBlockchain_GetTrc20Transaction(t *testing.T) {
	bl := newBlockchain()

	tx, err := bl.GetTransaction(context.Background(), "b7d8edcfaa0b665b39e7281e41ea6188b2635822c7fcb22f8dbbcb24dc674484")
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}

func TestBlockchain_GetBalanceOfAddress(t *testing.T) {
	bl := newBlockchain()

	trxBalance, err := bl.GetBalanceOfAddress(context.Background(), "TNFUgrTZ8ks12qNrZqMMBbAq3h7Y7S4DEq", "TRX")
	if err != nil {
		t.Fatal(err)
	}

	trc20Balance, err := bl.GetBalanceOfAddress(context.Background(), "TV7j9ZYUbMAiVjBq1D7WE3nhTFRAnRSYcr", "USDT")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(trxBalance)
	t.Log(trc20Balance)
}
