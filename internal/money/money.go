package money

import (
	"fmt"
	"strconv"
	"strings"
)

func FormatEURCents(cents int) string {
	amount := uint(cents)
	sign := ""
	if cents < 0 {
		amount = uint(-(cents + 1)) + 1
		sign = "-"
	}
	return sign + fmt.Sprintf("%d.%02d€", amount/100, amount%100)
}

func MultiplyCents(cents, qty int) (int, error) {
	if qty < 0 {
		return 0, fmt.Errorf("quantity must be non-negative")
	}
	if cents < 0 {
		return 0, fmt.Errorf("price must be non-negative")
	}
	maxInt := int(^uint(0) >> 1)
	if cents > 0 && qty > maxInt/cents {
		return 0, fmt.Errorf("line total exceeds supported range")
	}
	return cents * qty, nil
}

func AddCents(total, cents int) (int, error) {
	if total < 0 || cents < 0 {
		return 0, fmt.Errorf("amount must be non-negative")
	}
	maxInt := int(^uint(0) >> 1)
	if cents > maxInt-total {
		return 0, fmt.Errorf("total exceeds supported range")
	}
	return total + cents, nil
}

func ParseEURCents(value string) (int, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, nil
	}
	if strings.HasPrefix(raw, "-") {
		return 0, fmt.Errorf("invalid EUR amount %q", value)
	}
	euros, cents, decimal := strings.Cut(raw, ".")
	if euros == "" && cents == "" || strings.IndexFunc(cents, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, fmt.Errorf("invalid EUR amount %q", value)
	}
	if euros == "" {
		euros = "0"
	}
	if strings.Contains(cents, ".") || len(cents) > 2 {
		return 0, fmt.Errorf("invalid EUR amount %q", value)
	}
	if !decimal || cents == "" {
		cents = "00"
	} else if len(cents) == 1 {
		cents += "0"
	}
	euroValue, err := strconv.Atoi(euros)
	if err != nil {
		return 0, fmt.Errorf("invalid EUR amount %q", value)
	}
	centValue := int(cents[0]-'0')*10 + int(cents[1]-'0')
	maxInt := int(^uint(0) >> 1)
	if euroValue > (maxInt-centValue)/100 {
		return 0, fmt.Errorf("EUR amount %q exceeds supported range", value)
	}
	return euroValue*100 + centValue, nil
}
