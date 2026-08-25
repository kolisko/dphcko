package invoice

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Invoice struct {
	SourcePath     string
	SourceSHA256   string
	Number         string
	IssuerVATID    string
	RecipientVATID string
	TaxableDate    time.Time
	Currency       string
	TaxBase        Money
	Tax            Money
	// Rounding is a small non-taxable adjustment (QR Faktura field NTB)
	// applied only to the amount payable. It never enters the VAT base or tax.
	Rounding          Money
	Total             Money
	ConsumerConfirmed bool
}

const maxRounding Money = 99 // 0.99 CZK

func ParseQR(payload string) (Invoice, error) {
	outer := parseFields(payload)
	fields := outer
	if embedded := outer["X-INV"]; embedded != "" {
		decoded, err := url.PathUnescape(embedded)
		if err != nil {
			return Invoice{}, fmt.Errorf("neplatná integrovaná QR Faktura: %w", err)
		}
		fields = parseFields(decoded)
		for _, key := range []string{"AM", "CC", "DT"} {
			if fields[key] == "" {
				fields[key] = outer[key]
			}
		}
	}

	if tp := fields["TP"]; tp != "" && tp != "0" {
		return Invoice{}, fmt.Errorf("nepodporovaný typ daňového plnění TP:%s", tp)
	}
	if td := fields["TD"]; td != "" && td != "9" {
		return Invoice{}, fmt.Errorf("nepodporovaný typ dokladu TD:%s", td)
	}
	if sa := fields["SA"]; sa != "" && sa != "0" {
		return Invoice{}, fmt.Errorf("faktura obsahuje nepodporované zúčtování záloh")
	}
	for _, key := range []string{"TB1", "T1", "TB2", "T2"} {
		if value := fields[key]; value != "" && value != "0" && value != "0.00" {
			return Invoice{}, fmt.Errorf("QR Faktura obsahuje nepodporované plnění %s:%s", key, value)
		}
	}
	rounding := Money(0)
	if value := fields["NTB"]; value != "" {
		var err error
		rounding, err = ParseMoney(value)
		if err != nil {
			return Invoice{}, fmt.Errorf("zaokrouhlení NTB: %w", err)
		}
		if absMoney(rounding) > maxRounding {
			return Invoice{}, fmt.Errorf("QR Faktura obsahuje nepodporované nezdanitelné plnění NTB:%s; jako zaokrouhlení je povoleno nejvýše 0,99 Kč", value)
		}
	}

	get := func(keys ...string) string {
		for _, key := range keys {
			if value := fields[key]; value != "" {
				return value
			}
		}
		return ""
	}

	number := get("ID")
	if number == "" {
		return Invoice{}, fmt.Errorf("QR Faktura neobsahuje číslo dokladu (ID)")
	}
	issuer := normalizeVATID(get("VII"))
	if issuer == "" {
		return Invoice{}, fmt.Errorf("QR Faktura neobsahuje DIČ výstavce (VII)")
	}
	dateRaw := get("DPPD", "DUZP")
	date, err := time.Parse("20060102", dateRaw)
	if err != nil {
		return Invoice{}, fmt.Errorf("neplatné DUZP/DPPD %q", dateRaw)
	}
	base, err := ParseMoney(get("TB0"))
	if err != nil {
		return Invoice{}, fmt.Errorf("základ daně TB0: %w", err)
	}
	tax, err := ParseMoney(get("T0"))
	if err != nil {
		return Invoice{}, fmt.Errorf("daň T0: %w", err)
	}
	totalRaw := get("AM")
	var total Money
	if totalRaw == "" {
		total = base + tax + rounding
	} else if total, err = ParseMoney(totalRaw); err != nil {
		return Invoice{}, fmt.Errorf("celková částka: %w", err)
	}
	currency := strings.ToUpper(get("CC"))
	if currency == "" {
		currency = "CZK"
	}

	return Invoice{
		Number:         number,
		IssuerVATID:    issuer,
		RecipientVATID: normalizeVATID(get("VIR")),
		TaxableDate:    date,
		Currency:       currency,
		TaxBase:        base,
		Tax:            tax,
		Rounding:       rounding,
		Total:          total,
	}, nil
}

func parseFields(payload string) map[string]string {
	fields := map[string]string{}
	for _, item := range strings.Split(strings.TrimSpace(payload), "*") {
		if item == "" || item == "SID" || item == "SPD" || item == "1.0" {
			continue
		}
		key, value, found := strings.Cut(item, ":")
		if found {
			fields[strings.ToUpper(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	return fields
}

func normalizeVATID(value string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func VATStem(value string) string {
	return strings.TrimPrefix(normalizeVATID(value), "CZ")
}
