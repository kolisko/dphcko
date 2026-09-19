package epo

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const uploadPageURL = "https://adisspr.mfcr.cz/dpr/adis/idpr_epo/epo2/uvod/nacteni_souboru.faces"

// OpenUploadPage opens EPO's generic XML upload page. Generated files remain
// local and the user chooses which one to upload in the browser.
func OpenUploadPage(browser string) error {
	return openUploadPage(browser, openBrowser)
}

func openUploadPage(browser string, opener func(string, string) error) error {
	return opener(uploadPageURL, browser)
}

func openBrowser(rawURL, browser string) error {
	command, err := browserCommand(runtime.GOOS, browser, rawURL, exec.LookPath)
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		output, err := command.CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message != "" {
				return fmt.Errorf("otevření prohlížeče: %s", message)
			}
			return fmt.Errorf("otevření prohlížeče: %w", err)
		}
		return nil
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("otevření prohlížeče: %w", err)
	}
	go func() { _ = command.Wait() }()
	return nil
}

func browserCommand(goos, browser, rawURL string, lookPath func(string) (string, error)) (*exec.Cmd, error) {
	browser = strings.TrimSpace(browser)
	if browser == "" || browser == "default" {
		switch goos {
		case "darwin":
			return exec.Command("open", rawURL), nil
		case "windows":
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL), nil
		default:
			return exec.Command("xdg-open", rawURL), nil
		}
	}

	if goos == "darwin" {
		bundleIDs := map[string]string{
			"chrome": "com.google.Chrome", "chromium": "org.chromium.Chromium",
			"edge": "com.microsoft.edgemac", "firefox": "org.mozilla.firefox",
			"brave": "com.brave.Browser", "safari": "com.apple.Safari",
			"opera": "com.operasoftware.Opera", "vivaldi": "com.vivaldi.Vivaldi",
		}
		bundleID, ok := bundleIDs[browser]
		if !ok {
			return nil, fmt.Errorf("neznámý prohlížeč %q", browser)
		}
		return exec.Command("open", "-b", bundleID, rawURL), nil
	}

	executables := map[string][]string{
		"chrome":   {"google-chrome", "google-chrome-stable", "chrome"},
		"chromium": {"chromium", "chromium-browser"},
		"edge":     {"microsoft-edge", "microsoft-edge-stable", "msedge"},
		"firefox":  {"firefox"},
		"brave":    {"brave-browser", "brave"},
		"opera":    {"opera"},
		"vivaldi":  {"vivaldi", "vivaldi-stable"},
	}
	candidates, ok := executables[browser]
	if !ok {
		return nil, fmt.Errorf("prohlížeč %q není na systému %s podporovaný", browser, goos)
	}
	if goos == "windows" {
		windowsExecutables := map[string]string{
			"chrome": "chrome", "chromium": "chromium", "edge": "msedge",
			"firefox": "firefox", "brave": "brave", "opera": "opera", "vivaldi": "vivaldi",
		}
		return exec.Command("cmd", "/c", "start", "", windowsExecutables[browser], rawURL), nil
	}
	for _, candidate := range candidates {
		path, err := lookPath(candidate)
		if err == nil {
			return exec.Command(path, rawURL), nil
		}
	}
	return nil, fmt.Errorf("prohlížeč %q není nainstalovaný nebo není v PATH", browser)
}
