package invoice

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	_ "golang.org/x/image/tiff"
)

var pdfcpuConfigOnce sync.Once

func configurePDFCPU() {
	// QR extraction does not need pdfcpu's per-user config or fonts. Disabling
	// it keeps the application self-contained in its working directory.
	pdfcpuConfigOnce.Do(api.DisableConfigDir)
}

func DecodePDF(path string) (Invoice, error) {
	configurePDFCPU()
	f, err := os.Open(path)
	if err != nil {
		return Invoice{}, err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return Invoice{}, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Invoice{}, err
	}
	images, extractionErr := api.ExtractImagesRaw(f, nil, nil)
	if extractionErr != nil && len(images) == 0 {
		return Invoice{}, fmt.Errorf("PDF nelze přečíst: %w", extractionErr)
	}
	reader := qrcode.NewQRCodeReader()
	var lastErr error
	for _, page := range images {
		for _, pdfImage := range page {
			img, _, err := image.Decode(pdfImage.Reader)
			if err != nil {
				lastErr = err
				continue
			}
			bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
			if err != nil {
				lastErr = err
				continue
			}
			decoded, err := reader.Decode(bitmap, nil)
			if err != nil {
				lastErr = err
				continue
			}
			inv, err := ParseQR(decoded.GetText())
			if err != nil {
				lastErr = err
				continue
			}
			inv.SourcePath = path
			inv.SourceSHA256 = hex.EncodeToString(hash.Sum(nil))
			return inv, nil
		}
	}
	if lastErr != nil {
		return Invoice{}, fmt.Errorf("PDF neobsahuje čitelnou QR Fakturu: %w", lastErr)
	}
	return Invoice{}, fmt.Errorf("PDF neobsahuje žádný obraz s QR Fakturou")
}
