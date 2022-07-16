package bitcoin

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/wallet"
)

func TestWallet_CreateAddress(t *testing.T) {
	w := NewWallet()

	w.Configure(&wallet.Setting{
		Wallet: &wallet.SettingWallet{
			URI: "http://user:password@localhost:18443",
		},
		Currency: &currency.Currency{
			ID:       "BTC",
			Subunits: 8,
		},
	})

	address, secret, err := w.CreateAddress(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(address, secret)
}

func TestWallet_LoadBalance(t *testing.T) {
	w := NewWallet()

	w.Configure(&wallet.Setting{
		Wallet: &wallet.SettingWallet{
			URI:     "http://user:password@localhost:18443",
			Address: "bcrt1qqqd8hdc684cqpm5ydfd535eygxlmh54wysmzry",
			Secret:  "",
		},
		Currency: &currency.Currency{
			ID:       "BTC",
			Subunits: 8,
		},
	})

	balance, err := w.LoadBalance(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(balance)
}

func TestWallet_CreateTransaction(t *testing.T) {
	w := NewWallet()

	w.Configure(&wallet.Setting{
		Wallet: &wallet.SettingWallet{
			URI:     "http://user:password@localhost:18443",
			Address: "mwjUmhAW68zCtgZpW5b1xD5g7MZew6xPV4",
			Secret:  "",
		},
		Currency: &currency.Currency{
			ID:       "BTC",
			Subunits: 8,
		},
	})

	tx, err := w.CreateTransaction(context.Background(), &transaction.Transaction{
		ToAddress: "bcrt1qqqd8hdc684cqpm5ydfd535eygxlmh54wysmzry",
		Amount:    decimal.NewFromFloat(0.1),
		Currency:  "BTC",
	})
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}
