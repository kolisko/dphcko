package tax

import (
	"testing"
	"time"

	"dphcko/internal/invoice"
)

func TestBuildSummary(t *testing.T) {
	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	invoices := []invoice.Invoice{
		{Number: "A5", SourceSHA256: "a", TaxableDate: date, TaxBase: 100000, Tax: 21000, Total: 121000},
		{Number: "A4", SourceSHA256: "b", RecipientVATID: "CZ87654321", TaxableDate: date, TaxBase: 1000000, Tax: 210000, Total: 1210000},
	}
	s, err := Build(invoices)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.A4) != 1 || len(s.A5) != 1 || s.Base != 1100000 || s.Tax != 231000 || s.Total != 1331000 {
		t.Fatalf("neočekávaný souhrn: %#v", s)
	}
	if s.A5Base != 100000 || s.A5Tax != 21000 {
		t.Fatalf("neočekávaný A.5 souhrn: %#v", s)
	}
}

func TestBuildRejectsDuplicates(t *testing.T) {
	inv := invoice.Invoice{Number: "DUP", SourceSHA256: "hash"}
	if _, err := Build([]invoice.Invoice{inv, inv}); err == nil {
		t.Fatal("duplicitní číslo mělo být odmítnuto")
	}
	inv2 := inv
	inv2.Number = "JINE"
	if _, err := Build([]invoice.Invoice{inv, inv2}); err == nil {
		t.Fatal("duplicitní hash měl být odmítnut")
	}
}
