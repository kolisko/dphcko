package invoice

import (
	"net/url"
	"testing"
	"time"
)

const validSID = "SID*1.0*ID:FV-2026-08-001*VII:CZ12345678*VIR:CZ87654321*DUZP:20260815*TB0:1000.00*T0:210.00*AM:1210.00*CC:CZK*TD:9*TP:0*SA:0"

func TestParseQRInvoice(t *testing.T) {
	inv, err := ParseQR(validSID)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Number != "FV-2026-08-001" || inv.IssuerVATID != "CZ12345678" || inv.RecipientVATID != "CZ87654321" {
		t.Fatalf("neočekávaná identifikace: %#v", inv)
	}
	if inv.TaxableDate != time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("neočekávané DUZP: %s", inv.TaxableDate)
	}
	if inv.TaxBase != 100000 || inv.Tax != 21000 || inv.Total != 121000 {
		t.Fatalf("neočekávané částky: %#v", inv)
	}
}

func TestParseQRPlatbaPlusFaktura(t *testing.T) {
	payload := "SPD*1.0*AM:1210.00*CC:CZK*X-INV:" + url.PathEscape(validSID)
	inv, err := ParseQR(payload)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Number != "FV-2026-08-001" || inv.Total != 121000 {
		t.Fatalf("neočekávaná faktura: %#v", inv)
	}
}

func TestParseQRAllowsSmallNonTaxableRounding(t *testing.T) {
	payload := "SID*1.0*ID:20260001*VII:CZ12345678*VIR:CZ87654321*DUZP:20260714*TB0:4156.25*T0:872.81*NTB:-0.06*AM:5029.00*CC:CZK*TD:9*TP:0*SA:0"
	inv, err := ParseQR(payload)
	if err != nil {
		t.Fatal(err)
	}
	if inv.TaxBase != 415625 || inv.Tax != 87281 || inv.Rounding != -6 || inv.Total != 502900 {
		t.Fatalf("neočekávané částky se zaokrouhlením: %#v", inv)
	}
	if err := Validate(inv, ValidationOptions{IssuerVATID: "CZ12345678", Year: 2026, Month: time.July}); err != nil {
		t.Fatalf("běžné haléřové zaokrouhlení mělo projít: %v", err)
	}
}

func TestParseQRRejectsNonTaxableSupplyDisguisedAsRounding(t *testing.T) {
	payload := validSID + "*NTB:1.00"
	if _, err := ParseQR(payload); err == nil {
		t.Fatal("nezdanitelné plnění nad limit zaokrouhlení mělo být odmítnuto")
	}
}

func TestParseQRRejectsUnsupportedDocuments(t *testing.T) {
	for _, payload := range []string{
		validSID + "*TD:1",
		validSID + "*TP:1",
		validSID + "*SA:1",
		validSID + "*TB1:10.00",
	} {
		if _, err := ParseQR(payload); err == nil {
			t.Errorf("payload měl být odmítnut: %s", payload)
		}
	}
}
