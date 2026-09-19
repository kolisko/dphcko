package idoklad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dphcko/internal/invoice"
	"github.com/chromedp/chromedp"
)

const stateFileName = "idoklad-state.json"

type State struct {
	LastDocumentNumber string `json:"last_document_number"`
	IssuerVATID        string `json:"issuer_vat_id"`
}

type SyncResult struct {
	PreviousDocumentNumber string
	NewestDocumentNumber   string
	RemoteDocuments        []string
	Imported               []string
	Skipped                []string
}

type remoteRow struct {
	Number string `json:"number"`
}

func Sync(root, browserName, issuerVATID string, progress func(string)) (SyncResult, error) {
	anchor, err := syncAnchor(root, issuerVATID)
	if err != nil {
		return SyncResult{}, err
	}
	downloadDir := filepath.Join(root, ".dphcko", "idoklad-stazene")
	session, err := Open(root, downloadDir, browserName)
	if err != nil {
		return SyncResult{}, err
	}
	defer session.Close()

	if progress != nil {
		progress("Čekám na přihlášení a seznam vydaných faktur v iDokladu…")
	}
	if err := session.WaitUntilReady(5 * time.Minute); err != nil {
		return SyncResult{}, err
	}
	if progress != nil {
		if anchor == "" {
			progress("Lokálně zatím není žádný doklad; zjišťuji kompletní seznam faktur…")
		} else {
			progress("Hledám faktury novější než " + anchor + "…")
		}
	}
	newer, err := session.DiscoverNewer(anchor)
	if err != nil {
		return SyncResult{}, err
	}
	result := SyncResult{PreviousDocumentNumber: anchor, RemoteDocuments: newer}
	if len(newer) == 0 {
		if anchor != "" {
			if err := saveState(root, State{LastDocumentNumber: anchor, IssuerVATID: issuerVATID}); err != nil {
				return SyncResult{}, err
			}
			result.NewestDocumentNumber = anchor
		}
		return result, nil
	}

	if progress != nil {
		progress(fmt.Sprintf("Stahuji %d nových faktur…", len(newer)))
	}
	if err := session.DownloadDocuments(downloadDir, newer, progress); err != nil {
		return SyncResult{}, err
	}
	if err := WaitForDownloads(downloadDir, 30*time.Second); err != nil {
		return SyncResult{}, err
	}
	imported, processed, err := ImportDownloadedPDFs(downloadDir, root, issuerVATID)
	if err != nil {
		return SyncResult{}, err
	}
	if missing := missingDocumentNumbers(newer, processed); len(missing) > 0 {
		return SyncResult{}, fmt.Errorf("iDoklad nevrátil ověřitelné PDF pro doklady: %s", strings.Join(missing, ", "))
	}
	newest := newer[0]
	if err := saveState(root, State{LastDocumentNumber: newest, IssuerVATID: issuerVATID}); err != nil {
		return SyncResult{}, err
	}
	result.NewestDocumentNumber = newest
	result.Imported = imported.Imported
	result.Skipped = imported.Skipped
	return result, nil
}

func LastDocumentNumber(root, issuerVATID string) (string, error) {
	return syncAnchor(root, issuerVATID)
}

func (s *Session) WaitUntilReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(s.context, timeout)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.WaitVisible(`a[data-ui-id="csw-invoice-document-number"]`, chromedp.ByQuery)); err != nil {
		return errors.New("nepodařilo se načíst seznam vydaných faktur; dokončete přihlášení v otevřeném Chromu a spusťte synchronizaci znovu")
	}
	return nil
}

func (s *Session) DiscoverNewer(anchor string) ([]string, error) {
	if err := s.goToFirstPage(); err != nil {
		return nil, err
	}
	var newer []string
	foundAnchor := anchor == ""
	seen := make(map[string]bool)
	for pageNumber := 1; pageNumber <= 1000; pageNumber++ {
		rows, err := s.currentRows()
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Number == "" || seen[row.Number] {
				continue
			}
			seen[row.Number] = true
			if anchor != "" && row.Number == anchor {
				foundAnchor = true
				continue
			}
			if anchor == "" || naturalLess(anchor, row.Number) {
				newer = append(newer, row.Number)
			}
		}
		moved, err := s.goToNextPage(firstRowNumber(rows))
		if err != nil {
			return nil, err
		}
		if !moved {
			if !foundAnchor {
				return nil, fmt.Errorf("poslední místní doklad %s nebyl v iDokladu nalezen; stav nebyl změněn", anchor)
			}
			sort.SliceStable(newer, func(i, j int) bool { return naturalLess(newer[j], newer[i]) })
			return newer, nil
		}
	}
	return nil, errors.New("iDoklad má neočekávaně mnoho stránek faktur")
}

func (s *Session) DownloadDocuments(downloadDir string, numbers []string, progress func(string)) error {
	wanted := make(map[string]bool, len(numbers))
	for _, number := range numbers {
		wanted[number] = true
	}
	if err := s.goToFirstPage(); err != nil {
		return err
	}
	remaining := len(numbers)
	for pageNumber := 1; pageNumber <= 1000 && remaining > 0; pageNumber++ {
		rows, err := s.currentRows()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if !wanted[row.Number] {
				continue
			}
			if progress != nil {
				progress("Stahuji fakturu " + row.Number + "…")
			}
			before, err := pdfFiles(downloadDir)
			if err != nil {
				return err
			}
			if err := s.clickPDF(row.Number); err != nil {
				return err
			}
			if err := waitForNewPDF(downloadDir, before, 60*time.Second); err != nil {
				return fmt.Errorf("stažení faktury %s: %w", row.Number, err)
			}
			delete(wanted, row.Number)
			remaining--
		}
		if remaining == 0 {
			return nil
		}
		moved, err := s.goToNextPage(firstRowNumber(rows))
		if err != nil {
			return err
		}
		if !moved {
			break
		}
	}
	missing := make([]string, 0, len(wanted))
	for number := range wanted {
		missing = append(missing, number)
	}
	sort.Strings(missing)
	return fmt.Errorf("v seznamu iDokladu nebyly při stahování nalezeny doklady: %s", strings.Join(missing, ", "))
}

func (s *Session) currentRows() ([]remoteRow, error) {
	var rows []remoteRow
	err := chromedp.Run(s.context, chromedp.Evaluate(`Array.from(document.querySelectorAll('a[data-ui-id="csw-invoice-document-number"]')).map(a => ({number: (a.textContent || '').trim()}))`, &rows))
	return rows, err
}

func (s *Session) goToFirstPage() error {
	rows, err := s.currentRows()
	if err != nil {
		return err
	}
	previousFirst := firstRowNumber(rows)
	var clicked bool
	if err := chromedp.Run(s.context, chromedp.Evaluate(`(() => { const b = document.querySelector('.k-pager button.k-pager-first:not(.k-disabled)'); if (!b) return false; b.click(); return true })()`, &clicked)); err != nil {
		return err
	}
	if !clicked {
		return nil
	}
	return s.waitForFirstRowChange(previousFirst)
}

func (s *Session) goToNextPage(previousFirst string) (bool, error) {
	var clicked bool
	if err := chromedp.Run(s.context, chromedp.Evaluate(`(() => { const icon = document.querySelector('.k-pager button:not(.k-disabled) .icon-chevron-right'); if (!icon) return false; icon.closest('button').click(); return true })()`, &clicked)); err != nil {
		return false, err
	}
	if !clicked {
		return false, nil
	}
	return true, s.waitForFirstRowChange(previousFirst)
}

func (s *Session) waitForFirstRowChange(previous string) error {
	deadline := time.Now().Add(15 * time.Second)
	for {
		rows, err := s.currentRows()
		if err != nil {
			return err
		}
		if len(rows) > 0 && (previous == "" || rows[0].Number != previous) {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("iDoklad nezměnil stránku seznamu faktur")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Session) clickPDF(number string) error {
	encoded, _ := json.Marshal(number)
	script := `(() => { const number = ` + string(encoded) + `; const a = Array.from(document.querySelectorAll('a[data-ui-id="csw-invoice-document-number"]')).find(x => (x.textContent || '').trim() === number); if (!a) return false; const row = a.closest('tr, [role="row"]'); if (!row) return false; const button = row.querySelector('button[data-ui-id="csw-row-action-pdf-export"]'); if (!button) return false; button.click(); return true })()`
	var clicked bool
	if err := chromedp.Run(s.context, chromedp.Evaluate(script, &clicked)); err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("u dokladu %s nebyla nalezena akce PDF", number)
	}
	return nil
}

func syncAnchor(root, issuerVATID string) (string, error) {
	state, err := loadState(root)
	if err != nil {
		return "", err
	}
	if state.LastDocumentNumber != "" && invoice.VATStem(state.IssuerVATID) == invoice.VATStem(issuerVATID) {
		return state.LastDocumentNumber, nil
	}
	return latestLocalDocumentNumber(root, issuerVATID)
}

func latestLocalDocumentNumber(root, issuerVATID string) (string, error) {
	var numbers []string
	years, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, yearEntry := range years {
		if !yearEntry.IsDir() || len(yearEntry.Name()) != 4 {
			continue
		}
		if _, err := strconv.Atoi(yearEntry.Name()); err != nil {
			continue
		}
		yearDir := filepath.Join(root, yearEntry.Name())
		months, err := os.ReadDir(yearDir)
		if err != nil {
			return "", err
		}
		for _, monthEntry := range months {
			if !monthEntry.IsDir() || len(monthEntry.Name()) != 2 {
				continue
			}
			month, err := strconv.Atoi(monthEntry.Name())
			if err != nil || month < 1 || month > 12 {
				continue
			}
			periodDir := filepath.Join(yearDir, monthEntry.Name())
			files, err := os.ReadDir(periodDir)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".pdf") {
					continue
				}
				parsed, err := invoice.DecodePDF(filepath.Join(periodDir, file.Name()))
				if err == nil && parsed.Number != "" && invoice.VATStem(parsed.IssuerVATID) == invoice.VATStem(issuerVATID) {
					numbers = append(numbers, parsed.Number)
				}
			}
		}
	}
	if len(numbers) == 0 {
		return "", nil
	}
	sort.SliceStable(numbers, func(i, j int) bool { return naturalLess(numbers[i], numbers[j]) })
	return numbers[len(numbers)-1], nil
}

func naturalLess(left, right string) bool {
	leftFolded := strings.ToLower(left)
	rightFolded := strings.ToLower(right)
	for li, ri := 0, 0; li < len(leftFolded) && ri < len(rightFolded); {
		leftDigit := leftFolded[li] >= '0' && leftFolded[li] <= '9'
		rightDigit := rightFolded[ri] >= '0' && rightFolded[ri] <= '9'
		if leftDigit && rightDigit {
			leftEnd, rightEnd := li, ri
			for leftEnd < len(leftFolded) && leftFolded[leftEnd] >= '0' && leftFolded[leftEnd] <= '9' {
				leftEnd++
			}
			for rightEnd < len(rightFolded) && rightFolded[rightEnd] >= '0' && rightFolded[rightEnd] <= '9' {
				rightEnd++
			}
			leftNumber := strings.TrimLeft(leftFolded[li:leftEnd], "0")
			rightNumber := strings.TrimLeft(rightFolded[ri:rightEnd], "0")
			if leftNumber == "" {
				leftNumber = "0"
			}
			if rightNumber == "" {
				rightNumber = "0"
			}
			if len(leftNumber) != len(rightNumber) {
				return len(leftNumber) < len(rightNumber)
			}
			if leftNumber != rightNumber {
				return leftNumber < rightNumber
			}
			li, ri = leftEnd, rightEnd
			continue
		}
		if leftFolded[li] != rightFolded[ri] {
			return leftFolded[li] < rightFolded[ri]
		}
		li++
		ri++
	}
	return len(leftFolded) < len(rightFolded)
}

func statePath(root string) string { return filepath.Join(root, ".dphcko", stateFileName) }

func loadState(root string) (State, error) {
	data, err := os.ReadFile(statePath(root))
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("načtení stavu iDokladu: %w", err)
	}
	return state, nil
}

func saveState(root string, state State) error {
	dir := filepath.Join(root, ".dphcko")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".idoklad-state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, statePath(root))
}

func pdfFiles(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			files[entry.Name()] = true
		}
	}
	return files, nil
}

func waitForNewPDF(dir string, before map[string]bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		after, err := pdfFiles(dir)
		if err != nil {
			return err
		}
		for name := range after {
			if !before[name] {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return errors.New("PDF se v cílové složce neobjevilo")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func missingDocumentNumbers(want, got []string) []string {
	seen := make(map[string]bool, len(got))
	for _, number := range got {
		seen[number] = true
	}
	var missing []string
	for _, number := range want {
		if !seen[number] {
			missing = append(missing, number)
		}
	}
	return missing
}

func firstRowNumber(rows []remoteRow) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[0].Number
}
