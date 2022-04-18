package bitcoin

import (
	"encoding/json"
	"errors"
	"math/rand"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type VOut struct {
	Value        decimal.Decimal `json:"value"`
	N            int64           `json:"n"`
	ScriptPubKey *struct {
		Addresses []string `json:"addresses"`
	}
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
	currency *blockchain.Currency
	settings *blockchain.Settings
	client   *resty.Client
}

func NewBlockchain() blockchain.Blockchain {
	return &Blockchain{
		client: resty.New(),
	}
}

func (b *Blockchain) Configure(settings *blockchain.Settings) error {
	b.settings = settings

	for _, c := range settings.Currencies {
		// allow only one currency
		b.currency = c
		break
	}

	return nil
}

func (b *Blockchain) jsonRPC(resp interface{}, method string, params ...interface{}) error {
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
		}).Post(b.settings.URI)

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

func (b *Blockchain) GetLatestBlockNumber() (int64, error) {
	var resp int64
	if err := b.jsonRPC(&resp, "getblockcount"); err != nil {
		return 0, err
	}

	return resp, nil
}

func (b *Blockchain) GetBlockByNumber(block_number int64) (*block.Block, error) {
	var hash string
	if err := b.jsonRPC(&hash, "getblockhash", block_number); err != nil {
		return nil, err
	}

	return b.GetBlockByHash(hash)
}

func (b *Blockchain) GetBlockByHash(hash string) (*block.Block, error) {
	var resp *Block
	b.jsonRPC(&resp, "getblock", hash, 2)

	transactions := b.buildTransaction(resp.Tx[0])
	return &block.Block{
		Hash:         resp.Hash,
		Number:       resp.Height,
		Transactions: transactions,
	}, nil
}

func (b *Blockchain) GetBalanceOfAddress(address string, _currency_id string) (decimal.Decimal, error) {
	var resp [][][]string
	if err := b.jsonRPC(&resp, "listaddressgroupings", address); err != nil {
		return decimal.Zero, err
	}

	for _, gr := range resp {
		for _, a := range gr {
			if a[0] == address {
				return decimal.NewFromString(a[1])
			}
		}
	}

	return decimal.Zero, errors.New("unavailable address balance")
}

func (b *Blockchain) GetTransaction(transaction_hash string) ([]*transaction.Transaction, error) {
	var resp *TxHash
	if err := b.jsonRPC(&resp, "getrawtransaction", transaction_hash, 1); err != nil {
		return nil, err
	}

	return b.buildTransaction(resp), nil
}

func (b *Blockchain) transactionSource(transaction *transaction.Transaction) (address string) {
	var transHash *TxHash
	b.jsonRPC(&transHash, "getrawtransaction", transaction.TxHash.String, 1)

	for _, vin := range transHash.Vin {
		if len(vin.TxID) == 0 {
			continue
		}

		var vinTransaction *TxHash
		b.jsonRPC(&transHash, "getrawtransaction", vin.TxID, 1)

		var source *VOut
		for _, vout := range vinTransaction.VOut {
			if vout.N == vin.VOut {
				source = vout
			}
		}

		address := source.ScriptPubKey.Addresses[0]

		return address
	}

	return ""
}

func (b *Blockchain) buildTransaction(tx *TxHash) []*transaction.Transaction {
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
			Status:      transaction.TransactionStatusSuccess,
		}

		fromAddress := b.transactionSource(trans)

		trans.FromAddress = fromAddress

		transactions = append(transactions, trans)
	}

	return transactions
}
