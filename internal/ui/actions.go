package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"charm.land/huh/v2"
	"dphcko/internal/config"
	"dphcko/internal/epo"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
)

func CreatePeriod(root string, now time.Time) (Period, error) {
	year, month := invoice.PreviousMonth(now)
	yearValue := strconv.Itoa(year)
	monthValue := fmt.Sprintf("%02d", month)
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Rok").Value(&yearValue).Validate(func(value string) error {
			n, err := strconv.Atoi(value)
			if err != nil || n < 2000 || n > 2100 {
				return errors.New("zadejte rok 2000 až 2100")
			}
			return nil
		}),
		huh.NewInput().Title("Měsíc").Description("Předvolený je poslední dokončený měsíc.").Value(&monthValue).Validate(func(value string) error {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 12 {
				return errors.New("zadejte měsíc 1 až 12")
			}
			return nil
		}),
	))
	if err := form.Run(); err != nil {
		return Period{}, err
	}
	year, _ = strconv.Atoi(yearValue)
	monthNumber, _ := strconv.Atoi(monthValue)
	dir := invoice.PeriodDirectory(root, year, time.Month(monthNumber))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Period{}, err
	}
	return Period{Year: year, Month: time.Month(monthNumber), Dir: dir}, nil
}

func Generate(root string, cfg config.Config, period Period, now time.Time) (epo.OutputPaths, error) {
	results := invoice.ScanPeriod(period.Dir, invoice.ValidationOptions{IssuerVATID: cfg.Profile.VATID, Year: period.Year, Month: period.Month})
	var invoices []invoice.Invoice
	for _, result := range results {
		if result.Err != nil && result.Invoice != nil && result.Invoice.Total > invoice.A4Threshold && result.Invoice.RecipientVATID == "" {
			confirmed := false
			err := huh.NewConfirm().
				Title(fmt.Sprintf("%s je nad 10 000 Kč a nemá DIČ odběratele. Je odběratel koncový spotřebitel?", filepath.Base(result.Path))).
				Affirmative("Ano, zařadit do A.5").Negative("Ne, zastavit").Value(&confirmed).Run()
			if err != nil {
				return epo.OutputPaths{}, err
			}
			if confirmed {
				result.Invoice.ConsumerConfirmed = true
				result.Err = invoice.Validate(*result.Invoice, invoice.ValidationOptions{IssuerVATID: cfg.Profile.VATID, Year: period.Year, Month: period.Month})
			}
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
	return epo.WritePeriod(period.Dir, cfg.Profile, period.Year, int(period.Month), summary, now)
}
