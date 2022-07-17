package evm

import (
	"context"
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/currency"
)

func newBlockchain() blockchain.Blockchain {
	bl := NewBlockchain()

	bl.Configure(&blockchain.Setting{
		URI: "http://65.108.75.172:8575",
		Currencies: []*currency.Currency{
			{
				ID:       "BSC",
				Subunits: 18,
			},
			{
				ID:       "USDT",
				Subunits: 18,
				Options: map[string]interface{}{
					"erc20_contract_address": "0x337610d27c682e347c9cd60bd4b3b107c9d34ddd",
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

	block, err := bl.GetBlockByNumber(context.Background(), 21061227)
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestBlockchain_GetBlockByHash(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByHash(context.Background(), "0xf392b3a6808bde337a299ec1e6aaacbf93de86bab8e1a4dcdc3aeaa6e742a41b")
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestBlockchain_GetTransaction(t *testing.T) {
	bl := newBlockchain()

	// EVM Transaction
	txEvm, err := bl.GetTransaction(context.Background(), "0x8c7e11005dcab3048e0ec3bc8b13ab76d5fe3b9261d0410fe431c5e20641fe7c")
	if err != nil {
		t.Error(err)
	}

	t.Log("EVM Transaction: ", txEvm)

	// ERC20 Transaction
	txErc20, err := bl.GetTransaction(context.Background(), "0x56a12c1101e7d3916047ac969f71e121565ada81dcae3d6976e3d32bb9b4b11b")

	t.Log("ERC20 Transaction: ", txErc20)

	// TODO: add support for ERC20 transactions
}

func TestBlockchain_GetBalanceOfAddress(t *testing.T) {
	bl := newBlockchain()

	balance, err := bl.GetBalanceOfAddress(context.Background(), "0xF37111De2f6AE2f64Be1e59472b5C50801540C8c", "ETH")
	if err != nil {
		t.Error(err)
	}

	t.Log(balance)
}
