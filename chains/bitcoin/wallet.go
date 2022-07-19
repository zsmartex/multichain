package bitcoin

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"

	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/utils"
	"github.com/zsmartex/multichain/pkg/wallet"
)

type Wallet struct {
	client   *resty.Client
	currency *currency.Currency
	wallet   *wallet.SettingWallet
}

func NewWallet() wallet.Wallet {
	return &Wallet{
		client: resty.New(),
	}
}

func (w *Wallet) Configure(settings *wallet.Setting) {
	if settings.Wallet != nil {
		w.wallet = settings.Wallet
	}

	if settings.Currency != nil {
		w.currency = settings.Currency
	}
}

func (w *Wallet) jsonRPC(ctx context.Context, resp interface{}, method string, params ...interface{}) error {
	type Result struct {
		Version string           `json:"version"`
		ID      int              `json:"id"`
		Result  *json.RawMessage `json:"result"`
		Error   *json.RawMessage `json:"error"`
	}

	response, err := w.client.
		R().
		SetContext(ctx).
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
		}).Post(w.wallet.URI)

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

func (w *Wallet) CreateAddress(ctx context.Context) (address, secret string, err error) {
	secret = utils.RandomString(32)

	err = w.jsonRPC(ctx, &address, "getnewaddress", secret)

	return
}

func (w *Wallet) CreateTransaction(ctx context.Context, tx *transaction.Transaction) (*transaction.Transaction, error) {
	var txid string
	if err := w.jsonRPC(ctx, &txid, "sendtoaddress",
		tx.ToAddress,
		tx.Amount,
		"",
		"",
		false,
	); err != nil {
		return nil, err
	}

	tx.Status = transaction.StatusPending
	tx.TxHash = null.StringFrom(txid)

	return tx, nil
}

func (w *Wallet) LoadBalance(ctx context.Context) (balance decimal.Decimal, err error) {
	var resp [][][]interface{}

	err = w.jsonRPC(ctx, &resp, "listaddressgroupings")
	if err != nil {
		return
	}

	for _, gr := range resp {
		for _, addr := range gr {
			if len(addr) >= 2 {
				if strings.EqualFold(addr[0].(string), w.wallet.Address) {
					balance = balance.Add(decimal.NewFromFloat(addr[1].(float64)))
				}
			}
		}
	}

	return
}

func (w *Wallet) PrepareDepositCollection(context.Context, *transaction.Transaction, []*transaction.Transaction, *currency.Currency) (*transaction.Transaction, error) {
	return nil, errors.New("failed to prepare deposit collection due: bitcoin client are not supported")
}
