package bitcoin

import (
	"testing"

	"github.com/zsmartex/multichain/pkg/wallet"
)

func newWallet() wallet.Wallet {
	w := NewWallet()
	w.Configure(&wallet.Setting{
		Wallet: &wallet.SettingWallet{
			URI: "https://young-fragrant-sunset.btc.quiknode.pro/",
		},
	})
	return w
}

func TestCreateAddress(t *testing.T) {
	wallet := newWallet()

	address, secret, err := wallet.CreateAddress()
	if err != nil {
		t.Error(err)
	}

	t.Error(address)
	t.Error(secret)
}
