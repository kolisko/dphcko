package invoice

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileResult struct {
	Path    string
	Invoice *Invoice
	Err     error
}

func ScanPeriod(dir string, opts ValidationOptions) []FileResult {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []FileResult{{Path: dir, Err: err}}
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	results := make([]FileResult, 0, len(paths))
	for _, path := range paths {
		inv, err := DecodePDF(path)
		if err == nil {
			err = Validate(inv, opts)
		}
		if err != nil {
			var parsed *Invoice
			if inv.Number != "" {
				parsed = &inv
			}
			results = append(results, FileResult{Path: path, Invoice: parsed, Err: err})
			continue
		}
		results = append(results, FileResult{Path: path, Invoice: &inv})
	}
	return results
}

func PeriodDirectory(root string, year int, month time.Month) string {
	return filepath.Join(root, fmt.Sprintf("%04d", year), fmt.Sprintf("%02d", month))
}

func PreviousMonth(now time.Time) (int, time.Month) {
	previous := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -1, 0)
	return previous.Year(), previous.Month()
}
