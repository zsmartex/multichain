package transaction

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/shopspring/decimal"
	"github.com/volatiletech/null/v9"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusSucceed  Status = "succeed"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped"
	StatusRejected Status = "rejected"
)

type Transaction struct {
	Currency    string                 `json:"currency"`
	CurrencyFee string                 `json:"currency_fee"`
	FromAddress string                 `json:"from_address"`
	ToAddress   string                 `json:"to_address"`
	Fee         decimal.Decimal        `json:"fee"`
	Amount      decimal.Decimal        `json:"amount"`
	BlockNumber int64                  `json:"block_number"`
	TxHash      null.String            `json:"tx_hash"`
	Status      Status                 `json:"status"`
	Options     map[string]interface{} `json:"options"`
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (e *Transaction) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := Transaction{}
	err := json.Unmarshal(bytes, &result)
	*e = Transaction(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (t Transaction) Value() (driver.Value, error) {
	if reflect.DeepEqual(Transaction{}, t) {
		return []byte{}, nil
	}

	return json.Marshal(t)
}
