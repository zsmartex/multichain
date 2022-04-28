package bitcoin

import (
	"encoding/json"
	"errors"
	"math/rand"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/utils"
	"github.com/zsmartex/multichain/pkg/wallet"
)

type Wallet struct {
	client   *resty.Client
	currency *blockchain.Currency
	wallet   *wallet.SettingWallet
}

func NewWallet() wallet.Wallet {
	return &Wallet{
		client: resty.New(),
	}
}

func (w *Wallet) Configure(settings *wallet.Setting) error {
	w.currency = settings.Currency
	w.wallet = settings.Wallet

	return nil
}

func (b *Wallet) jsonRPC(resp interface{}, method string, params ...interface{}) error {
	type Result struct {
		Version string           `json:"version"`
		ID      int              `json:"id"`
		Result  *json.RawMessage `json:"result"`
		Error   *json.RawMessage `json:"error"`
	}

	response, err := b.client.
		R().
		SetResult(Result{}).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(map[string]interface{}{
			"version": "2.0",
			"id":      rand.Int(),
			"method":  method,
			"params":  params,
		}).Post(b.wallet.URI)

	if err != nil {
		return err
	}

	result := response.Result().(*Result)

	if result.Error != nil {
		return errors.New("jsonRPC error: " + string(*result.Error))
	}

	if result.Result == nil {
		return errors.New("jsonRPC error: result is nil")
	}

	if err := json.Unmarshal(*result.Result, resp); err != nil {
		return err
	}

	return nil
}

func (w *Wallet) CreateAddress() (address, secret string, err error) {
	secret = utils.RandomString(32)

	err = w.jsonRPC(&address, "getnewaddress", secret)

	return
}

func (w *Wallet) CreateTransaction(trans *transaction.Transaction) (transaction *transaction.Transaction, err error) {
	var txid string
	err = w.jsonRPC(&txid, "sendtoaddress", []interface{}{
		trans.ToAddress,
		trans.Amount,
		"",
		"",
		false,
	})

	trans.TxHash = null.StringFrom(txid)

	return trans, err
}

func (w *Wallet) LoadBalance() (balance decimal.Decimal, err error) {
	err = w.jsonRPC(&balance, "getbalance")

	return
}

func (w *Wallet) PrepareDepositCollection(trans *transaction.Transaction, deposit_spreads []*transaction.Transaction, deposit_currency *blockchain.Currency) (*transaction.Transaction, error) {
	return nil, errors.New("failed to prepare deposit collection due: bitcoin client are not supported")
}
