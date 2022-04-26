package evm

import (
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
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
	"github.com/zsmartex/multichain/pkg/utils"
	"github.com/zsmartex/multichain/pkg/wallet"
)

type options struct {
	GasLimit uint64
	GasPrice uint64
	GasRate  wallet.GasPriceRate
}

var default_evm_fee = options{
	GasLimit: 21_000,
	GasRate:  wallet.GasPriceRateStandard,
}

var default_erc20_fee = options{
	GasLimit: 90_000,
	GasRate:  wallet.GasPriceRateStandard,
}

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

func (w *Wallet) jsonRPC(resp interface{}, method string, params ...interface{}) error {
	type Result struct {
		Version string           `json:"version"`
		ID      int              `json:"id"`
		Result  *json.RawMessage `json:"result"`
		Error   *json.RawMessage `json:"error"`
	}

	response, err := w.client.
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

func (w *Wallet) CreateAddress() (address, secret string, err error) {
	secret = utils.RandomString(32)

	err = w.jsonRPC(&address, "personal_newAccount", secret)

	return
}

func (w *Wallet) PrepareDepositCollection(deposit_transaction *transaction.Transaction, deposit_spreads []*transaction.Transaction, deposit_currency *blockchain.Currency) ([]*transaction.Transaction, error) {
	if len(deposit_currency.Options["erc20_contract_address"].(string)) == 0 {
		return []*transaction.Transaction{}, nil
	}

	if len(deposit_spreads) == 0 {
		return []*transaction.Transaction{}, nil
	}

	options := w.merege_options(default_erc20_fee, deposit_currency.Options)

	gas_price, err := w.calculate_gas_price(options.GasRate)
	if err != nil {
		return nil, err
	}

	options.GasPrice = gas_price

	fees := decimal.NewFromBigInt(big.NewInt(int64(options.GasLimit*gas_price)), -w.currency.BaseFactor)

	amount := fees.Mul(decimal.NewFromInt(int64(len(deposit_spreads))))

	deposit_transaction.Amount = amount
	deposit_transaction.Options["gas_limit"] = options.GasLimit
	deposit_transaction.Options["gas_price"] = options.GasPrice

	transactions := make([]*transaction.Transaction, 0)

	t, err := w.createEvmTransaction(deposit_transaction)
	if err != nil {
		return nil, err
	}

	transactions = append(transactions, t)

	return transactions, nil
}

func (w *Wallet) CreateTransaction(transaction *transaction.Transaction) (*transaction.Transaction, error) {
	if len(w.currency.Options["erc20_contract_address"].(string)) > 0 {
		return w.createErc20Transaction(transaction)
	} else {
		return w.createEvmTransaction(transaction)
	}
}

func (w *Wallet) createEvmTransaction(transaction *transaction.Transaction) (t *transaction.Transaction, err error) {
	options := w.merege_options(default_evm_fee, w.currency.Options)

	if options.GasPrice == 0 {
		gp, err := w.calculate_gas_price(options.GasRate)
		if err != nil {
			return nil, err
		}

		options.GasPrice = gp
	}

	sub_units := decimal.NewFromInt(int64(math.Pow10(int(w.currency.BaseFactor))))
	amount := transaction.Amount.Mul(sub_units)

	var txid string
	err = w.jsonRPC(&txid, "personal_sendTransaction", map[string]string{
		"from":     transaction.FromAddress,
		"to":       transaction.ToAddress,
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

func (w *Wallet) createErc20Transaction(transaction *transaction.Transaction) (*transaction.Transaction, error) {
	contract_address := w.currency.Options["erc20_contract_address"].(string)
	options := w.merege_options(default_evm_fee, w.currency.Options)

	if options.GasPrice == 0 {
		gp, err := w.calculate_gas_price(options.GasRate)
		if err != nil {
			return nil, err
		}

		options.GasPrice = gp
	}

	sub_units := decimal.NewFromInt(int64(math.Pow10(int(w.currency.BaseFactor))))
	amount := transaction.Amount.Mul(sub_units)

	abi, err := abi.JSON(strings.NewReader(abiDefinition))
	if err != nil {
		return nil, err
	}

	data, err := abi.Pack("transfer", common.HexToAddress(transaction.ToAddress), hexutil.EncodeBig(amount.BigInt()))
	if err != nil {
		return nil, err
	}

	var txid string
	err = w.jsonRPC(&txid, "personal_sendTransaction", map[string]string{
		"from":     transaction.FromAddress,
		"to":       contract_address, // to contract address
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

func (w *Wallet) calculate_gas_price(gas_rate wallet.GasPriceRate) (gas_price uint64, err error) {
	var result string
	err = w.jsonRPC(&result, "eth_gasPrice")
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

	return (gp * uint64(rate)), err
}

func (w *Wallet) LoadBalance() (balance decimal.Decimal, err error) {
	if len(w.currency.Options["erc20_contract_address"].(string)) > 0 {
		return w.loadBalanceEvmBalance(w.wallet.Address)
	} else {
		return w.loadBalanceErc20Balance(w.wallet.Address)
	}
}

func (w *Wallet) loadBalanceEvmBalance(address string) (balance decimal.Decimal, err error) {
	err = w.jsonRPC(&balance, "eth_getBalance", address, "latest")

	return
}

func (w *Wallet) loadBalanceErc20Balance(address string) (balance decimal.Decimal, err error) {
	contract_address := w.currency.Options["erc20_contract_address"].(string)

	abi, err := abi.JSON(strings.NewReader(abiDefinition))
	if err != nil {
		return decimal.Zero, err
	}

	data, err := abi.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return decimal.Zero, err
	}

	var result string
	w.jsonRPC(&result, "eth_call", map[string]string{
		"to":   contract_address,
		"data": hexutil.Encode(data),
	})

	b, err := hexutil.DecodeBig(result)
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(b, -w.currency.BaseFactor), nil
}

func (w *Wallet) merege_options(first options, step map[string]interface{}) options {
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
