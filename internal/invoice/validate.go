package invoice

import (
	"fmt"
	"math"
	"regexp"
	"time"
)

const A4Threshold Money = 1_000_000 // 10 000.00 CZK

type Section string

const (
	SectionA4 Section = "A4"
	SectionA5 Section = "A5"
)

type ValidationOptions struct {
	IssuerVATID string
	Year        int
	Month       time.Month
}

func Validate(inv Invoice, opts ValidationOptions) error {
	if normalizeVATID(inv.IssuerVATID) != normalizeVATID(opts.IssuerVATID) {
		return fmt.Errorf("DIČ výstavce %s neodpovídá profilu %s", inv.IssuerVATID, opts.IssuerVATID)
	}
	if inv.Currency != "CZK" {
		return fmt.Errorf("nepodporovaná měna %s", inv.Currency)
	}
	if inv.RecipientVATID != "" && !regexp.MustCompile(`^CZ[0-9]{8,10}$`).MatchString(normalizeVATID(inv.RecipientVATID)) {
		return fmt.Errorf("nepodporované tuzemské DIČ odběratele %s", inv.RecipientVATID)
	}
	if len([]rune(inv.Number)) > 60 {
		return fmt.Errorf("číslo dokladu má více než 60 znaků")
	}
	if inv.TaxBase <= 0 || inv.Tax <= 0 || inv.Total <= 0 {
		return fmt.Errorf("záporný, nulový nebo opravný doklad není podporován")
	}
	if absMoney(inv.Rounding) > maxRounding {
		return fmt.Errorf("nepodporované nezdanitelné plnění nebo zaokrouhlení %s Kč; povoleno je nejvýše 0,99 Kč", inv.Rounding.String())
	}
	if int64(inv.TaxBase) > math.MaxInt64-int64(inv.Tax) || int64(inv.TaxBase) > (math.MaxInt64-50)/21 {
		return fmt.Errorf("částky dokladu jsou příliš vysoké")
	}
	if inv.TaxableDate.Year() != opts.Year || inv.TaxableDate.Month() != opts.Month {
		return fmt.Errorf("DUZP %s není ve zvoleném období %04d/%02d", inv.TaxableDate.Format("02.01.2006"), opts.Year, opts.Month)
	}
	expectedTotal := inv.TaxBase + inv.Tax
	if inv.Rounding > 0 && int64(expectedTotal) > math.MaxInt64-int64(inv.Rounding) {
		return fmt.Errorf("částky dokladu jsou příliš vysoké")
	}
	expectedTotal += inv.Rounding
	if absMoney(inv.Total-expectedTotal) > 1 {
		return fmt.Errorf("základ + DPH + zaokrouhlení se liší od celku o více než 0,01 Kč")
	}
	expectedTax := Money((int64(inv.TaxBase)*21 + 50) / 100)
	if absMoney(inv.Tax-expectedTax) > 1 {
		return fmt.Errorf("částka DPH neodpovídá sazbě 21 %%")
	}
	if inv.Total > A4Threshold && inv.RecipientVATID == "" && !inv.ConsumerConfirmed {
		return fmt.Errorf("doklad nad 10 000 Kč nemá DIČ odběratele ani potvrzení koncového spotřebitele")
	}
	return nil
}

func Classify(inv Invoice) Section {
	if inv.Total > A4Threshold && inv.RecipientVATID != "" {
		return SectionA4
	}
	return SectionA5
}
