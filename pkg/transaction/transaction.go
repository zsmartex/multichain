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
	Currency    string                 `json:"currency,omitempty"`
	CurrencyFee string                 `json:"currency_fee,omitempty"`
	FromAddress string                 `json:"from_address,omitempty"`
	ToAddress   string                 `json:"to_address,omitempty"`
	Fee         decimal.Decimal        `json:"fee,omitempty"`
	Amount      decimal.Decimal        `json:"amount,omitempty"`
	BlockNumber int64                  `json:"block_number,omitempty"`
	TxHash      null.String            `json:"tx_hash,omitempty"`
	Status      Status                 `json:"status,omitempty"`
	Options     map[string]interface{} `json:"options,omitempty"`
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
