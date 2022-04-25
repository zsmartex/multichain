package tron

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/huandu/xstrings"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
	"github.com/zsmartex/multichain/chains/tron/concerns"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/wallet"
)

type Wallet struct {
	client   *resty.Client
	currency *blockchain.Currency  // selected currency for this wallet
	wallet   *wallet.SettingWallet // selected wallet for this currency
}

func NewWallet(currency *blockchain.Currency) wallet.Wallet {
	return &Wallet{
		client: resty.New(),
	}
}

func (w *Wallet) Configure(settings *wallet.Setting) error {
	w.currency = settings.Currency
	w.wallet = settings.Wallet

	return nil
}

func (w *Wallet) jsonRPC(resp interface{}, method string, params interface{}) error {
	type Result struct {
		Error *json.RawMessage `json:"Error,omitempty"`
	}

	response, err := w.client.
		R().
		SetResult(Result{}).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(params).
		Post(w.wallet.URI)

	if err != nil {
		return err
	}

	result := response.Result().(*Result)

	if result.Error != nil {
		return errors.New("jsonRPC error: " + string(*result.Error))
	}

	if err := json.Unmarshal(response.Body(), resp); err != nil {
		return err
	}

	return nil
}

func (w *Wallet) CreateAddress() (address, secret string, err error) {
	type Result struct {
		Address    string `json:"address"`
		PrivateKey string `json:"privateKey"`
	}
	var resp *Result
	err = w.jsonRPC(&resp, "wallet/generateaddress", nil)

	return resp.Address, resp.PrivateKey, err
}

func (w *Wallet) PrepareDepositCollection(trans *transaction.Transaction, deposit_spreads []*transaction.Transaction, deposit_currency *blockchain.Currency) ([]*transaction.Transaction, error) {
	if len(deposit_currency.Options["trc10_asset_id"]) == 0 && len(deposit_currency.Options["trc20_contract_address"]) == 0 {
		return []*transaction.Transaction{}, nil
	}

	if len(deposit_spreads) == 0 {
		return []*transaction.Transaction{}, nil
	}

	fee_limit, err := strconv.ParseInt(trans.Options["fee_limit"], 10, 64)
	if err != nil {
		return nil, err
	}

	fees := decimal.NewFromBigInt(big.NewInt(fee_limit), -w.currency.BaseFactor)
	amount := fees.Mul(decimal.NewFromInt(int64(len(deposit_spreads))))

	trans.Amount = amount

	transactions := make([]*transaction.Transaction, 0)
	t, err := w.createTrxTransaction(trans)
	if err != nil {
		return nil, err
	}

	transactions = append(transactions, t)

	return transactions, nil
}

func (w *Wallet) CreateTransaction(tx *transaction.Transaction) (*transaction.Transaction, error) {
	if len(w.currency.Options["trc20_contract_address"]) > 0 {
		return w.createTrc20Transaction(tx)
	} else if len(w.currency.Options["trc10_asset_id"]) > 0 {
		return w.createTrc10Transaction(tx)
	} else {
		return w.createTrxTransaction(tx)
	}
}

func (w *Wallet) createTrxTransaction(tx *transaction.Transaction) (*transaction.Transaction, error) {
	to_address, err := concerns.DecodeAddress(tx.ToAddress)
	if err != nil {
		return nil, err
	}

	amount := tx.Amount.Mul(decimal.NewFromInt(int64(math.Pow10(int(w.currency.BaseFactor)))))

	var resp *struct {
		Transaction struct {
			TxID string `json:"txID"`
		} `json:"transaction"`
	}

	if err := w.jsonRPC(&resp, "wallet/easytransferassetbyprivate", map[string]interface{}{
		"privateKey": w.wallet.Secret,
		"toAddress":  to_address,
		"amount":     amount,
	}); err != nil {
		return nil, err
	}

	tx.TxHash = null.StringFrom(resp.Transaction.TxID)

	return tx, nil
}

func (w *Wallet) createTrc10Transaction(tx *transaction.Transaction) (*transaction.Transaction, error) {
	to_address, err := concerns.DecodeAddress(tx.ToAddress)
	if err != nil {
		return nil, err
	}

	amount := tx.Amount.Mul(decimal.NewFromInt(int64(math.Pow10(int(w.currency.BaseFactor)))))

	var resp *struct {
		Transaction struct {
			TxID string `json:"txID"`
		} `json:"transaction"`
	}

	if err := w.jsonRPC(&resp, "wallet/easytransferassetbyprivate", map[string]interface{}{
		"privateKey": w.wallet.Secret,
		"toAddress":  to_address,
		"assetId":    w.currency.Options["trc10_asset_id"],
		"amount":     amount,
	}); err != nil {
		return nil, err
	}

	tx.TxHash = null.StringFrom(resp.Transaction.TxID)

	return tx, nil
}

func (w *Wallet) createTrc20Transaction(tx *transaction.Transaction) (*transaction.Transaction, error) {
	signed_txn, err := w.signTransaction(tx)
	if err != nil {
		return nil, err
	}

	resp := new(struct {
		Result bool `json:"result"`
	})
	if err := w.jsonRPC(&resp, "wallet/broadcasttransaction", signed_txn); err != nil || !resp.Result {
		return nil, fmt.Errorf("failed to create trc20 transaction from %s to %s", tx.FromAddress, tx.ToAddress)
	}

	tx.TxHash = null.StringFrom(signed_txn["txID"].(string))

	return tx, nil
}

func (w *Wallet) signTransaction(tx *transaction.Transaction) (map[string]interface{}, error) {
	txn, err := w.triggerSmartContract(tx)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := w.jsonRPC(&resp, "wallet/gettransactionsign", map[string]interface{}{
		"transaction": txn,
		"privateKey":  w.wallet.Secret,
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (w *Wallet) triggerSmartContract(tx *transaction.Transaction) (json.RawMessage, error) {
	contract_address, err := concerns.DecodeAddress(w.currency.Options["trc20_contract_address"])
	if err != nil {
		return nil, err
	}

	owner_address, err := concerns.DecodeAddress(tx.FromAddress)
	if err != nil {
		return nil, err
	}

	type respResult struct {
		Transaction json.RawMessage `json:"transaction"`
	}

	sub_units := decimal.NewFromInt(int64(math.Pow10(int(w.currency.BaseFactor))))

	var result *respResult
	if err := w.jsonRPC(&result, "wallet/triggersmartcontract", map[string]string{
		"contract_address":  contract_address,
		"function_selector": "transfer(address,uint256)",
		"parameter":         xstrings.RightJustify(owner_address[2:], 64, "0") + xstrings.RightJustify(tx.Amount.Mul(sub_units).String(), 64, "0"),
		"fee_limit":         w.currency.Options["fee_limit"],
		"owner_address":     owner_address,
	}); err != nil {
		return nil, err
	}

	return result.Transaction, nil
}

func (w *Wallet) LoadBalance() (decimal.Decimal, error) {
	if len(w.currency.Options["trc20_contract_address"]) > 0 {
		return w.loadTrc20Balance()
	} else if len(w.currency.Options["trc10_asset_id"]) > 0 {
		return w.loadTrc10Balance()
	} else {
		return w.loadTrxBalance()
	}
}

func (w *Wallet) loadTrc20Balance() (decimal.Decimal, error) {
	contract_address, err := concerns.DecodeAddress(w.currency.Options["trc20_contract_address"])
	if err != nil {
		return decimal.Zero, err
	}

	owner_address, err := concerns.DecodeAddress(w.wallet.Address)
	if err != nil {
		return decimal.Zero, err
	}

	var resp *struct {
		ConstantResult []string `json:"constant_result"`
	}

	if err := w.jsonRPC(&resp, "wallet/triggersmartcontract", map[string]string{
		"owner_address":     owner_address,
		"contract_address":  contract_address,
		"function_selector": "balanceOf(address)",
		"parameter":         xstrings.RightJustify(owner_address[2:], 64, "0"),
	}); err != nil {
		return decimal.Zero, err
	}

	b := &big.Int{}
	b.SetString(resp.ConstantResult[0], 16)

	return decimal.NewFromBigInt(b, -w.currency.BaseFactor), nil
}

func (w *Wallet) loadTrc10Balance() (decimal.Decimal, error) {
	address_decoded, err := concerns.DecodeAddress(w.wallet.Address)
	if err != nil {
		return decimal.Zero, err
	}

	type Result struct {
		AssetV2 []struct {
			Key   string `json:"key"`
			Value decimal.Decimal
		} `json:"assetV2"`
	}

	var result *Result
	if err := w.jsonRPC(&result, "wallet/getbalance", map[string]interface{}{
		"address": address_decoded,
	}); err != nil {
		return decimal.Zero, err
	}

	if result.AssetV2 == nil {
		return decimal.Zero, nil
	}

	balance := decimal.Zero

	for _, asset := range result.AssetV2 {
		if asset.Key == w.currency.Options["trc10_asset_id"] {
			balance = asset.Value
			break
		}
	}

	return balance, nil
}

func (w *Wallet) loadTrxBalance() (decimal.Decimal, error) {
	address_decoded, err := concerns.DecodeAddress(w.wallet.Address)
	if err != nil {
		return decimal.Zero, err
	}

	type Result struct {
		Balance decimal.Decimal `json:"balance"`
	}

	var result *Result
	if err := w.jsonRPC(&result, "wallet/getbalance", map[string]interface{}{
		"address": address_decoded,
	}); err != nil {
		return decimal.Zero, err
	}

	return result.Balance, nil
}
