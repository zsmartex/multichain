package tron

import (
	"context"
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
	"github.com/zsmartex/multichain/pkg/currency"
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
			Type string `json:"type"` // TransferContract or TransferAssetContract
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
	currency   *currency.Currency
	contracts  []*currency.Currency
	currencies []*currency.Currency
	client     *resty.Client
	setting    *blockchain.Setting
}

func NewBlockchain() blockchain.Blockchain {
	return &Blockchain{
		contracts: make([]*currency.Currency, 0),
	}
}

func (b *Blockchain) Configure(setting *blockchain.Setting) {
	b.setting = setting
	b.client = resty.New()
	b.currencies = setting.Currencies

	for _, c := range setting.Currencies {
		if c.Options["trc20_contract_address"] != nil {
			b.contracts = append(b.contracts, c)
		} else {
			b.currency = c
		}
	}
}

func (b *Blockchain) jsonRPC(ctx context.Context, resp interface{}, method string, params interface{}) error {
	type Result struct {
		Error *json.RawMessage `json:"Error,omitempty"`
	}

	response, err := b.client.
		R().
		SetContext(ctx).
		SetResult(Result{}).
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(params).
		Post(fmt.Sprintf("%s/%s", b.setting.URI, method))

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

func (b *Blockchain) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	var resp *Block
	err := b.jsonRPC(ctx, &resp, "wallet/getnowblock", nil)
	if err != nil {
		return 0, err
	}

	return resp.BlockHeader.RawData.Number, nil
}

func (b *Blockchain) GetBlockByNumber(ctx context.Context, blockNumber int64) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(ctx, &resp, "wallet/getblockbynum", map[string]interface{}{
		"num": blockNumber,
	})
	if err != nil {
		return nil, err
	}

	return b.buildBlock(ctx, resp)
}

func (b *Blockchain) GetBlockByHash(ctx context.Context, hash string) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(ctx, &resp, "wallet/getblockbyid", map[string]interface{}{
		"value": hash,
	})
	if err != nil {
		return nil, err
	}

	return b.buildBlock(ctx, resp)
}

func (b *Blockchain) buildBlock(ctx context.Context, blk *Block) (*block.Block, error) {
	transactions := make([]*transaction.Transaction, 0)
	for _, t := range blk.Transactions {
		trans, err := b.buildTransaction(ctx, t)
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

func (b *Blockchain) buildTransaction(ctx context.Context, tx *Transaction) ([]*transaction.Transaction, error) {
	if len(tx.RawData.Contract) == 0 {
		if b.invalidTxn(tx) {
			return nil, errors.New("transaction contract not found")
		}
	}

	if tx.RawData.Contract[0].Type == "TransferContract" || tx.RawData.Contract[0].Type == "TransferAssetContract" {
		if b.invalidTxn(tx) {
			return nil, errors.New("transaction invalid txn")
		}

		if tx.RawData.Contract[0].Type == "TransferContract" {
			txr, err := b.buildTrxTransaction(tx)
			if err != nil {
				return nil, err
			}

			return []*transaction.Transaction{txr}, nil
		}
	}

	var txn *TransactionInfo
	err := b.jsonRPC(ctx, &txn, "wallet/gettransactioninfobyid", map[string]interface{}{
		"value": tx.TxID,
	})
	if err != nil {
		return nil, err
	}

	if b.invalidTrc20Txn(txn) {
		return nil, errors.New("transaction invalid trc20 txn")
	}

	return b.buildTrc20Transaction(txn)
}

func (b *Blockchain) invalidTxn(tx *Transaction) bool {
	return tx.RawData.Contract[0].Parameter.Value.Amount == 0 || tx.Ret[0].ContractRet == "REVERT"
}

func (b *Blockchain) invalidTrc20Txn(txn *TransactionInfo) bool {
	if txn.Log == nil {
		return false
	}

	return len(txn.ContractAddress) == 0 || len(txn.Log) == 0
}

func (b *Blockchain) buildTrxTransaction(txn *Transaction) (*transaction.Transaction, error) {
	tx := txn.RawData.Contract[0]
	fromAddress := concerns.HexToAddress(tx.Parameter.Value.OwnerAddress)
	toAddress := concerns.HexToAddress(tx.Parameter.Value.ToAddress)

	t := &transaction.Transaction{
		Currency:    b.currency.ID,
		CurrencyFee: b.currency.ID,
		TxHash:      null.StringFrom(txn.TxID),
		ToAddress:   toAddress.String(),
		FromAddress: fromAddress.String(),
		Amount:      decimal.NewFromBigInt(big.NewInt(tx.Parameter.Value.Amount), -b.currency.Subunits),
		Fee:         decimal.NewNullDecimal(decimal.NewFromFloat(1)),
		Status:      transaction.StatusSucceed,
	}

	return t, nil
}

func (b *Blockchain) buildTrc20Transaction(txnReceipt *TransactionInfo) ([]*transaction.Transaction, error) {
	if txnReceipt.Log == nil {
		return b.buildInvalidTrc20Txn(txnReceipt)
	}

	if b.trc20TxnStatus(txnReceipt) == transaction.StatusFailed && len(txnReceipt.Log) == 0 {
		return b.buildInvalidTrc20Txn(txnReceipt)
	}

	transactions := make([]*transaction.Transaction, 0)
	for _, log := range txnReceipt.Log {
		if len(log.Topics) == 0 || log.Topics[0] != "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
			continue
		}

		var c *currency.Currency
		for _, contract := range b.contracts {
			contractAddress := concerns.HexToAddress(fmt.Sprintf("41%s", log.Address))
			if contract.Options["trc20_contract_address"] == contractAddress.String() {
				c = contract
				break
			}
		}

		if c == nil {
			continue
		}

		bigAmount := big.NewInt(0)
		bigAmount.SetString(log.Data, 16)

		fromAddress := concerns.HexToAddress(fmt.Sprintf("41%s", log.Topics[1][24:]))
		toAddress := concerns.HexToAddress(fmt.Sprintf("41%s", log.Topics[2][24:]))
		amount := decimal.NewFromBigInt(bigAmount, -c.Subunits)

		transactions = append(transactions, &transaction.Transaction{
			Currency:    c.ID,
			CurrencyFee: b.currency.ID,
			TxHash:      null.StringFrom(txnReceipt.ID),
			ToAddress:   toAddress.String(),
			FromAddress: fromAddress.String(),
			Amount:      amount,
			Fee:         decimal.NewNullDecimal(decimal.NewFromFloat(10)),
			Status:      b.trc20TxnStatus(txnReceipt),
		})
	}

	return transactions, nil
}

func (b *Blockchain) trc20TxnStatus(txnReceipt *TransactionInfo) transaction.Status {
	if txnReceipt.Receipt.Result == "SUCCESS" {
		return transaction.StatusSucceed
	} else {
		return transaction.StatusFailed
	}
}

func (b *Blockchain) buildInvalidTrc20Txn(txnReceipt *TransactionInfo) ([]*transaction.Transaction, error) {
	var c *currency.Currency
	for _, contract := range b.contracts {
		contractAddress := concerns.HexToAddress(txnReceipt.ContractAddress)

		if contract.Options["trc20_contract_address"] == contractAddress.String() {
			c = contract
			break
		}
	}

	if c == nil {
		return nil, nil
	}

	return []*transaction.Transaction{
		{
			Currency:    c.ID,
			CurrencyFee: b.currency.ID,
			TxHash:      null.StringFrom(txnReceipt.ID),
			Status:      b.trc20TxnStatus(txnReceipt),
		},
	}, nil
}

func (b *Blockchain) GetBalanceOfAddress(ctx context.Context, address string, currencyID string) (decimal.Decimal, error) {
	var c *currency.Currency
	for _, cu := range b.currencies {
		if cu.ID == currencyID {
			c = cu
			break
		}
	}

	if c == nil {
		return decimal.Zero, errors.New("currency not found")
	}

	if c.Options["trc20_contract_address"] != nil {
		return b.loadTrc20Balance(ctx, address, c)
	} else {
		return b.loadTrxBalance(ctx, address)
	}
}

func (b *Blockchain) loadTrxBalance(ctx context.Context, address string) (decimal.Decimal, error) {
	decodedAddress, err := concerns.Base58ToAddress(address)
	if err != nil {
		return decimal.Zero, err
	}

	var resp *Account
	if err := b.jsonRPC(ctx, &resp, "wallet/getaccount", map[string]interface{}{
		"address": decodedAddress.Hex(),
	}); err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(big.NewInt(resp.Balance), -b.currency.Subunits), nil
}

func (b *Blockchain) loadTrc20Balance(ctx context.Context, address string, currency *currency.Currency) (decimal.Decimal, error) {
	ownerAddress, err := concerns.Base58ToAddress(address)
	if err != nil {
		return decimal.Zero, err
	}

	contractAddress, err := concerns.Base58ToAddress(currency.Options["trc20_contract_address"].(string))
	if err != nil {
		return decimal.Zero, err
	}

	type Result struct {
		ConstantResult []string `json:"constant_result"`
	}

	var resp *Result
	err = b.jsonRPC(ctx, &resp, "wallet/triggersmartcontract", map[string]interface{}{
		"owner_address":     ownerAddress.Hex(),
		"contract_address":  contractAddress.Hex(),
		"function_selector": "balanceOf(address)",
		"parameter":         xstrings.RightJustify(ownerAddress.Hex()[2:], 64, "0"),
	})
	if err != nil {
		return decimal.Zero, err
	}

	s := resp.ConstantResult[0]
	bi := new(big.Int)
	bi.SetString(s, 16)

	return decimal.NewFromBigInt(bi, -currency.Subunits), nil
}

func (b *Blockchain) GetTransaction(ctx context.Context, transactionHash string) (*transaction.Transaction, error) {
	var resp *Transaction
	if err := b.jsonRPC(ctx, &resp, "wallet/gettransactionbyid", map[string]interface{}{
		"value": transactionHash,
	}); err != nil {
		return nil, err
	}

	ts, err := b.buildTransaction(ctx, resp)
	if err != nil {
		return nil, err
	}

	return ts[0], err
}
