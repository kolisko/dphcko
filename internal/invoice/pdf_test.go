package invoice

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestDecodeSyntheticQRInvoicePDF(t *testing.T) {
	configurePDFCPU()
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "qr.png")
	pdfPath := filepath.Join(dir, "faktura.pdf")
	matrix, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(validSID, gozxing.BarcodeFormat_QR_CODE, 700, 700)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, matrix.GetWidth(), matrix.GetHeight()))
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			shade := color.Gray{Y: 255}
			if matrix.Get(x, y) {
				shade.Y = 0
			}
			img.SetGray(x, y, shade)
		}
	}
	file, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := api.ImportImagesFile([]string{pngPath}, pdfPath, nil, nil); err != nil {
		t.Fatal(err)
	}
	inv, err := DecodePDF(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Number != "FV-2026-08-001" || inv.SourceSHA256 == "" || inv.SourcePath != pdfPath {
		t.Fatalf("neočekávaný dekódovaný doklad: %#v", inv)
	}
}

func TestDecodePDFWithoutQR(t *testing.T) {
	configurePDFCPU()
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "plain.png")
	pdfPath := filepath.Join(dir, "plain.pdf")
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	file, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := api.ImportImagesFile([]string{pngPath}, pdfPath, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePDF(pdfPath); err == nil {
		t.Fatal("PDF bez QR mělo být odmítnuto")
	}
}
