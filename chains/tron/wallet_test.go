package tron

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
			URI:     "https://api.nileex.io",
			Address: "TKjUc4RaW9q9CXy6PbBmkTTKSBkmGHK5Gm",
			Secret:  "6e85b5700512636db4680e37286e95fee69ae05663c7899edd490ed7de98daf6",
		},
	})

	return w
}

func TestWallet_CreateAddress(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "TRX",
			Subunits: 6,
		},
	})

	address, secret, err := w.CreateAddress(context.Background())
	if err != nil {
		t.Error(err)
	}

	t.Log(address, secret)
}

func TestWallet_CreateTrxTransaction(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "TRX",
			Subunits: 6,
		},
	})

	tx, err := w.CreateTransaction(context.Background(), &transaction.Transaction{
		ToAddress: "TGKFmSijnD6iNLgaf7CbQVysw81MTDbvHq",
		Amount:    decimal.NewFromFloat(30),
	})
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}

func TestWallet_CreateTrc20Transaction(t *testing.T) {
	w := newWallet()

	w.Configure(&wallet.Setting{
		Currency: &currency.Currency{
			ID:       "USDT",
			Subunits: 6,
			Options: map[string]interface{}{
				"trc20_contract_address": "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
			},
		},
	})

	tx, err := w.CreateTransaction(context.Background(), &transaction.Transaction{
		ToAddress:   "TGKFmSijnD6iNLgaf7CbQVysw81MTDbvHq",
		Amount:      decimal.NewFromFloat(30),
		Currency:    "USDT",
		CurrencyFee: "TRX",
	})
	if err != nil {
		t.Error(err)
	}

	t.Log(tx)
}
