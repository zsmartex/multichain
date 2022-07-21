package tron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-resty/resty/v2"
	"github.com/huandu/xstrings"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"

	"github.com/zsmartex/multichain/chains/tron/concerns"
	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/wallet"
)

var defaultTrxFee = map[string]interface{}{
	"fee_limit": 1_000_000,
}

var defaultTrc20Fee = map[string]interface{}{
	"fee_limit": 10_000_000,
}

type Wallet struct {
	client   *resty.Client
	currency *currency.Currency    // selected currency for this wallet
	wallet   *wallet.SettingWallet // selected wallet for this currency
}

func NewWallet() wallet.Wallet {
	return &Wallet{
		client: resty.New(),
	}
}

func (w *Wallet) Configure(settings *wallet.Setting) {
	if settings.Currency != nil {
		w.currency = settings.Currency
	}

	if settings.Wallet != nil {
		w.wallet = settings.Wallet
	}
}

func (w *Wallet) jsonRPC(ctx context.Context, resp interface{}, method string, params interface{}) error {
	type Result struct {
		Error *json.RawMessage `json:"Error,omitempty"`
	}

	response, err := w.client.
		R().
		SetContext(ctx).
		SetResult(Result{}).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(params).
		Post(fmt.Sprintf("%s/%s", w.wallet.URI, method))

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

func (w *Wallet) CreateAddress(ctx context.Context) (address, secret string, err error) {
	type Result struct {
		Address    string `json:"address"`
		PrivateKey string `json:"privateKey"`
	}
	var resp *Result
	err = w.jsonRPC(ctx, &resp, "wallet/generateaddress", nil)
	if err != nil {
		return
	}

	return resp.Address, resp.PrivateKey, err
}

func (w *Wallet) PrepareDepositCollection(ctx context.Context, tx *transaction.Transaction, depositSpreads []*transaction.Transaction, depositCurrency *currency.Currency) (*transaction.Transaction, error) {
	if depositCurrency.Options["trc20_contract_address"] == nil {
		return nil, nil
	}

	options := w.mergeOptions(defaultTrxFee, depositCurrency.Options)

	fees := decimal.NewFromBigInt(big.NewInt(int64(options["fee_limit"].(int))), -w.currency.Subunits)
	amount := fees.Mul(decimal.NewFromInt(int64(len(depositSpreads))))

	tx.Amount = amount

	if tx.Options == nil {
		tx.Options = make(map[string]interface{})
	}

	if options["fee_limit"] != nil {
		tx.Options["fee_limit"] = options["fee_limit"]
	}

	return w.createTrxTransaction(ctx, tx, nil)
}

func (w *Wallet) CreateTransaction(ctx context.Context, tx *transaction.Transaction, options map[string]interface{}) (*transaction.Transaction, error) {
	if w.currency.Options["trc20_contract_address"] != nil {
		return w.createTrc20Transaction(ctx, tx, options)
	} else {
		return w.createTrxTransaction(ctx, tx, options)
	}
}

func (w *Wallet) createTrxTransaction(ctx context.Context, tx *transaction.Transaction, options map[string]interface{}) (*transaction.Transaction, error) {
	options = w.mergeOptions(options, defaultTrxFee, w.currency.Options)

	toAddress, err := concerns.Base58ToAddress(tx.ToAddress)
	if err != nil {
		return nil, err
	}

	amount := w.ConvertToBaseUnit(tx.Amount)
	feeLimit := int64(options["fee_limit"].(int))
	fee := w.ConvertToBaseUnit(decimal.NewFromInt(feeLimit))

	if options["subtract_fee"] != nil {
		if options["subtract_fee"].(bool) {
			amount = amount.Sub(fee)
		}
	}

	var resp *struct {
		Transaction struct {
			TxID string `json:"txID"`
		} `json:"transaction"`
	}

	if err := w.jsonRPC(ctx, &resp, "wallet/easytransferbyprivate", map[string]interface{}{
		"privateKey": w.wallet.Secret,
		"toAddress":  toAddress.Hex(),
		"amount":     amount,
	}); err != nil {
		return nil, err
	}

	tx.Fee = decimal.NewNullDecimal(w.ConvertFromBaseUnit(fee))
	tx.Status = transaction.StatusPending
	tx.TxHash = null.StringFrom(resp.Transaction.TxID)

	return tx, nil
}

func (w *Wallet) createTrc20Transaction(ctx context.Context, tx *transaction.Transaction, options map[string]interface{}) (*transaction.Transaction, error) {
	options = w.mergeOptions(options, defaultTrc20Fee, w.currency.Options)

	signedTxn, err := w.signTransaction(ctx, tx, options)
	if err != nil {
		return nil, err
	}

	feeLimit := int64(options["fee_limit"].(int))
	fee := w.ConvertToBaseUnit(decimal.NewFromInt(feeLimit))

	resp := new(struct {
		Result bool `json:"result"`
	})
	if err := w.jsonRPC(ctx, &resp, "wallet/broadcasttransaction", signedTxn); err != nil || !resp.Result {
		return nil, fmt.Errorf("failed to create trc20 transaction from %s to %s", w.wallet.Address, tx.ToAddress)
	}

	tx.Fee = decimal.NewNullDecimal(w.ConvertFromBaseUnit(fee))
	tx.Status = transaction.StatusPending
	tx.TxHash = null.StringFrom(signedTxn["txID"].(string))

	return tx, nil
}

func (w *Wallet) signTransaction(ctx context.Context, tx *transaction.Transaction, options map[string]interface{}) (map[string]interface{}, error) {
	txn, err := w.triggerSmartContract(ctx, tx, options)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := w.jsonRPC(ctx, &resp, "wallet/gettransactionsign", map[string]interface{}{
		"transaction": txn,
		"privateKey":  w.wallet.Secret,
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (w *Wallet) triggerSmartContract(ctx context.Context, tx *transaction.Transaction, options map[string]interface{}) (json.RawMessage, error) {
	contractAddress, err := concerns.Base58ToAddress(w.currency.Options["trc20_contract_address"].(string))
	if err != nil {
		return nil, err
	}

	ownerAddress, err := concerns.Base58ToAddress(w.wallet.Address)
	if err != nil {
		return nil, err
	}

	toAddress, err := concerns.Base58ToAddress(tx.ToAddress)
	if err != nil {
		return nil, err
	}

	type respResult struct {
		Transaction json.RawMessage `json:"transaction"`
	}

	amount := w.ConvertToBaseUnit(tx.Amount)
	hexAmount := hexutil.EncodeBig(amount.BigInt())
	parameter := xstrings.RightJustify(toAddress.Hex()[2:], 64, "0") + xstrings.RightJustify(strings.TrimLeft(hexAmount, "0x"), 64, "0")

	var result *respResult
	if err := w.jsonRPC(ctx, &result, "wallet/triggersmartcontract", map[string]interface{}{
		"contract_address":  contractAddress.Hex(),
		"function_selector": "transfer(address,uint256)",
		"parameter":         parameter,
		"fee_limit":         options["fee_limit"],
		"owner_address":     ownerAddress.Hex(),
	}); err != nil {
		return nil, err
	}

	return result.Transaction, nil
}

func (w *Wallet) LoadBalance(ctx context.Context) (decimal.Decimal, error) {
	if w.currency.Options["trc20_contract_address"] != nil {
		return w.loadTrc20Balance(ctx)
	} else {
		return w.loadTrxBalance(ctx)
	}
}

func (w *Wallet) loadTrc20Balance(ctx context.Context) (decimal.Decimal, error) {
	contractAddress, err := concerns.Base58ToAddress(w.currency.Options["trc20_contract_address"].(string))
	if err != nil {
		return decimal.Zero, err
	}

	ownerAddress, err := concerns.Base58ToAddress(w.wallet.Address)
	if err != nil {
		return decimal.Zero, err
	}

	var resp *struct {
		ConstantResult []string `json:"constant_result"`
	}

	if err := w.jsonRPC(ctx, &resp, "wallet/triggersmartcontract", map[string]string{
		"owner_address":     ownerAddress.Hex(),
		"contract_address":  contractAddress.Hex(),
		"function_selector": "balanceOf(address)",
		"parameter":         xstrings.RightJustify(ownerAddress.Hex()[2:], 64, "0"),
	}); err != nil {
		return decimal.Zero, err
	}

	b := &big.Int{}
	b.SetString(resp.ConstantResult[0], 16)

	return decimal.NewFromBigInt(b, -w.currency.Subunits), nil
}

func (w *Wallet) loadTrxBalance(ctx context.Context) (decimal.Decimal, error) {
	addressDecoded, err := concerns.Base58ToAddress(w.wallet.Address)
	if err != nil {
		return decimal.Zero, err
	}

	type Result struct {
		Balance decimal.Decimal `json:"balance"`
	}

	var result *Result
	if err := w.jsonRPC(ctx, &result, "wallet/getbalance", map[string]interface{}{
		"address": addressDecoded.Hex(),
	}); err != nil {
		return decimal.Zero, err
	}

	return result.Balance, nil
}

func (w *Wallet) mergeOptions(first map[string]interface{}, steps ...map[string]interface{}) map[string]interface{} {
	if first == nil {
		first = make(map[string]interface{})
	}

	opts := first

	for _, step := range steps {
		for k, v := range step {
			opts[k] = v
		}
	}

	return opts
}

func (w *Wallet) ConvertToBaseUnit(amount decimal.Decimal) decimal.Decimal {
	return amount.Mul(decimal.NewFromInt(int64(math.Pow10(int(w.currency.Subunits)))))
}

func (w *Wallet) ConvertFromBaseUnit(amount decimal.Decimal) decimal.Decimal {
	return amount.Div(decimal.NewFromInt(int64(math.Pow10(int(w.currency.Subunits)))))
}
