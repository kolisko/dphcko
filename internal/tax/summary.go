package tax

import (
	"fmt"
	"math"
	"sort"

	"dphcko/internal/invoice"
)

type Summary struct {
	Invoices []invoice.Invoice
	A4       []invoice.Invoice
	A5       []invoice.Invoice
	Base     invoice.Money
	Tax      invoice.Money
	Total    invoice.Money
	A5Base   invoice.Money
	A5Tax    invoice.Money
}

func Build(invoices []invoice.Invoice) (Summary, error) {
	result := Summary{Invoices: append([]invoice.Invoice(nil), invoices...)}
	seenNumbers := map[string]bool{}
	seenHashes := map[string]bool{}
	for _, inv := range invoices {
		if seenNumbers[inv.Number] {
			return Summary{}, fmt.Errorf("duplicitní číslo dokladu %s", inv.Number)
		}
		seenNumbers[inv.Number] = true
		if inv.SourceSHA256 != "" {
			if seenHashes[inv.SourceSHA256] {
				return Summary{}, fmt.Errorf("duplicitní soubor faktury %s", inv.SourcePath)
			}
			seenHashes[inv.SourceSHA256] = true
		}
		var ok bool
		if result.Base, ok = add(result.Base, inv.TaxBase); !ok {
			return Summary{}, fmt.Errorf("součet základů daně je příliš vysoký")
		}
		if result.Tax, ok = add(result.Tax, inv.Tax); !ok {
			return Summary{}, fmt.Errorf("součet DPH je příliš vysoký")
		}
		if result.Total, ok = add(result.Total, inv.Total); !ok {
			return Summary{}, fmt.Errorf("celkový součet je příliš vysoký")
		}
		if invoice.Classify(inv) == invoice.SectionA4 {
			result.A4 = append(result.A4, inv)
		} else {
			result.A5 = append(result.A5, inv)
			if result.A5Base, ok = add(result.A5Base, inv.TaxBase); !ok {
				return Summary{}, fmt.Errorf("součet A.5 je příliš vysoký")
			}
			if result.A5Tax, ok = add(result.A5Tax, inv.Tax); !ok {
				return Summary{}, fmt.Errorf("součet DPH A.5 je příliš vysoký")
			}
		}
	}
	sort.Slice(result.A4, func(i, j int) bool {
		if result.A4[i].TaxableDate.Equal(result.A4[j].TaxableDate) {
			return result.A4[i].Number < result.A4[j].Number
		}
		return result.A4[i].TaxableDate.Before(result.A4[j].TaxableDate)
	})
	return result, nil
}

func add(a, b invoice.Money) (invoice.Money, bool) {
	if b > 0 && int64(a) > math.MaxInt64-int64(b) {
		return 0, false
	}
	if b < 0 && int64(a) < math.MinInt64-int64(b) {
		return 0, false
	}
	return a + b, true
}
