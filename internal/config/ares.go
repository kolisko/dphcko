package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const aresBaseURL = "https://ares.gov.cz/ekonomicke-subjekty-v-be/rest/ekonomicke-subjekty/"

type aresResponse struct {
	ICO          string      `json:"ico"`
	BusinessName string      `json:"obchodniJmeno"`
	VATID        string      `json:"dic"`
	TaxOffice    string      `json:"financniUrad"`
	NACE         []string    `json:"czNace"`
	NACE2008     []string    `json:"czNace2008"`
	RegisteredAt aresAddress `json:"sidlo"`
}

type aresAddress struct {
	Street        string `json:"nazevUlice"`
	City          string `json:"nazevObce"`
	CityPart      string `json:"nazevCastiObce"`
	Region        string `json:"nazevKraje"`
	HouseNumber   int    `json:"cisloDomovni"`
	OrientationNo int    `json:"cisloOrientacni"`
	OrientationCh string `json:"cisloOrientacniPismeno"`
	PostalCode    int    `json:"psc"`
}

func LookupARES(ctx context.Context, ico string) (Profile, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	return lookupARES(ctx, client, aresBaseURL, ico)
}

func lookupARES(ctx context.Context, client *http.Client, baseURL, ico string) (Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+ico, nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("ARES není dostupný: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("ARES vrátil stav %s", resp.Status)
	}
	var data aresResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Profile{}, fmt.Errorf("odpověď ARES: %w", err)
	}
	nace := data.NACE
	if len(nace) == 0 {
		nace = data.NACE2008
	}
	profile := Profile{
		ICO:         data.ICO,
		VATID:       strings.ToUpper(data.VATID),
		Street:      data.RegisteredAt.Street,
		HouseNumber: intString(data.RegisteredAt.HouseNumber),
		City:        data.RegisteredAt.City,
		PostalCode:  fmt.Sprintf("%05d", data.RegisteredAt.PostalCode),
		Country:     "CZ",
		TaxOffice:   epoOfficeForRegion(data.RegisteredAt.Region),
	}
	if data.RegisteredAt.OrientationNo > 0 {
		profile.OrientationNo = fmt.Sprintf("%d%s", data.RegisteredAt.OrientationNo, data.RegisteredAt.OrientationCh)
	}
	if profile.Street == "" {
		profile.Street = data.RegisteredAt.CityPart
	}
	if len(nace) > 0 {
		profile.NACE = nace[0]
	}
	first, last := splitPersonName(data.BusinessName)
	profile.FirstName, profile.LastName = first, last
	return profile, nil
}

func epoOfficeForRegion(region string) string {
	// ARES' financniUrad is a different (legacy) code system. EPO's c_ufo
	// identifies the 14 regional offices with codes 451–464.
	return map[string]string{
		"Hlavní město Praha":   "451",
		"Středočeský kraj":     "452",
		"Jihočeský kraj":       "453",
		"Plzeňský kraj":        "454",
		"Karlovarský kraj":     "455",
		"Ústecký kraj":         "456",
		"Liberecký kraj":       "457",
		"Královéhradecký kraj": "458",
		"Pardubický kraj":      "459",
		"Kraj Vysočina":        "460",
		"Jihomoravský kraj":    "461",
		"Olomoucký kraj":       "462",
		"Moravskoslezský kraj": "463",
		"Zlínský kraj":         "464",
	}[region]
}

func intString(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func splitPersonName(value string) (string, string) {
	parts := strings.Fields(value)
	if len(parts) < 2 {
		return "", value
	}
	// ARES may prefix academic titles. The wizard always asks for confirmation.
	for len(parts) > 2 && (strings.HasSuffix(parts[0], ".") || parts[0] == "Ing" || parts[0] == "Mgr") {
		parts = parts[1:]
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}
