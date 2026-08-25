package epo

import (
	"fmt"
	"os/exec"
	"runtime"
)

const uploadPageURL = "https://adisspr.mfcr.cz/dpr/adis/idpr_epo/epo2/uvod/nacteni_souboru.faces"

// OpenUploadPage opens EPO's generic XML upload page. Generated files remain
// local and the user chooses which one to upload in the browser.
func OpenUploadPage() error {
	return openUploadPage(openBrowser)
}

func openUploadPage(opener func(string) error) error {
	return opener(uploadPageURL)
}

func openBrowser(rawURL string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", rawURL)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		command = exec.Command("xdg-open", rawURL)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("otevření prohlížeče: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}
