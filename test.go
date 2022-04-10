package main

import (
	"log"

	"github.com/zsmartex/multichain/chains/evm"
	"github.com/zsmartex/multichain/pkg/blockchain"
)

func main() {
	address := "0x3cd751e6b0078be393132286c442345e5dc49699"

	b, _ := evm.NewBlockchain(blockchain.BlockchainConfig{
		URI: "https://mainnet.infura.io/v3/846bf642a0e647ad8b2ce35e999d2b57",
	})

	b.Configure(&blockchain.BlockchainSettings{
		Currencies: []*blockchain.BlockchainSettingsCurrency{
			{
				ID:         "usdt",
				BaseFactor: 6,
				Options: map[string]interface{}{
					"erc20_contract_address": "0xdac17f958d2ee523a2206206994597c13d831ec7",
				},
			},
			{
				ID:         "eth",
				BaseFactor: 18,
			},
		},
	})

	log.Println(b.GetBalanceOfAddress(address, "usdt"))
	log.Println(b.GetBalanceOfAddress(address, "eth"))
}
