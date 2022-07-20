package evm

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"

	"github.com/zsmartex/multichain/pkg/block"
	"github.com/zsmartex/multichain/pkg/blockchain"
	"github.com/zsmartex/multichain/pkg/currency"
	"github.com/zsmartex/multichain/pkg/transaction"
)

var abiDefinition = `[{"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"}],"name":"approve","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_from","type":"address"},{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transferFrom","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"name":"_addr","type":"address"}],"name":"getNonce","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"founder","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"version","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"allocateEndBlock","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_from","type":"address"},{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"},{"name":"_v","type":"uint8"},{"name":"_r","type":"bytes32"},{"name":"_s","type":"bytes32"}],"name":"approveProxy","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_owners","type":"address[]"},{"name":"_values","type":"uint256[]"}],"name":"allocateTokens","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"},{"name":"_extraData","type":"bytes"}],"name":"approveAndCallcode","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"},{"name":"_extraData","type":"bytes"}],"name":"approveAndCall","outputs":[{"name":"success","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"name":"_owner","type":"address"},{"name":"_spender","type":"address"}],"name":"allowance","outputs":[{"name":"remaining","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"allocateStartBlock","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_from","type":"address"},{"name":"_to","type":"address"},{"name":"_value","type":"uint256"},{"name":"_feeUgt","type":"uint256"},{"name":"_v","type":"uint8"},{"name":"_r","type":"bytes32"},{"name":"_s","type":"bytes32"}],"name":"transferProxy","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"payable":false,"stateMutability":"nonpayable","type":"fallback"},{"anonymous":false,"inputs":[{"indexed":true,"name":"_from","type":"address"},{"indexed":true,"name":"_to","type":"address"},{"indexed":false,"name":"_value","type":"uint256"}],"name":"Transfer","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"name":"_owner","type":"address"},{"indexed":true,"name":"_spender","type":"address"},{"indexed":false,"name":"_value","type":"uint256"}],"name":"Approval","type":"event"}]`
var tokenEventIdentifier = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

type Blockchain struct {
	currency  *currency.Currency
	contracts []*currency.Currency
	client    *ethclient.Client
	setting   *blockchain.Setting
}

func NewBlockchain() blockchain.Blockchain {
	return &Blockchain{
		contracts: make([]*currency.Currency, 0),
	}
}

func (b *Blockchain) Configure(setting *blockchain.Setting) {
	rpcClient, err := rpc.Dial(setting.URI)
	if err != nil {
		panic(err)
	}

	client := ethclient.NewClient(rpcClient)
	b.client = client
	b.setting = setting

	for _, c := range setting.Currencies {
		if c.Options["erc20_contract_address"] != nil {
			b.contracts = append(b.contracts, c)
		} else {
			b.currency = c
		}
	}
}

func (b *Blockchain) GetLatestBlockNumber(ctx context.Context) (int64, error) {
	blockNumber, err := b.client.BlockNumber(ctx)

	return int64(blockNumber), err
}

func (b *Blockchain) GetBlockByNumber(ctx context.Context, blockNumber int64) (*block.Block, error) {
	result, err := b.client.BlockByNumber(ctx, big.NewInt(blockNumber))
	if err != nil {
		return nil, err
	}

	return b.GetBlockByHash(ctx, result.Hash().Hex())
}

func (b *Blockchain) GetBlockByHash(ctx context.Context, hash string) (*block.Block, error) {
	result, err := b.client.BlockByHash(ctx, common.HexToHash(hash))
	if err != nil {
		return nil, err
	}

	transactions := make([]*transaction.Transaction, 0)
	for _, t := range result.Transactions() {
		txs, err := b.buildTransaction(ctx, t)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, txs...)
	}

	return &block.Block{
		Hash:         result.Hash().Hex(),
		Number:       result.Number().Int64(),
		Transactions: transactions,
	}, nil
}

func (b *Blockchain) GetTransaction(ctx context.Context, txHash string) (*transaction.Transaction, error) {
	result, _, err := b.client.TransactionByHash(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil, err
	}

	ts, err := b.buildTransaction(ctx, result)
	if err != nil {
		return nil, err
	}

	return ts[0], nil
}

func (b *Blockchain) GetBalanceOfAddress(ctx context.Context, address string, currencyID string) (decimal.Decimal, error) {
	for _, contract := range b.contracts {
		if currencyID == contract.ID {
			return b.getERC20Balance(ctx, address, contract)
		}
	}

	blockNumber, err := b.GetLatestBlockNumber(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	amount, err := b.client.BalanceAt(context.Background(), common.HexToAddress(address), big.NewInt(blockNumber))
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(amount, -b.currency.Subunits), nil
}

func (b *Blockchain) getERC20Balance(ctx context.Context, address string, currency *currency.Currency) (decimal.Decimal, error) {
	contractAddress := common.HexToAddress(currency.Options["erc20_contract_address"].(string))

	blockNumber, err := b.GetLatestBlockNumber(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	abiJSON, err := abi.JSON(strings.NewReader(abiDefinition))
	if err != nil {
		return decimal.Zero, err
	}

	data, err := abiJSON.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return decimal.Zero, err
	}

	bytes, err := b.client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}, big.NewInt(blockNumber))
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromBigInt(new(big.Int).SetBytes(bytes), -currency.Subunits), nil
}

func (b *Blockchain) buildTransaction(ctx context.Context, tx *types.Transaction) ([]*transaction.Transaction, error) {
	receipt, err := b.client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return nil, err
	}

	if len(receipt.Logs) > 0 {
		return b.buildERC20Transactions(tx, receipt)
	} else {
		return b.buildETHTransactions(tx, receipt)
	}
}

func (b *Blockchain) buildETHTransactions(tx *types.Transaction, receipt *types.Receipt) ([]*transaction.Transaction, error) {
	msg, err := tx.AsMessage(types.LatestSignerForChainID(tx.ChainId()), tx.GasPrice())
	if err != nil {
		return nil, err
	}

	cost := decimal.NewFromBigInt(tx.Cost(), -b.currency.Subunits)
	amount := decimal.NewFromBigInt(tx.Value(), -b.currency.Subunits)
	fee := cost.Sub(amount)

	var toAddress string
	if tx.To() == nil {
		return []*transaction.Transaction{}, nil
	}

	toAddress = tx.To().Hex()

	return []*transaction.Transaction{
		{
			Currency:    b.currency.ID,
			CurrencyFee: b.currency.ID,
			TxHash:      null.StringFrom(tx.Hash().Hex()),
			FromAddress: msg.From().Hex(),
			ToAddress:   toAddress,
			Fee:         fee,
			Amount:      amount,
			Status:      b.transactionStatus(receipt),
		},
	}, nil
}

func (b *Blockchain) buildERC20Transactions(tx *types.Transaction, receipt *types.Receipt) ([]*transaction.Transaction, error) {
	if b.transactionStatus(receipt) == transaction.StatusFailed && len(receipt.Logs) == 0 {
		return b.buildInvalidErc20Transaction(tx, receipt)
	}

	fee := decimal.NewFromBigInt(big.NewInt(int64(receipt.GasUsed*tx.GasFeeCap().Uint64())), -b.currency.Subunits)

	transactions := make([]*transaction.Transaction, 0)
	for _, l := range receipt.Logs {
		if len(l.BlockHash.Bytes()) == 0 && l.BlockNumber == 0 {
			continue
		}
		if len(l.Topics) == 0 || l.Topics[0].Hex() != tokenEventIdentifier {
			continue
		}

		// Contract: l.Address.Hex()
		fromAddress := fmt.Sprintf("0x%s", l.Topics[1].Hex()[26:])
		toAddress := fmt.Sprintf("0x%s", l.Topics[2].Hex()[26:])

		for _, c := range b.contracts {
			contractAddress := c.Options["erc20_contract_address"].(string)
			if strings.EqualFold(contractAddress, l.Address.Hex()) {
				i := new(big.Int)
				i.SetString(common.Bytes2Hex(l.Data), 16)
				amount := decimal.NewFromBigInt(i, -c.Subunits)

				transactions = append(transactions, &transaction.Transaction{
					Currency:    c.ID,
					CurrencyFee: b.currency.ID,
					TxHash:      null.StringFrom(tx.Hash().Hex()),
					FromAddress: fromAddress,
					ToAddress:   toAddress,
					Fee:         fee,
					Amount:      amount,
					Status:      b.transactionStatus(receipt),
				})
			}
		}
	}

	return transactions, nil
}

func (b *Blockchain) buildInvalidErc20Transaction(tx *types.Transaction, receipt *types.Receipt) ([]*transaction.Transaction, error) {
	fee := decimal.NewFromBigInt(big.NewInt(int64(receipt.GasUsed*tx.GasFeeCap().Uint64())), -b.currency.Subunits)

	transactions := make([]*transaction.Transaction, 0)

	for _, c := range b.contracts {
		if c.Options["erc20_contract_address"] == tx.To().Hex() {
			transactions = append(transactions, &transaction.Transaction{
				TxHash:      null.StringFrom(tx.Hash().Hex()),
				BlockNumber: receipt.BlockNumber.Int64(),
				CurrencyFee: b.currency.ID,
				Currency:    c.ID,
				Fee:         fee,
				Status:      b.transactionStatus(receipt),
			})
		}
	}

	return transactions, nil
}

func (b *Blockchain) transactionStatus(receiptTx *types.Receipt) transaction.Status {
	switch receiptTx.Status {
	case 1:
		return transaction.StatusSucceed
	case 0:
		return transaction.StatusFailed
	default:
		return transaction.StatusPending
	}
}
