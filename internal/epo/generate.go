package epo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"dphcko/internal/config"
	"dphcko/internal/invoice"
	"dphcko/internal/tax"
)

const (
	DPHSchemaVersion = "03.01.03 (09.03.2026)"
	KHSchemaVersion  = "03.01.14 (09.03.2026)"
)

type commonPerson struct {
	TaxOffice       string `xml:"c_ufo,attr"`
	TaxOfficeBranch string `xml:"c_pracufo,attr,omitempty"`
	VATStem         string `xml:"dic,attr"`
	FirstName       string `xml:"jmeno,attr"`
	LastName        string `xml:"prijmeni,attr"`
	Street          string `xml:"ulice,attr"`
	HouseNumber     string `xml:"c_pop,attr"`
	OrientationNo   string `xml:"c_orient,attr,omitempty"`
	City            string `xml:"naz_obce,attr"`
	PostalCode      string `xml:"psc,attr"`
	Country         string `xml:"stat,attr"`
	SubjectType     string `xml:"typ_ds,attr"`
	Phone           string `xml:"c_telef,attr,omitempty"`
	Email           string `xml:"email,attr,omitempty"`
}

func person(profile config.Profile) commonPerson {
	return commonPerson{
		TaxOffice: profile.TaxOffice, TaxOfficeBranch: profile.TaxOfficeBranch,
		VATStem: invoice.VATStem(profile.VATID), FirstName: profile.FirstName, LastName: profile.LastName,
		Street: profile.Street, HouseNumber: profile.HouseNumber, OrientationNo: profile.OrientationNo,
		City: profile.City, PostalCode: strings.ReplaceAll(profile.PostalCode, " ", ""), Country: "ČESKÁ REPUBLIKA",
		SubjectType: "F", Phone: profile.Phone, Email: profile.Email,
	}
}

type dphEnvelope struct {
	XMLName xml.Name `xml:"Pisemnost"`
	Form    dphForm  `xml:"DPHDP3"`
}

type dphForm struct {
	Header dphHeader    `xml:"VetaD"`
	Person commonPerson `xml:"VetaP"`
	Row1   *dphRow1     `xml:"Veta1,omitempty"`
	Row6   *dphRow6     `xml:"Veta6,omitempty"`
}

type dphHeader struct {
	Created     string `xml:"d_poddp,attr"`
	Form        string `xml:"dapdph_forma,attr"`
	Document    string `xml:"dokument,attr"`
	TaxType     string `xml:"k_uladis,attr"`
	Month       int    `xml:"mesic,attr"`
	Year        int    `xml:"rok,attr"`
	Transaction string `xml:"trans,attr"`
	PayerType   string `xml:"typ_platce,attr"`
	NACE        string `xml:"c_okec,attr"`
}

type dphRow1 struct {
	Base int64 `xml:"obrat23,attr"`
	Tax  int64 `xml:"dan23,attr"`
}

type dphRow6 struct {
	OutputTax int64 `xml:"dan_zocelk,attr"`
	Deduction int64 `xml:"odp_zocelk,attr"`
	OwnTax    int64 `xml:"dano_da,attr"`
}

func DPH(profile config.Profile, year, month int, summary tax.Summary, created time.Time) ([]byte, error) {
	header := dphHeader{
		Created: created.Format("02.01.2006"), Form: "B", Document: "DP3", TaxType: "DPH",
		Month: month, Year: year, Transaction: "N", PayerType: "P", NACE: profile.NACE,
	}
	form := dphForm{Header: header, Person: person(profile)}
	if summary.Base != 0 || summary.Tax != 0 {
		base := summary.Base.WholeCrowns()
		taxAmount := summary.Tax.WholeCrowns()
		form.Header.Transaction = "A"
		form.Row1 = &dphRow1{Base: base, Tax: taxAmount}
		form.Row6 = &dphRow6{OutputTax: taxAmount, Deduction: 0, OwnTax: taxAmount}
	}
	return marshal(dphEnvelope{Form: form})
}

type khEnvelope struct {
	XMLName xml.Name `xml:"Pisemnost"`
	Form    khForm   `xml:"DPHKH1"`
}

type khForm struct {
	Header khHeader     `xml:"VetaD"`
	Person commonPerson `xml:"VetaP"`
	A4     []khA4       `xml:"VetaA4"`
	A5     *khA5        `xml:"VetaA5,omitempty"`
	C      *khC         `xml:"VetaC,omitempty"`
}

type khHeader struct {
	Document string `xml:"dokument,attr"`
	TaxType  string `xml:"k_uladis,attr"`
	Month    int    `xml:"mesic,attr"`
	Year     int    `xml:"rok,attr"`
	Created  string `xml:"d_poddp,attr"`
	Form     string `xml:"khdph_forma,attr"`
}

type khA4 struct {
	RecipientVATStem string `xml:"dic_odb,attr"`
	Number           string `xml:"c_evid_dd,attr"`
	TaxableDate      string `xml:"dppd,attr"`
	Base             string `xml:"zakl_dane1,attr"`
	Tax              string `xml:"dan1,attr"`
	Mode             string `xml:"kod_rezim_pl,attr"`
	BadDebt          string `xml:"zdph_44,attr"`
}

type khA5 struct {
	Base string `xml:"zakl_dane1,attr"`
	Tax  string `xml:"dan1,attr"`
}

type khC struct {
	Base string `xml:"obrat23,attr"`
}

func KH(profile config.Profile, year, month int, summary tax.Summary, created time.Time) ([]byte, error) {
	if len(summary.Invoices) == 0 {
		return nil, fmt.Errorf("kontrolní hlášení se bez podporovaných dokladů nevytváří")
	}
	form := khForm{
		Header: khHeader{Document: "KH1", TaxType: "DPH", Month: month, Year: year, Created: created.Format("02.01.2006"), Form: "B"},
		Person: person(profile),
		C:      &khC{Base: summary.Base.String()},
	}
	for _, inv := range summary.A4 {
		form.A4 = append(form.A4, khA4{
			RecipientVATStem: invoice.VATStem(inv.RecipientVATID), Number: inv.Number,
			TaxableDate: inv.TaxableDate.Format("02.01.2006"), Base: inv.TaxBase.String(), Tax: inv.Tax.String(),
			Mode: "0", BadDebt: "N",
		})
	}
	if len(summary.A5) > 0 {
		form.A5 = &khA5{Base: summary.A5Base.String(), Tax: summary.A5Tax.String()}
	}
	return marshal(khEnvelope{Form: form})
}

func marshal(value any) ([]byte, error) {
	body, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(xml.Header)
	out.Write(body)
	out.WriteByte('\n')
	return out.Bytes(), nil
}
