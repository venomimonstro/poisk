package organizations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

var (
	ErrInvalidRow = errors.New("invalid organization source row")
	ErrInvalidBatch = errors.New("invalid organization import batch")
	sourceKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	categoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)
	cityPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)
)

const maxRawPayloadBytes = 64 << 10

type SourceRow struct {
	SourceRecordID string   `json:"source_record_id"`
	Name           string   `json:"name"`
	Phone          string   `json:"phone,omitempty"`
	Website        string   `json:"website,omitempty"`
	Address        string   `json:"address,omitempty"`
	CityKey        string   `json:"city_key,omitempty"`
	CategoryKey    string   `json:"category_key,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
}

type NormalizedRow struct {
	SourceRecordID    string
	Name              string
	NormalizedName    string
	Phone             string
	Website           string
	Address           string
	NormalizedAddress string
	CityKey           string
	CategoryKey       string
	Latitude          *float64
	Longitude         *float64
	PayloadHash       string
	RawPayload        []byte
}

func NormalizeSourceRow(row SourceRow) (NormalizedRow, error) {
	rawPayload, err := json.Marshal(row)
	if err != nil || len(rawPayload) == 0 || len(rawPayload) > maxRawPayloadBytes { return NormalizedRow{}, ErrInvalidRow }

	row.SourceRecordID = strings.TrimSpace(row.SourceRecordID)
	row.Name = compactText(row.Name)
	if row.SourceRecordID == "" || len(row.SourceRecordID) > 256 || row.Name == "" || utf8.RuneCountInString(row.Name) > 300 {
		return NormalizedRow{}, ErrInvalidRow
	}

	normalizedName := normalizeIdentityText(row.Name)
	if normalizedName == "" { return NormalizedRow{}, ErrInvalidRow }
	phone, err := normalizePhone(row.Phone)
	if err != nil { return NormalizedRow{}, err }
	website, err := normalizeWebsite(row.Website)
	if err != nil { return NormalizedRow{}, err }
	address := compactText(row.Address)
	if utf8.RuneCountInString(address) > 700 { return NormalizedRow{}, ErrInvalidRow }
	normalizedAddress := normalizeIdentityText(address)
	city := strings.ToLower(strings.TrimSpace(row.CityKey))
	if city != "" && !cityPattern.MatchString(city) { return NormalizedRow{}, ErrInvalidRow }
	category := strings.ToLower(strings.TrimSpace(row.CategoryKey))
	if category != "" && !categoryPattern.MatchString(category) { return NormalizedRow{}, ErrInvalidRow }
	if (row.Latitude == nil) != (row.Longitude == nil) { return NormalizedRow{}, ErrInvalidRow }
	if row.Latitude != nil && (*row.Latitude < -90 || *row.Latitude > 90 || *row.Longitude < -180 || *row.Longitude > 180) { return NormalizedRow{}, ErrInvalidRow }

	canonical, err := json.Marshal(struct {
		SourceRecordID string `json:"source_record_id"`
		Name string `json:"name"`
		Phone string `json:"phone,omitempty"`
		Website string `json:"website,omitempty"`
		Address string `json:"address,omitempty"`
		CityKey string `json:"city_key,omitempty"`
		CategoryKey string `json:"category_key,omitempty"`
		Latitude *float64 `json:"latitude,omitempty"`
		Longitude *float64 `json:"longitude,omitempty"`
	}{row.SourceRecordID,row.Name,phone,website,address,city,category,row.Latitude,row.Longitude})
	if err != nil { return NormalizedRow{}, err }
	sum := sha256.Sum256(canonical)
	return NormalizedRow{
		SourceRecordID: row.SourceRecordID, Name: row.Name, NormalizedName: normalizedName,
		Phone: phone, Website: website, Address: address, NormalizedAddress: normalizedAddress,
		CityKey: city, CategoryKey: category, Latitude: row.Latitude, Longitude: row.Longitude,
		PayloadHash: hex.EncodeToString(sum[:]), RawPayload: rawPayload,
	}, nil
}

func ValidSourceKey(value string) bool { return sourceKeyPattern.MatchString(strings.TrimSpace(value)) }

func compactText(value string) string { return strings.Join(strings.Fields(strings.TrimSpace(value)), " ") }

func normalizeIdentityText(value string) string {
	value = strings.ToLower(compactText(value))
	var b strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 { b.WriteByte(' ') }
			space = false
			b.WriteRune(r)
		} else {
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizePhone(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" { return "", nil }
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' { digits.WriteRune(r) }
	}
	value := digits.String()
	if len(value) < 7 || len(value) > 15 { return "", ErrInvalidRow }
	if len(value) == 11 && value[0] == '8' { value = "7" + value[1:] }
	return "+" + value, nil
}

func normalizeWebsite(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" { return "", nil }
	if !strings.Contains(raw, "://") { raw = "https://" + raw }
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" { return "", ErrInvalidRow }
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	host, err = idna.Lookup.ToASCII(host)
	if err != nil || host == "" || strings.ContainsAny(host, " /\\") { return "", ErrInvalidRow }
	port := u.Port()
	if port != "" && !((u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443")) { return "", ErrInvalidRow }
	u.Scheme = "https"
	u.Host = host
	u.RawQuery = ""
	u.Fragment = ""
	u.RawPath = ""
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" { u.Path = "/" }
	return u.String(), nil
}
