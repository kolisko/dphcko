package idoklad

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dphcko/internal/invoice"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const AppURL = "https://app.idoklad.cz/IssuedInvoice"

type Session struct {
	context         context.Context
	cancelContext   context.CancelFunc
	cancelAllocator context.CancelFunc
}

type ImportResult struct {
	Imported []string
	Skipped  []string
}

func Open(root, downloadDir, browserName string) (*Session, error) {
	executable, err := findChromiumExecutable(runtime.GOOS, browserName, exec.LookPath, os.Stat, os.UserHomeDir)
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(root, ".dphcko", "idoklad-chrome")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, fmt.Errorf("vytvoření profilu Chromu pro iDoklad: %w", err)
	}
	if err := os.Chmod(profileDir, 0o700); err != nil {
		return nil, fmt.Errorf("zabezpečení profilu Chromu pro iDoklad: %w", err)
	}
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return nil, fmt.Errorf("vytvoření složky pro stažené faktury: %w", err)
	}

	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(executable),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("start-maximized", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", true),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	allocatorContext, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	browserContext, cancelContext := chromedp.NewContext(allocatorContext)
	err = chromedp.Run(browserContext,
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).
			WithDownloadPath(downloadDir).
			WithEventsEnabled(true),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, errorText, _, err := page.Navigate(AppURL).Do(ctx)
			if err != nil {
				return err
			}
			if errorText != "" {
				return errors.New(errorText)
			}
			return nil
		}),
	)
	if err != nil {
		cancelContext()
		cancelAllocator()
		return nil, fmt.Errorf("spuštění Chromu pro iDoklad: %w", err)
	}
	return &Session{context: browserContext, cancelContext: cancelContext, cancelAllocator: cancelAllocator}, nil
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	s.cancelContext()
	s.cancelAllocator()
}

func WaitForDownloads(dir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pending, err := pendingDownloads(dir)
		if err != nil {
			return err
		}
		if !pending {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("některé soubory se stále stahují; dokončete je v Chromu a import spusťte znovu")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func ImportPDFs(downloadDir, periodDir string) (ImportResult, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return ImportResult{}, err
	}
	var result ImportResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		source := filepath.Join(downloadDir, entry.Name())
		identical, err := directoryContainsFile(source, periodDir)
		if err != nil {
			return result, err
		}
		if identical {
			if err := os.Remove(source); err != nil {
				return result, fmt.Errorf("odstranění duplicitního staženého souboru %s: %w", entry.Name(), err)
			}
			result.Skipped = append(result.Skipped, entry.Name())
			continue
		}
		destination, identical, err := availableDestination(source, periodDir, entry.Name())
		if err != nil {
			return result, err
		}
		if identical {
			if err := os.Remove(source); err != nil {
				return result, fmt.Errorf("odstranění duplicitního staženého souboru %s: %w", entry.Name(), err)
			}
			result.Skipped = append(result.Skipped, entry.Name())
			continue
		}
		if err := moveWithoutOverwrite(source, destination); err != nil {
			return result, fmt.Errorf("import %s: %w", entry.Name(), err)
		}
		result.Imported = append(result.Imported, destination)
	}
	return result, nil
}

func ImportDownloadedPDFs(downloadDir, root, issuerVATID string) (ImportResult, []string, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return ImportResult{}, nil, err
	}
	var result ImportResult
	var documentNumbers []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		source := filepath.Join(downloadDir, entry.Name())
		parsed, err := invoice.DecodePDF(source)
		if err != nil {
			return result, documentNumbers, fmt.Errorf("ověření staženého PDF %s: %w", entry.Name(), err)
		}
		if invoice.VATStem(parsed.IssuerVATID) != invoice.VATStem(issuerVATID) {
			return result, documentNumbers, fmt.Errorf("stažené PDF %s patří jinému výstavci %s", entry.Name(), parsed.IssuerVATID)
		}
		periodDir := invoice.PeriodDirectory(root, parsed.TaxableDate.Year(), parsed.TaxableDate.Month())
		if err := os.MkdirAll(periodDir, 0o750); err != nil {
			return result, documentNumbers, err
		}
		identical, err := directoryContainsFile(source, periodDir)
		if err != nil {
			return result, documentNumbers, err
		}
		if identical {
			if err := os.Remove(source); err != nil {
				return result, documentNumbers, fmt.Errorf("odstranění duplicitního staženého souboru %s: %w", entry.Name(), err)
			}
			result.Skipped = append(result.Skipped, entry.Name())
			documentNumbers = append(documentNumbers, parsed.Number)
			continue
		}
		destination, _, err := availableDestination(source, periodDir, entry.Name())
		if err != nil {
			return result, documentNumbers, err
		}
		if err := moveWithoutOverwrite(source, destination); err != nil {
			return result, documentNumbers, fmt.Errorf("import %s: %w", entry.Name(), err)
		}
		result.Imported = append(result.Imported, destination)
		documentNumbers = append(documentNumbers, parsed.Number)
	}
	return result, documentNumbers, nil
}

func directoryContainsFile(source, dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		same, err := sameFileContents(source, filepath.Join(dir, entry.Name()))
		if err != nil {
			return false, err
		}
		if same {
			return true, nil
		}
	}
	return false, nil
}

func pendingDownloads(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".pdf.crdownload") || strings.HasSuffix(name, ".pdf.tmp") {
			return true, nil
		}
	}
	return false, nil
}

func availableDestination(source, periodDir, name string) (string, bool, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for suffix := 1; ; suffix++ {
		candidateName := name
		if suffix > 1 {
			candidateName = fmt.Sprintf("%s-%d%s", stem, suffix, extension)
		}
		candidate := filepath.Join(periodDir, candidateName)
		_, err := os.Stat(candidate)
		if os.IsNotExist(err) {
			return candidate, false, nil
		}
		if err != nil {
			return "", false, err
		}
		same, err := sameFileContents(source, candidate)
		if err != nil {
			return "", false, err
		}
		if same {
			return candidate, true, nil
		}
	}
}

func sameFileContents(first, second string) (bool, error) {
	firstHash, err := fileHash(first)
	if err != nil {
		return false, err
	}
	secondHash, err := fileHash(second)
	if err != nil {
		return false, err
	}
	return firstHash == secondHash, nil
}

func fileHash(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

func moveWithoutOverwrite(source, destination string) error {
	destinationFile, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	sourceFile, err := os.Open(source)
	if err != nil {
		destinationFile.Close()
		os.Remove(destination)
		return err
	}
	_, copyErr := io.Copy(destinationFile, sourceFile)
	closeSourceErr := sourceFile.Close()
	syncErr := destinationFile.Sync()
	closeDestinationErr := destinationFile.Close()
	if copyErr != nil || closeSourceErr != nil || syncErr != nil || closeDestinationErr != nil {
		os.Remove(destination)
		return errors.Join(copyErr, closeSourceErr, syncErr, closeDestinationErr)
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return nil
}

func findChromiumExecutable(
	goos, browserName string,
	lookPath func(string) (string, error),
	stat func(string) (os.FileInfo, error),
	userHomeDir func() (string, error),
) (string, error) {
	browserName = strings.TrimSpace(browserName)
	if browserName == "" || browserName == "default" {
		browserName = "chrome"
	}
	if browserName == "safari" || browserName == "firefox" || browserName == "opera" || browserName == "vivaldi" {
		return "", fmt.Errorf("import z iDokladu vyžaduje Chrome, Chromium, Edge nebo Brave; nastaveno %q", browserName)
	}
	aliases := map[string][]string{
		"chrome":   {"google-chrome", "google-chrome-stable", "chrome"},
		"chromium": {"chromium", "chromium-browser"},
		"edge":     {"microsoft-edge", "microsoft-edge-stable", "msedge"},
		"brave":    {"brave-browser", "brave"},
	}
	commands, ok := aliases[browserName]
	if !ok {
		return "", fmt.Errorf("neznámý prohlížeč %q pro import z iDokladu", browserName)
	}
	if goos != "darwin" {
		for _, command := range commands {
			if path, err := lookPath(command); err == nil {
				return path, nil
			}
		}
		return "", fmt.Errorf("prohlížeč %q nebyl nalezen", browserName)
	}

	appNames := map[string]string{
		"chrome": "Google Chrome.app", "chromium": "Chromium.app",
		"edge": "Microsoft Edge.app", "brave": "Brave Browser.app",
	}
	appName := appNames[browserName]
	paths := []string{filepath.Join("/Applications", appName)}
	if home, err := userHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Applications", appName))
	}
	for _, appPath := range paths {
		if _, err := stat(appPath); err == nil {
			binaryName := strings.TrimSuffix(appName, ".app")
			return filepath.Join(appPath, "Contents", "MacOS", binaryName), nil
		}
	}
	return "", fmt.Errorf("prohlížeč %q nebyl nalezen", browserName)
}
