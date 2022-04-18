package tron

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/go-resty/resty/v2"
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
	blockHeader  BlockHeader    `json:"block_header"`
	Transactions []*Transaction `json:"transactions"`
}

type Blockchain struct {
	currency        *blockchain.Currency
	trc10_contracts []*blockchain.Currency
	trc20_contracts []*blockchain.Currency
	client          *resty.Client
	settings        *blockchain.Settings
}

func (b *Blockchain) jsonRPC(resp interface{}, method string, params ...interface{}) error {
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
		SetBody(map[string]interface{}{
			"version": "2.0",
			"id":      rand.Int(),
			"method":  method,
			"params":  params,
		}).
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
	err := b.jsonRPC(&resp, "wallet/getnowblock")
	if err != nil {
		return 0, err
	}

	return resp.blockHeader.RawData.Number, nil
}

func (b *Blockchain) GetBlockByNumber(block_number int64) (*block.Block, error) {
	var resp *Block
	err := b.jsonRPC(&resp, "wallet/getblockbynum", map[string]interface{}{
		"num": block_number,
	})
	if err != nil {
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0)
	for _, t := range resp.Transactions {
		if len(t.RawData.Contract) == 0 {
			continue
		}

		if t.RawData.Contract[0].Type == "TransferContract" || t.RawData.Contract[0].Type == "TransferAssetContract" {
			if b.invalid_txn(t) {
				continue
			}

			// build transaction for coin and trc10
			switch t.RawData.Contract[0].Type {
			case "TransferContract":
				transaction, err := b.buildTrxTransaction(t)
				if err != nil {
					return nil, err
				}

				transactions = append(transactions, transaction)
			case "TransferAssetContract":
				transaction, err := b.buildTrc10Transaction(t)
				if err != nil {
					return nil, err
				}

				transactions = append(transactions, transaction)
			}
		} else {
			var txn *TransactionInfo
			err := b.jsonRPC(&txn, "wallet/gettransactioninfobyid", map[string]interface{}{
				"value": t.TxID,
			})
			if err != nil {
				continue
			}

			if txn == nil {
				continue
			}

			if b.invalid_trc20_txn(txn) {
				continue
			}

			trans, err := b.buildTrc20Transaction(txn)
			if err != nil {
				return nil, err
			}

			transactions = append(transactions, trans...)
		}
	}

	return &block.Block{
		Number:       block_number,
		Transactions: transactions,
	}, nil
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
		Amount:      decimal.NewFromBigInt(big.NewInt(tx.Parameter.Value.Amount), -b.currency.Subunits),
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
		Amount:      decimal.NewFromBigInt(big.NewInt(tx.Parameter.Value.Amount), -currency.Subunits),
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

		amount := decimal.NewFromBigInt(big.NewInt(0), -currency.Subunits)

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
