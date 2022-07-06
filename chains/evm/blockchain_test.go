package evm

import (
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
)

func newBlockchain() blockchain.Blockchain {
	bl := NewBlockchain()
	bl.Configure(&blockchain.Settings{
		URI: "https://mainnet.infura.io/v3/846bf642a0e647ad8b2ce35e999d2b57",
		Currencies: []*blockchain.Currency{
			{
				ID:       "eth",
				SubUnits: 18,
			},
			{
				ID:       "usdt",
				SubUnits: 6,
				Options: map[string]interface{}{
					"erc20_contract_address": "0xdac17f958d2ee523a2206206994597c13d831ec7",
				},
			},
		},
	})

	return bl
}

func TestGetBlock(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByNumber(4000000)
	if err != nil {
		t.Error(err)
	}

	t.Error(block)
}

func TestGetEvmTransaction(t *testing.T) {
	bl := newBlockchain()

	txs, err := bl.GetTransaction("0xe8f87a93b067a8b5af7a2cf9fe36614fd46c69da5704d43eed31c3d3ab148d12")
	if err != nil {
		t.Error(err)
	}

	t.Error(txs[0])
}

func TestGetErc20Transaction(t *testing.T) {
	bl := newBlockchain()

	txs, err := bl.GetTransaction("0x8ac459c59568f4cc7b1944437feedbeb547da160f24623f6b0a887446b47dfb6")
	if err != nil {
		t.Error(err)
	}

	t.Error(txs[0])
}
