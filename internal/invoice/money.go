package invoice

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money stores Czech crowns as integer halere. It deliberately avoids floats.
type Money int64

func ParseMoney(value string) (Money, error) {
	s := strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if s == "" {
		return 0, fmt.Errorf("prázdná částka")
	}
	negative := strings.HasPrefix(s, "-")
	if negative || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("neplatná částka %q", value)
	}
	koruny, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("neplatná částka %q", value)
	}
	halere := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, fmt.Errorf("částka %q má více než dvě desetinná místa", value)
		}
		frac := parts[1] + strings.Repeat("0", 2-len(parts[1]))
		halere, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("neplatná částka %q", value)
		}
	}
	if koruny > (math.MaxInt64-halere)/100 {
		return 0, fmt.Errorf("částka %q je příliš vysoká", value)
	}
	result := Money(koruny*100 + halere)
	if negative {
		result = -result
	}
	return result, nil
}

func (m Money) String() string {
	abs := int64(m)
	sign := ""
	if abs < 0 {
		sign = "-"
		abs = -abs
	}
	return fmt.Sprintf("%s%d.%02d", sign, abs/100, abs%100)
}

func (m Money) WholeCrowns() int64 {
	if m >= 0 {
		return (int64(m) + 50) / 100
	}
	return (int64(m) - 50) / 100
}

func absMoney(m Money) Money {
	if m < 0 {
		return -m
	}
	return m
}
