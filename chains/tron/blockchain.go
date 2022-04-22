package tron

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/go-resty/resty/v2"
	"github.com/huandu/xstrings"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
	"github.com/zsmartex/multichain/chains/tron/concerns"
	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/transaction"
)

type BlockHeader struct {
	RawData struct {
		Number int64 `json:"number"`
	} `json:"raw_data"`
}

type Transaction struct {
	TxID string `json:"txID"`
	Ret  []struct {
		ContractRet string `json:"contractRet"`
	} `json:"ret"`
	RawData struct {
		Contract []struct {
			Parameter struct {
				Value struct {
					AssetName       string `json:"asset_name"`
					Data            string `json:"data"`
					Amount          int64  `json:"amount"`
					OwnerAddress    string `json:"owner_address"`
					ToAddress       string `json:"to_address"`
					ContractAddress string `json:"contract_address"`
				} `json:"value"`
				TypeUrl string `json:"type_url"`
			} `json:"parameter"`
			Type string `json:"TransferContract"` // TransferContract or TransferAssetContract
		} `json:"contract"`
	} `json:"raw_data"`
}

type TransactionInfo struct {
	ID              string `json:"id"`
	ContractAddress string `json:"contract_address"`
	Receipt         struct {
		Result string
	}
	Log []struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	}
}

type Block struct {
	BlockHeader  BlockHeader    `json:"block_header"`
	Transactions []*Transaction `json:"transactions"`
}

type Account struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
	AssetV2 []*struct {
		Key   string `json:"key"`
		Value int64  `json:"value"`
	}
}

type Blockchain struct {
	currency        *blockchain.Currency
	trc10_contracts []*blockchain.Currency
	trc20_contracts []*blockchain.Currency
	currencies      []*blockchain.Currency
	client          *resty.Client
	settings        *blockchain.Settings
}

func NewBlockchain() blockchain.Blockchain {
	return &Blockchain{
		trc10_contracts: make([]*blockchain.Currency, 0),
		trc20_contracts: make([]*blockchain.Currency, 0),
	}
}

func (b *Blockchain) Configure(settings *blockchain.Settings) error {
	b.settings = settings
	b.client = resty.New()
	b.currencies = settings.Currencies

	for _, c := range settings.Currencies {
		if len(c.Options["trc10_asset_id"]) > 0 {
			b.trc10_contracts = append(b.trc10_contracts, c)
		} else if len(c.Options["trc20_asset_id"]) > 0 {
			b.trc20_contracts = append(b.trc20_contracts, c)
		} else {
			b.currency = c
		}
	}

	return nil
}

func (b *Blockchain) jsonRPC(resp interface{}, method string, params interface{}) error {
	type Result struct {
		Error *json.RawMessage `json:"Error,omitempty"`
	}

	response, err := b.client.
		R().
		SetResult(Result{}).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(params).
		Post(b.settings.URI)

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

func (b *Blockchain) GetLatestBlockNumber() (int64, error) {
	var resp *Block
	err := b.jsonRPC(&resp, "wallet/getnowblock", nil)
	if err != nil {
		return 0, err
	}

	return resp.BlockHeader.RawData.Number, nil
}

func (b *Blockchain) GetBlockByNumber(block_number int64) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(&resp, "wallet/getblockbynum", map[string]interface{}{
		"num": block_number,
	})
	if err != nil {
		return nil, err
	}

	return b.buildBlock(resp)
}

func (b *Blockchain) GetBlockByHash(hash string) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(&resp, "wallet/getblockbyid", map[string]interface{}{
		"value": hash,
	})
	if err != nil {
		return nil, err
	}

	return b.buildBlock(resp)
}

func (b *Blockchain) buildBlock(blk *Block) (*block.Block, error) {
	transactions := make([]*transaction.Transaction, 0)
	for _, t := range blk.Transactions {
		trans, err := b.buildTransaction(t)
		if err != nil {
			return nil, err
		}

		for _, t2 := range trans {
			t2.BlockNumber = blk.BlockHeader.RawData.Number
		}

		transactions = append(transactions, trans...)
	}

	return &block.Block{
		Number:       blk.BlockHeader.RawData.Number,
		Transactions: transactions,
	}, nil
}

func (b *Blockchain) buildTransaction(tx *Transaction) ([]*transaction.Transaction, error) {
	if len(tx.RawData.Contract) == 0 {
		if b.invalid_txn(tx) {
			return nil, errors.New("transaction contract not found")
		}
	}

	if tx.RawData.Contract[0].Type == "TransferContract" || tx.RawData.Contract[0].Type == "TransferAssetContract" {
		if b.invalid_txn(tx) {
			return nil, errors.New("transaction invalid txn")
		}

		// build transaction for coin and trc10
		switch tx.RawData.Contract[0].Type {
		case "TransferContract":
			txr, err := b.buildTrxTransaction(tx)
			if err != nil {
				return nil, err
			}

			return []*transaction.Transaction{txr}, nil
		case "TransferAssetContract":
			txr, err := b.buildTrc10Transaction(tx)
			if err != nil {
				return nil, err
			}

			return []*transaction.Transaction{txr}, nil
		}
	}

	var txn *TransactionInfo
	err := b.jsonRPC(&txn, "wallet/gettransactioninfobyid", map[string]interface{}{
		"value": tx.TxID,
	})
	if err != nil {
		return nil, err
	}

	if b.invalid_trc20_txn(txn) {
		return nil, errors.New("transaction invalid trc20 txn")
	}

	return b.buildTrc20Transaction(txn)
}

func (b *Blockchain) invalid_txn(tx *Transaction) bool {
	return tx.RawData.Contract[0].Parameter.Value.Amount == 0 || tx.Ret[0].ContractRet == "REVERT"
}

func (b *Blockchain) invalid_trc20_txn(txn *TransactionInfo) bool {
	if txn.Log == nil {
		return false
	}

	return len(txn.ContractAddress) == 0 || len(txn.Log) == 0
}

func (b *Blockchain) buildTrxTransaction(txn *Transaction) (*transaction.Transaction, error) {
	tx := txn.RawData.Contract[0]
	from_address, err := concerns.DecodeAddress(tx.Parameter.Value.OwnerAddress)
	if err != nil {
		return nil, err
	}
	to_address, err := concerns.EncodeAddress(tx.Parameter.Value.ToAddress)
	if err != nil {
		return nil, err
	}

	transaction := &transaction.Transaction{
		Currency:    b.currency.ID,
		CurrencyFee: b.currency.ID,
		TxHash:      null.StringFrom(txn.TxID),
		ToAddress:   to_address,
		FromAddress: from_address,
		Amount:      decimal.NewFromBigInt(big.NewInt(tx.Parameter.Value.Amount), -b.currency.BaseFactor),
		Status:      transaction.TransactionStatusSuccess,
	}

	return transaction, nil
}

func (b *Blockchain) buildTrc10Transaction(txn *Transaction) (*transaction.Transaction, error) {
	tx := txn.RawData.Contract[0]
	var currency *blockchain.Currency
	for _, c := range b.trc10_contracts {
		asset_id, err := concerns.DecodeHex(tx.Parameter.Value.AssetName)
		if err != nil {
			return nil, err
		}
		if asset_id == c.Options["trc10_asset_id"] {
			currency = c
			break
		}
	}

	if currency == nil {
		return nil, errors.New("currency not found")
	}

	from_address, err := concerns.DecodeAddress(tx.Parameter.Value.OwnerAddress)
	if err != nil {
		return nil, err
	}
	to_address, err := concerns.EncodeAddress(tx.Parameter.Value.ToAddress)
	if err != nil {
		return nil, err
	}

	transaction := &transaction.Transaction{
		Currency:    currency.ID,
		CurrencyFee: b.currency.ID,
		TxHash:      null.StringFrom(txn.TxID),
		ToAddress:   to_address,
		FromAddress: from_address,
		Amount:      decimal.NewFromBigInt(big.NewInt(tx.Parameter.Value.Amount), -currency.BaseFactor),
		Status:      transaction.TransactionStatusSuccess,
	}

	return transaction, nil
}

func (b *Blockchain) buildTrc20Transaction(txn_receipt *TransactionInfo) ([]*transaction.Transaction, error) {
	if txn_receipt.Log == nil {
		return b.buildInvalidTrc20Txn(txn_receipt)
	}

	if b.trc20TxnStatus(txn_receipt) == transaction.TransactionStatusFailed && len(txn_receipt.Log) == 0 {
		return b.buildInvalidTrc20Txn(txn_receipt)
	}

	transactions := make([]*transaction.Transaction, 0)
	for _, log := range txn_receipt.Log {
		if len(log.Topics) == 0 {
			continue
		}

		if log.Topics[0] == "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
			continue
		}

		var currency *blockchain.Currency
		for _, c := range b.trc20_contracts {
			contract_address, err := concerns.EncodeAddress(fmt.Sprintf("41%s", log.Address))
			if err != nil {
				return nil, err
			}

			if c.Options["trc20_contract_address"] == contract_address {
				currency = c
				break
			}
		}

		if currency == nil {
			continue
		}

		from_address, err := concerns.EncodeAddress(fmt.Sprintf("41%s", log.Topics[1][24:]))
		if err != nil {
			return nil, err
		}

		to_address, err := concerns.EncodeAddress(fmt.Sprintf("41%s", log.Topics[2][24:]))
		if err != nil {
			return nil, err
		}

		amount := decimal.NewFromBigInt(big.NewInt(0), -currency.BaseFactor)

		transactions = append(transactions, &transaction.Transaction{
			Currency:    currency.ID,
			CurrencyFee: b.currency.ID,
			TxHash:      null.StringFrom(txn_receipt.ID),
			ToAddress:   to_address,
			FromAddress: from_address,
			Amount:      amount,
			Status:      b.trc20TxnStatus(txn_receipt),
		})
	}

	return transactions, nil
}

func (b *Blockchain) trc20TxnStatus(txn_receipt *TransactionInfo) transaction.TransactionStatus {
	if txn_receipt.Receipt.Result == "SUCCESS" {
		return transaction.TransactionStatusSuccess
	} else {
		return transaction.TransactionStatusFailed
	}
}

func (b *Blockchain) buildInvalidTrc20Txn(txn_receipt *TransactionInfo) ([]*transaction.Transaction, error) {
	var currency *blockchain.Currency
	for _, c := range b.trc20_contracts {
		contract_address, err := concerns.EncodeAddress(txn_receipt.ContractAddress)
		if err != nil {
			return nil, err
		}

		if c.Options["trc20_contract_address"] == contract_address {
			currency = c
			break
		}
	}

	if currency == nil {
		return nil, errors.New("currency not found")
	}

	return []*transaction.Transaction{
		{
			Currency:    currency.ID,
			CurrencyFee: b.currency.ID,
			TxHash:      null.StringFrom(txn_receipt.ID),
			Status:      b.trc20TxnStatus(txn_receipt),
		},
	}, nil
}

func (b *Blockchain) GetBalanceOfAddress(address string, currency_id string) (decimal.Decimal, error) {
	var currency *blockchain.Currency
	for _, c := range b.currencies {
		if c.ID == currency_id {
			currency = c
			break
		}
	}

	if currency == nil {
		return decimal.Zero, errors.New("currency not found")
	}

	if len(currency.Options["trc10_asset_id"]) > 0 {
		return b.loadTrc10Balance(address, currency)
	} else if len(currency.Options["trc20_contract_address"]) > 0 {
		return b.loadTrc20Balance(address, currency)
	} else {
		return b.loadTrxBalance(address)
	}
}

func (b *Blockchain) loadTrxBalance(address string) (decimal.Decimal, error) {
	decoded_address, err := concerns.DecodeAddress(address)
	if err != nil {
		return decimal.Zero, err
	}

	var resp *Account
	if err := b.jsonRPC(&resp, "wallet/getaccount", map[string]interface{}{
		"address": decoded_address,
	}); err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(big.NewInt(resp.Balance), -b.currency.BaseFactor), nil
}

func (b *Blockchain) loadTrc10Balance(address string, currency *blockchain.Currency) (decimal.Decimal, error) {
	decoded_address, err := concerns.DecodeAddress(address)
	if err != nil {
		return decimal.Zero, err
	}

	var resp *Account
	if err := b.jsonRPC(&resp, "wallet/getaccount", map[string]interface{}{
		"address": decoded_address,
	}); err != nil {
		return decimal.Zero, err
	}

	if resp.AssetV2 == nil {
		return decimal.Zero, errors.New("asset not found")
	}

	for _, a := range resp.AssetV2 {
		if a.Key == currency.Options["trc10_asset_id"] {
			return decimal.NewFromBigInt(big.NewInt(a.Value), -currency.BaseFactor), nil
		}
	}

	return decimal.Zero, nil
}

func (b *Blockchain) loadTrc20Balance(address string, currency *blockchain.Currency) (decimal.Decimal, error) {
	owner_address, err := concerns.DecodeAddress(address)
	if err != nil {
		return decimal.Zero, err
	}

	contract_address, err := concerns.DecodeAddress(currency.Options["trc20_contract_address"])
	if err != nil {
		return decimal.Zero, err
	}

	type Result struct {
		ConstantResult []string `json:"constant_result"`
	}

	var resp *Result
	b.jsonRPC(&resp, "wallet/triggersmartcontract", map[string]interface{}{
		"owner_address":     owner_address,
		"contract_address":  contract_address,
		"function_selector": "balanceOf(address)",
		"parameter":         xstrings.RightJustify(owner_address[2:], 64, "0"),
	})

	s := resp.ConstantResult[0]
	bi := new(big.Int)
	bi.SetString(s, 16)

	return decimal.NewFromBigInt(bi, -currency.BaseFactor), nil
}

func (b *Blockchain) GetTransaction(transaction_hash string) ([]*transaction.Transaction, error) {
	var resp *Transaction
	if err := b.jsonRPC(&resp, "wallet/gettransactionbyid ", map[string]interface{}{
		"value": transaction_hash,
	}); err != nil {
		return nil, err
	}

	return b.buildTransaction(resp)
}
