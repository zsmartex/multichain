package bitcoin

import (
	"testing"

	"github.com/zsmartex/multichain/pkg/blockchain"
)

func newBlockchain() blockchain.Blockchain {
	bl := NewBlockchain()
	bl.Configure(&blockchain.Settings{
		Currencies: []*blockchain.Currency{
			{
				ID:       "btc",
				SubUnits: 0,
			},
		},
		URI: "https://young-fragrant-sunset.btc.quiknode.pro/",
	})

	return bl
}

func TestGetLatestBlockNumber(t *testing.T) {
	bl := newBlockchain()

	latest_block_number, err := bl.GetLatestBlockNumber()
	if err != nil {
		t.Error(err)
	}

	t.Log(latest_block_number)
}

func TestGetBlockByNumber(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByNumber(120000)
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestGetBlockByHash(t *testing.T) {
	bl := newBlockchain()

	block, err := bl.GetBlockByHash("00000000000000000009a5308fc443d1fde1b624a9dd2bb9ab9a902e0ed0909d")
	if err != nil {
		t.Error(err)
	}

	t.Log(block)
}

func TestGetBalanceOfAddress(t *testing.T) {
	bl := newBlockchain()

	balance, err := bl.GetBalanceOfAddress("1GNgwA8JfG7Kc8akJ8opdNWJUihqUztfPe", "btc")
	if err != nil {
		t.Error(err)
	}

	t.Log(balance)
}

func TestGetTransaction(t *testing.T) {
	bl := newBlockchain()

	txs, err := bl.GetTransaction("3a7b085a1f463f32f5f568c3e9340fc15275605936d6293e396db75935c6a185")
	if err != nil {
		t.Error(err)
	}

	for _, tx := range txs {
		t.Error(tx)
	}
}
