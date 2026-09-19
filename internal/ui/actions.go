package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/epo"
	"dphcko/internal/idoklad"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
)

func ImportFromIDoklad(root string, cfg config.Config) (idoklad.SyncResult, error) {
	return idoklad.Sync(root, cfg.App.Browser, cfg.Profile.VATID, func(message string) {
		fmt.Println(message)
	})
}

type GenerateOptions struct {
	ConsumerDocuments map[string]bool
	Overwrite         bool
}

func Generate(root string, cfg config.Config, period Period, now time.Time, options GenerateOptions) (epo.OutputPaths, error) {
	results := invoice.ScanPeriod(period.Dir, invoice.ValidationOptions{IssuerVATID: cfg.Profile.VATID, Year: period.Year, Month: period.Month})
	var invoices []invoice.Invoice
	for _, result := range results {
		if result.Err != nil && result.Invoice != nil && result.Invoice.Total > invoice.A4Threshold && result.Invoice.RecipientVATID == "" && options.ConsumerDocuments[result.Path] {
			result.Invoice.ConsumerConfirmed = true
			result.Err = invoice.Validate(*result.Invoice, invoice.ValidationOptions{IssuerVATID: cfg.Profile.VATID, Year: period.Year, Month: period.Month})
		}
		if result.Err != nil {
			return epo.OutputPaths{}, fmt.Errorf("%s: %w", filepath.Base(result.Path), result.Err)
		}
		invoices = append(invoices, *result.Invoice)
	}
	summary, err := tax.Build(invoices)
	if err != nil {
		return epo.OutputPaths{}, err
	}
	existing, err := epo.ExistingPeriodOutputs(period.Dir, period.Year, int(period.Month))
	if err != nil {
		return epo.OutputPaths{}, err
	}
	if len(existing) == 0 {
		return epo.WritePeriod(period.Dir, cfg.Profile, period.Year, int(period.Month), summary, now)
	}
	if !options.Overwrite {
		return epo.OutputPaths{}, errors.New("existující výstupy zůstaly beze změny")
	}
	return epo.ReplacePeriod(period.Dir, cfg.Profile, period.Year, int(period.Month), summary, now)
}
