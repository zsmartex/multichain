package bitcoin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"

	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type VOut struct {
	Value        decimal.Decimal `json:"value"`
	N            int64           `json:"n"`
	ScriptPubKey *struct {
		Addresses []string `json:"addresses"`
	} `json:"scriptPubKey"`
}

type Vin struct {
	TxID string `json:"txid"`
	VOut int64  `json:"vout"`
}

type TxHash struct {
	TxID string  `json:"txid"`
	Vin  []*Vin  `json:"vin"`
	VOut []*VOut `json:"vout"`
}

type Block struct {
	Hash          string    `json:"hash"`
	Confirmations int       `json:"confirmations"`
	Size          int       `json:"size"`
	Height        int64     `json:"height"`
	Version       int       `json:"version"`
	MerkleRoot    string    `json:"merkleroot"`
	Tx            []*TxHash `json:"tx"`
}

type Blockchain struct {
	currency *currency.Currency
	setting  *blockchain.Setting
	client   *resty.Client
}

func NewBlockchain() blockchain.Blockchain {
	return &Blockchain{
		client: resty.New(),
	}
}

func (b *Blockchain) Configure(settings *blockchain.Setting) {
	b.setting = settings

	for _, c := range settings.Currencies {
		// allow only one currency
		b.currency = c
		break
	}
}

func (b *Blockchain) jsonRPC(ctx context.Context, resp interface{}, method string, params ...interface{}) error {
	type Result struct {
		Version string           `json:"version"`
		ID      int              `json:"id"`
		Result  *json.RawMessage `json:"result"`
		Error   *json.RawMessage `json:"error"`
	}

	response, err := b.client.
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
		}).Post(b.setting.URI)

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

func (b *Blockchain) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	var resp int64
	if err := b.jsonRPC(ctx, &resp, "getblockcount"); err != nil {
		return 0, err
	}

	return resp, nil
}

func (b *Blockchain) GetBlockByNumber(ctx context.Context, block_number int64) (*block.Block, error) {
	var hash string
	if err := b.jsonRPC(ctx, &hash, "getblockhash", block_number); err != nil {
		return nil, err
	}

	return b.GetBlockByHash(ctx, hash)
}

func (b *Blockchain) GetBlockByHash(ctx context.Context, hash string) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(ctx, &resp, "getblock", hash, 2)
	if err != nil {
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0)
	for _, tx := range resp.Tx {
		transactions = append(transactions, b.buildTransaction(ctx, tx)...)
	}

	for _, t := range transactions {
		fmt.Println(t)
	}

	return &block.Block{
		Hash:         resp.Hash,
		Number:       resp.Height,
		Transactions: transactions,
	}, nil
}

func (b *Blockchain) GetBalanceOfAddress(ctx context.Context, address string, currencyID string) (decimal.Decimal, error) {
	var resp [][][]interface{}
	if err := b.jsonRPC(ctx, &resp, "listaddressgroupings"); err != nil {
		return decimal.Zero, err
	}

	for _, gr := range resp {
		for _, a := range gr {
			if strings.EqualFold(address, a[0].(string)) {
				return decimal.NewFromFloat(a[1].(float64)), nil
			}
		}
	}

	return decimal.Zero, errors.New("unavailable address balance")
}

func (b *Blockchain) GetTransaction(ctx context.Context, transaction_hash string) (tx *transaction.Transaction, err error) {
	var resp *TxHash
	if err := b.jsonRPC(ctx, &resp, "getrawtransaction", transaction_hash, 1); err != nil {
		return nil, err
	}

	for _, v := range b.buildVOut(resp.VOut) {
		fee, err := b.calculateFee(ctx, resp)
		if err != nil {
			return nil, err
		}

		tx = &transaction.Transaction{
			TxHash:      null.StringFrom(resp.TxID),
			ToAddress:   v.ScriptPubKey.Addresses[0],
			Currency:    b.currency.ID,
			CurrencyFee: b.currency.ID,
			Fee:         fee,
			Amount:      v.Value,
			Status:      transaction.StatusSucceed,
		}
	}

	return
}

func (b *Blockchain) calculateFee(ctx context.Context, tx *TxHash) (decimal.Decimal, error) {
	vins := decimal.Zero
	vouts := decimal.Zero
	for _, v := range tx.Vin {
		vin := v.TxID
		vin_id := v.VOut

		if len(vin) == 0 {
			continue
		}

		var resp *TxHash
		if err := b.jsonRPC(ctx, &resp, "getrawtransaction", vin, 1); err != nil {
			return decimal.Zero, err
		}
		if len(resp.VOut) == 0 {
			continue
		}

		for _, vout := range resp.VOut {
			if vout.N != vin_id {
				continue
			}

			vins.Add(vout.Value)
		}
	}

	for _, vout := range tx.VOut {
		vouts.Add(vout.Value)
	}

	return vins.Sub(vouts), nil
}

func (b *Blockchain) buildVOut(vout []*VOut) []*VOut {
	nvout := make([]*VOut, 0)
	for _, v := range vout {
		if v.Value.IsPositive() && v.ScriptPubKey.Addresses != nil {
			nvout = append(nvout, v)
		}
	}

	return nvout
}

func (b *Blockchain) transactionSource(ctx context.Context, transaction *transaction.Transaction) (string, error) {
	var transHash *TxHash
	err := b.jsonRPC(ctx, &transHash, "getrawtransaction", transaction.TxHash.String, 1)
	if err != nil {
		return "", err
	}

	for _, vin := range transHash.Vin {
		if len(vin.TxID) == 0 {
			continue
		}

		var vinTransaction *TxHash
		err := b.jsonRPC(ctx, &vinTransaction, "getrawtransaction", vin.TxID, 1)
		if err != nil {
			return "", err
		}

		var source *VOut
		for _, vout := range vinTransaction.VOut {
			if vout.N == vin.VOut {
				source = vout
			}
		}

		if len(source.ScriptPubKey.Addresses) == 0 {
			return "", nil
		}

		address := source.ScriptPubKey.Addresses[0]

		return address, nil
	}

	return "", nil
}

func (b *Blockchain) buildTransaction(ctx context.Context, tx *TxHash) []*transaction.Transaction {
	transactions := make([]*transaction.Transaction, 0)
	for _, entry := range tx.VOut {
		if entry.Value.IsNegative() || entry.ScriptPubKey.Addresses == nil {
			continue
		}

		trans := &transaction.Transaction{
			Currency:    b.currency.ID,
			CurrencyFee: b.currency.ID,
			ToAddress:   entry.ScriptPubKey.Addresses[0],
			Amount:      entry.Value,
			TxHash:      null.StringFrom(tx.TxID),
			Status:      transaction.StatusSucceed,
		}

		fromAddress, err := b.transactionSource(ctx, trans)
		if err != nil {

		}

		trans.FromAddress = fromAddress

		transactions = append(transactions, trans)
	}

	return transactions
}
