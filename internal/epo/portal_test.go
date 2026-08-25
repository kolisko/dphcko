package epo

import "testing"

func TestOpenUploadPageOnlyOpensOfficialURL(t *testing.T) {
	opened := ""
	err := openUploadPage(func(rawURL string) error {
		opened = rawURL
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened != "https://adisspr.mfcr.cz/dpr/adis/idpr_epo/epo2/uvod/nacteni_souboru.faces" {
		t.Fatalf("otevřeno neočekávané URL: %s", opened)
	}
}
