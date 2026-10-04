// Represents money exactly while preserving decimal JSON and PostgreSQL values.
package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Money stores ten-thousandths of a dollar, matching decimal(12,4).
// In Go, $12.50 is Money(125000). JSON and SQL still represent it as 12.5000.
type Money int64

const MoneyScale Money = 10000
const MaxMoney Money = 999999999999

func ParseMoney(s string) (Money, error) {
	// Bound input before parsing large exponents or arbitrarily long numbers.
	if len(s) > 64 || !json.Valid([]byte(s)) || s == "null" || strings.ContainsAny(s, "\"[]{}") {
		return 0, fmt.Errorf("invalid money: %w", ErrInvalidOrder)
	}
	// JSON numbers can use exponents. Limit them before big.Rat allocates.
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.Atoi(s[i+1:])
		if err != nil || exp < -20 || exp > 20 {
			return 0, fmt.Errorf("money exponent out of range: %w", ErrInvalidOrder)
		}
	}
	amount, ok := new(big.Rat).SetString(s)
	if !ok {
		return 0, fmt.Errorf("invalid money: %w", ErrInvalidOrder)
	}
	amount.Mul(amount, big.NewRat(int64(MoneyScale), 1))
	if !amount.IsInt() || !amount.Num().IsInt64() {
		return 0, fmt.Errorf("money requires at most four decimal places: %w", ErrInvalidOrder)
	}
	value := Money(amount.Num().Int64())
	if value > MaxMoney || value < -MaxMoney {
		return 0, fmt.Errorf("money exceeds decimal(12,4): %w", ErrInvalidOrder)
	}
	return value, nil
}

func (m Money) String() string {
	// Values are bounded before JSON/SQL output, avoiding signed overflow.
	n := int64(m)
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	return fmt.Sprintf("%s%d.%04d", sign, n/10000, n%10000)
}

func (m Money) MarshalJSON() ([]byte, error) {
	if m > MaxMoney || m < -MaxMoney {
		return nil, ErrInvalidOrder
	}
	return []byte(m.String()), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	value, err := ParseMoney(string(data))
	if err != nil {
		return err
	}
	*m = value
	return nil
}

// Value and Scan preserve exact decimals at the SQL boundary (no float64).
func (m Money) Value() (driver.Value, error) {
	if m > MaxMoney || m < -MaxMoney {
		return nil, ErrInvalidOrder
	}
	return m.String(), nil
}

func (m *Money) Scan(value interface{}) error {
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	case int64:
		text = strconv.FormatInt(v, 10)
	default:
		return fmt.Errorf("cannot scan money from %T", value)
	}
	parsed, err := ParseMoney(text)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
