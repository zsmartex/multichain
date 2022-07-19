package evm

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/wallet"
)

func newWallet() wallet.Wallet {
	w := NewWallet()

	w.Configure(&wallet.Setting{
		Wallet: &wallet.SettingWallet{
			URI:     "http://65.108.75.172:8575",
			Address: "0x249aeb18f3a323c12334a595cb6220912c4b9087",
			Secret:  "AHyDK2saIAQlDmCiPoteP3uGBUgBzLG0",
		},
	})

	return w
}

func TestWallet_CreateAddress(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "ETH",
			Subunits: 18,
		},
	})

	address, secret, err := w.CreateAddress(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(address, secret)
}

func TestWallet_LoadEVMBalance(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "ETH",
			Subunits: 18,
		},
	})

	balance, err := w.LoadBalance(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(balance)
}

func TestWallet_LoadErc20Balance(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "USDT",
			Subunits: 18,
			Options: map[string]interface{}{
				"erc20_contract_address": "0x337610d27c682e347c9cd60bd4b3b107c9d34ddd",
			},
		},
	})

	balance, err := w.LoadBalance(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(balance)
}

func TestWallet_CreateEVMTransaction(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "ETH",
			Subunits: 18,
		},
	})

	tx, err := w.CreateTransaction(context.Background(), &transaction.Transaction{
		ToAddress: "0xF37111De2f6AE2f64Be1e59472b5C50801540C8c",
		Amount:    decimal.NewFromFloat(0.001),
		Currency:  "ETH",
	})
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}

func TestWallet_CreateErc20Transaction(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "USDT",
			Subunits: 18,
			Options: map[string]interface{}{
				"erc20_contract_address": "0x337610d27c682e347c9cd60bd4b3b107c9d34ddd",
			},
		},
	})

	tx, err := w.CreateTransaction(context.Background(), &transaction.Transaction{
		ToAddress: "0xF37111De2f6AE2f64Be1e59472b5C50801540C8c",
		Amount:    decimal.NewFromFloat(0.01),
	})
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}

func TestWallet_PrepareDepositCollection(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "ETH",
			Subunits: 18,
		},
	})

	depositSpreadCollectionTx, err := w.PrepareDepositCollection(
		context.Background(),
		&transaction.Transaction{
			Currency:    "ETH",
			CurrencyFee: "ETH",
			Amount:      decimal.NewFromFloat(10),
		},
		[]*transaction.Transaction{
			{
				Currency:    "USDT",
				CurrencyFee: "ETH",
				ToAddress:   "0xF37111De2f6AE2f64Be1e59472b5C50801540C8c",
				Amount:      decimal.NewFromFloat(10),
			},
		},
		&currency.Currency{
			ID:       "USDT",
			Subunits: 18,
			Options: map[string]interface{}{
				"erc20_contract_address": "0x337610d27c682e347c9cd60bd4b3b107c9d34ddd",
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	t.Log(depositSpreadCollectionTx)
}
