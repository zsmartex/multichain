package evm

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"

	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/utils"
	"github.com/zsmartex/multichain/pkg/wallet"
)

type options struct {
	GasLimit uint64
	GasPrice uint64
	GasRate  wallet.GasPriceRate
}

var defaultEvmFee = options{
	GasLimit: 21_000,
	GasRate:  wallet.GasPriceRateStandard,
}

var defaultErc20Fee = options{
	GasLimit: 90_000,
	GasRate:  wallet.GasPriceRateStandard,
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
		}).
		Post(w.wallet.URI)

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

	err = w.jsonRPC(ctx, &address, "personal_newAccount", secret)

	return
}

// PrepareDepositCollection this func don't execute create transaction just return transaction was built
func (w *Wallet) PrepareDepositCollection(ctx context.Context, depositTransaction *transaction.Transaction, depositSpreads []*transaction.Transaction, depositCurrency *currency.Currency) (*transaction.Transaction, error) {
	if depositCurrency.Options["erc20_contract_address"] == nil {
		return nil, nil
	}

	options := w.mergeOptions(defaultEvmFee, depositCurrency.Options)

	gasPrice, err := w.calculateGasPrice(ctx, options.GasRate)
	if err != nil {
		return nil, err
	}

	fees := decimal.NewFromBigInt(big.NewInt(int64(options.GasLimit*gasPrice)), -w.currency.Subunits)
	amount := fees.Mul(decimal.NewFromInt(int64(len(depositSpreads))))

	depositTransaction.Amount = amount

	if depositTransaction.Options == nil {
		depositTransaction.Options = make(map[string]interface{})
	}

	if options.GasLimit != 0 {
		depositTransaction.Options["gas_limit"] = options.GasLimit
	}

	if options.GasPrice != 0 {
		depositTransaction.Options["gas_price"] = options.GasPrice
	}

	return depositTransaction, nil
}

func (w *Wallet) CreateTransaction(ctx context.Context, transaction *transaction.Transaction) (*transaction.Transaction, error) {
	if len(w.ContractAddress()) > 0 {
		return w.createErc20Transaction(ctx, transaction)
	} else {
		return w.createEvmTransaction(ctx, transaction)
	}
}

func (w *Wallet) createEvmTransaction(ctx context.Context, transaction *transaction.Transaction) (t *transaction.Transaction, err error) {
	options := w.mergeOptions(defaultEvmFee, w.currency.Options)

	if options.GasPrice == 0 {
		gp, err := w.calculateGasPrice(ctx, options.GasRate)
		if err != nil {
			return nil, err
		}

		options.GasPrice = gp
	}

	subunits := decimal.NewFromInt(int64(math.Pow10(int(w.currency.Subunits))))
	amount := transaction.Amount.Mul(subunits)

	var txid string
	err = w.jsonRPC(ctx, &txid, "personal_sendTransaction", map[string]string{
		"from":     w.normalizeAddress(w.wallet.Address),
		"to":       w.normalizeAddress(transaction.ToAddress),
		"value":    hexutil.EncodeBig(amount.BigInt()),
		"gas":      hexutil.EncodeUint64(options.GasLimit),
		"gasPrice": hexutil.EncodeUint64(options.GasPrice),
	}, w.wallet.Secret)
	if err != nil {
		return nil, err
	}

	transaction.TxHash = null.StringFrom(txid)

	return transaction, nil
}

func (w *Wallet) createErc20Transaction(ctx context.Context, transaction *transaction.Transaction) (*transaction.Transaction, error) {
	options := w.mergeOptions(defaultErc20Fee, w.currency.Options)

	if options.GasPrice == 0 {
		gp, err := w.calculateGasPrice(ctx, options.GasRate)
		if err != nil {
			return nil, err
		}

		options.GasPrice = gp
	}

	subunits := decimal.NewFromInt(int64(math.Pow10(int(w.currency.Subunits))))
	amount := transaction.Amount.Mul(subunits)

	abiJSON, err := abi.JSON(strings.NewReader(abiDefinition))
	if err != nil {
		return nil, err
	}

	data, err := abiJSON.Pack("transfer", common.HexToAddress(w.normalizeAddress(transaction.ToAddress)), amount.BigInt())
	if err != nil {
		return nil, err
	}

	var txid string
	err = w.jsonRPC(ctx, &txid, "personal_sendTransaction", map[string]string{
		"from":     w.normalizeAddress(w.wallet.Address),
		"to":       w.ContractAddress(), // to contract address
		"data":     hexutil.Encode(data),
		"gas":      hexutil.EncodeUint64(options.GasLimit),
		"gasPrice": hexutil.EncodeUint64(options.GasPrice),
	}, w.wallet.Secret)
	if err != nil {
		return nil, err
	}

	transaction.TxHash = null.StringFrom(txid)

	return transaction, nil
}

func (w *Wallet) normalizeAddress(address string) string {
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	return strings.ToLower(address)
}

func (w *Wallet) ContractAddress() string {
	if w.currency.Options["erc20_contract_address"] != nil {
		return w.currency.Options["erc20_contract_address"].(string)
	} else {
		return ""
	}
}

func (w *Wallet) calculateGasPrice(ctx context.Context, gas_rate wallet.GasPriceRate) (gas_price uint64, err error) {
	var result string
	err = w.jsonRPC(ctx, &result, "eth_gasPrice")
	if err != nil {
		return
	}

	var rate float64
	switch gas_rate {
	case wallet.GasPriceRateFast:
		rate = 1.1
	default:
		rate = 1
	}

	var gp uint64
	gp, err = hexutil.DecodeUint64(result)

	return gp * uint64(rate), err
}

func (w *Wallet) LoadBalance(ctx context.Context) (balance decimal.Decimal, err error) {
	if len(w.ContractAddress()) > 0 {
		return w.loadBalanceErc20Balance(ctx, w.wallet.Address)
	} else {
		return w.loadBalanceEvmBalance(ctx, w.wallet.Address)
	}
}

func (w *Wallet) loadBalanceEvmBalance(ctx context.Context, address string) (balance decimal.Decimal, err error) {
	var result string
	err = w.jsonRPC(ctx, &result, "eth_getBalance", address, "latest")
	if err != nil {
		return
	}

	return w.hexToDecimal(result)
}

func (w *Wallet) loadBalanceErc20Balance(ctx context.Context, address string) (balance decimal.Decimal, err error) {
	abiJSON, err := abi.JSON(strings.NewReader(abiDefinition))
	if err != nil {
		return decimal.Zero, err
	}

	data, err := abiJSON.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return decimal.Zero, err
	}

	var result string
	if err := w.jsonRPC(ctx, &result, "eth_call", map[string]string{"to": w.ContractAddress(), "data": hexutil.Encode(data)}, "latest"); err != nil {
		return decimal.Zero, err
	}

	return w.hexToDecimal(result)
}

func (w *Wallet) hexToDecimal(hex string) (decimal.Decimal, error) {
	hex = "0x" + strings.TrimLeft(strings.TrimLeft(hex, "0x"), "0")

	b, err := hexutil.DecodeBig(hex)
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(b, -w.currency.Subunits), nil
}

func (w *Wallet) mergeOptions(first options, step map[string]interface{}) options {
	opts := first

	if step == nil {
		return opts
	}

	if step["gas_price"] != nil {
		opts.GasPrice = step["gas_price"].(uint64)
	}

	if step["gas_limit"] != nil {
		opts.GasLimit = step["gas_limit"].(uint64)
	}

	if step["gas_rate"] != nil {
		opts.GasRate = step["gas_rate"].(wallet.GasPriceRate)
	}

	return opts
}
