package invoice

import (
	"strings"
	"testing"
	"time"
)

func baseInvoice() Invoice {
	return Invoice{
		Number: "1", IssuerVATID: "CZ12345678", RecipientVATID: "CZ87654321",
		TaxableDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), Currency: "CZK",
		TaxBase: 100000, Tax: 21000, Total: 121000,
	}
}

func TestValidateAndClassifyThreshold(t *testing.T) {
	opts := ValidationOptions{IssuerVATID: "CZ12345678", Year: 2026, Month: 8}
	inv := baseInvoice()
	inv.TaxBase, inv.Tax, inv.Total = 826446, 173554, A4Threshold
	if err := Validate(inv, opts); err != nil {
		t.Fatal(err)
	}
	if got := Classify(inv); got != SectionA5 {
		t.Fatalf("přesně 10 000 Kč patří do A.5, dostal jsem %s", got)
	}
	inv.TaxBase, inv.Tax, inv.Total = 826447, 173554, A4Threshold+1
	if err := Validate(inv, opts); err != nil {
		t.Fatal(err)
	}
	if got := Classify(inv); got != SectionA4 {
		t.Fatalf("nad 10 000 Kč s DIČ patří do A.4, dostal jsem %s", got)
	}
	inv.RecipientVATID = ""
	if err := Validate(inv, opts); err == nil {
		t.Fatal("nadlimitní doklad bez DIČ a potvrzení měl být odmítnut")
	}
	inv.ConsumerConfirmed = true
	if err := Validate(inv, opts); err != nil || Classify(inv) != SectionA5 {
		t.Fatalf("potvrzený spotřebitel má projít do A.5: %v", err)
	}
}

func TestValidateRejectsInvalidInvoices(t *testing.T) {
	opts := ValidationOptions{IssuerVATID: "CZ12345678", Year: 2026, Month: 8}
	tests := []struct {
		name string
		edit func(*Invoice)
		want string
	}{
		{"výstavce", func(i *Invoice) { i.IssuerVATID = "CZ00000000" }, "výstavce"},
		{"měna", func(i *Invoice) { i.Currency = "EUR" }, "měna"},
		{"cizí DIČ", func(i *Invoice) { i.RecipientVATID = "SK1234567890" }, "tuzemské DIČ"},
		{"období", func(i *Invoice) { i.TaxableDate = i.TaxableDate.AddDate(0, -1, 0) }, "období"},
		{"záporná", func(i *Invoice) { i.TaxBase = -1 }, "není podporován"},
		{"sazba", func(i *Invoice) { i.Tax = 20000; i.Total = 120000 }, "21"},
		{"součet", func(i *Invoice) { i.Total += 2 }, "0,01"},
		{"nesedící zaokrouhlení", func(i *Invoice) { i.Rounding = -6 }, "zaokrouhlení"},
		{"příliš velké zaokrouhlení", func(i *Invoice) { i.Rounding = -100; i.Total = 120900 }, "0,99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := baseInvoice()
			tt.edit(&inv)
			err := Validate(inv, opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("chyba = %v, očekávám text %q", err, tt.want)
			}
		})
	}
}

func TestPreviousMonthAcrossYear(t *testing.T) {
	year, month := PreviousMonth(time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC))
	if year != 2025 || month != time.December {
		t.Fatalf("předchozí období = %d/%d", year, month)
	}
}
