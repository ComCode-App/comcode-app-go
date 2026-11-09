package vcard

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Parser parses RFC-compliant vCard files (2.1, 3.0, 4.0) into Contact structures.
type Parser struct{}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	return &Parser{}
}

// ParseReader parses all vCards from an io.Reader.
func (p *Parser) ParseReader(r io.Reader) ([]Contact, error) {
	var contacts []Contact
	scanner := bufio.NewScanner(r)

	// Custom buffer for large vCards with embedded photos
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // up to 10MB

	var currentRawLines []string
	inVCard := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.EqualFold(trimmed, "BEGIN:VCARD") {
			inVCard = true
			currentRawLines = []string{line}
			continue
		}

		if inVCard {
			currentRawLines = append(currentRawLines, line)
			if strings.EqualFold(trimmed, "END:VCARD") {
				inVCard = false
				contact, err := p.parseSingleRaw(currentRawLines)
				if err == nil && contact != nil {
					contacts = append(contacts, *contact)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading vcard stream: %w", err)
	}

	return contacts, nil
}

// Parse parses a string containing one or more vCards.
func (p *Parser) Parse(vcf string) ([]Contact, error) {
	return p.ParseReader(strings.NewReader(vcf))
}

// ParseSingle parses the first vCard found in a string.
func (p *Parser) ParseSingle(vcf string) (*Contact, error) {
	contacts, err := p.Parse(vcf)
	if err != nil {
		return nil, err
	}
	if len(contacts) == 0 {
		return nil, fmt.Errorf("no valid vcard found")
	}
	return &contacts[0], nil
}

// Raw property line representation
type vcardProp struct {
	Group  string
	Name   string
	Params map[string]string
	Value  string
}

func (p *Parser) parseSingleRaw(rawLines []string) (*Contact, error) {
	unfolded := unfoldLines(rawLines)
	groups := make(map[string]map[string]string) // group -> key -> value (e.g. item1 -> X-ABLabel -> Work)
	var props []vcardProp

	contact := &Contact{
		CustomFields: make(map[string]string),
		Version:      Version30,
	}

	for _, line := range unfolded {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "BEGIN:VCARD") || strings.EqualFold(line, "END:VCARD") {
			continue
		}

		prop := parsePropLine(line)
		if prop.Name == "" {
			continue
		}

		if prop.Group != "" {
			if _, ok := groups[prop.Group]; !ok {
				groups[prop.Group] = make(map[string]string)
			}
			groups[prop.Group][prop.Name] = prop.Value
		}

		props = append(props, prop)
	}

	// First pass for version
	for _, prop := range props {
		if prop.Name == "VERSION" {
			if strings.HasPrefix(prop.Value, "4") {
				contact.Version = Version40
			} else {
				contact.Version = Version30
			}
			break
		}
	}

	// Second pass for properties
	for _, prop := range props {
		groupLabel := ""
		if prop.Group != "" && groups[prop.Group] != nil {
			if lbl, exists := groups[prop.Group]["X-ABLABEL"]; exists {
				groupLabel = lbl
			} else if lbl, exists := groups[prop.Group]["X-ABRELATEDNAMES"]; exists {
				groupLabel = lbl
			}
		}

		switch prop.Name {
		case "VERSION":
			// Handled above

		case "UID":
			contact.UID = cleanUID(prop.Value)

		case "FN":
			contact.DisplayName = unescapeValue(prop.Value)

		case "N":
			parseStructuredName(prop.Value, contact)

		case "NICKNAME":
			contact.Nickname = unescapeValue(prop.Value)

		case "ORG":
			parseOrg(prop.Value, contact)

		case "TITLE":
			contact.Title = unescapeValue(prop.Value)

		case "ROLE":
			contact.Role = unescapeValue(prop.Value)

		case "TEL":
			phone := parsePhone(prop, groupLabel)
			if phone.Number != "" {
				contact.Phones = append(contact.Phones, phone)
			}

		case "EMAIL":
			email := parseEmail(prop, groupLabel)
			if email.Address != "" {
				contact.Emails = append(contact.Emails, email)
			}

		case "ADR":
			addr := parseAddress(prop, groupLabel)
			contact.Addresses = append(contact.Addresses, addr)

		case "NOTE", "COMMENT":
			noteVal := unescapeValue(prop.Value)
			if contact.Bio == "" {
				contact.Bio = noteVal
			} else if contact.Note == "" {
				contact.Note = noteVal
			} else {
				contact.Note += "\n" + noteVal
			}

		case "BDAY":
			contact.Birthday = parseBirthday(prop.Value)

		case "GENDER":
			contact.Gender = prop.Value

		case "PHOTO":
			parsePhoto(prop, contact)

		case "URL":
			link := parseSocialOrURL(prop, groupLabel)
			if link.URL != "" {
				contact.SocialLinks = append(contact.SocialLinks, link)
			}

		case "X-SOCIALPROFILE":
			link := parseXSocialProfile(prop, groupLabel)
			if link.URL != "" {
				contact.SocialLinks = append(contact.SocialLinks, link)
			}

		default:
			// Check for special X- properties
			if strings.HasPrefix(prop.Name, "X-") && !strings.HasPrefix(prop.Name, "X-AB") {
				lowerName := strings.ToLower(prop.Name)
				if isSocialPlatformName(lowerName) {
					urlVal := strings.TrimSpace(prop.Value)
					if urlVal != "" {
						platform := strings.TrimPrefix(lowerName, "x-")
						contact.SocialLinks = append(contact.SocialLinks, SocialLink{
							Platform: platform,
							Label:    cases.Title(language.English).String(platform),
							URL:      urlVal,
						})
					}
				} else {
					contact.CustomFields[prop.Name] = unescapeValue(prop.Value)
				}
			}
		}
	}

	// Post-processing & cleanup
	if contact.DisplayName == "" {
		contact.DisplayName = strings.TrimSpace(fmt.Sprintf("%s %s %s %s %s",
			contact.Prefix, contact.FirstName, contact.MiddleName, contact.LastName, contact.Suffix))
		contact.DisplayName = strings.Join(strings.Fields(contact.DisplayName), " ")
	}

	// Default primary flags if none set
	if len(contact.Phones) > 0 {
		hasPrimary := false
		for _, ph := range contact.Phones {
			if ph.IsPrimary {
				hasPrimary = true
				break
			}
		}
		if !hasPrimary {
			contact.Phones[0].IsPrimary = true
		}
	}

	if len(contact.Emails) > 0 {
		hasPrimary := false
		for _, em := range contact.Emails {
			if em.IsPrimary {
				hasPrimary = true
				break
			}
		}
		if !hasPrimary {
			contact.Emails[0].IsPrimary = true
		}
	}

	if len(contact.Addresses) > 0 {
		hasPrimary := false
		for _, ad := range contact.Addresses {
			if ad.IsPrimary {
				hasPrimary = true
				break
			}
		}
		if !hasPrimary {
			contact.Addresses[0].IsPrimary = true
		}
	}

	return contact, nil
}

// Line unfolding according to RFC 5545 / RFC 6350 / RFC 2426
func unfoldLines(rawLines []string) []string {
	var unfolded []string
	var current strings.Builder

	for _, line := range rawLines {
		if len(line) == 0 {
			continue
		}

		// Check for RFC line continuation (starts with space or tab)
		if (line[0] == ' ' || line[0] == '\t') && current.Len() > 0 {
			current.WriteString(line[1:])
		} else {
			if current.Len() > 0 {
				unfolded = append(unfolded, current.String())
				current.Reset()
			}
			current.WriteString(line)
		}
	}

	if current.Len() > 0 {
		unfolded = append(unfolded, current.String())
	}

	return unfolded
}

func parsePropLine(line string) vcardProp {
	colonIdx := strings.Index(line, ":")
	if colonIdx == -1 {
		return vcardProp{}
	}

	left := line[:colonIdx]
	val := line[colonIdx+1:]

	prop := vcardProp{
		Params: make(map[string]string),
		Value:  val,
	}

	// Split parameters by semicolon
	parts := strings.Split(left, ";")
	namePart := parts[0]

	// Check for group: e.g. "item1.TEL"
	if dotIdx := strings.Index(namePart, "."); dotIdx != -1 {
		prop.Group = strings.ToUpper(strings.TrimSpace(namePart[:dotIdx]))
		prop.Name = strings.ToUpper(strings.TrimSpace(namePart[dotIdx+1:]))
	} else {
		prop.Name = strings.ToUpper(strings.TrimSpace(namePart))
	}

	// Parse parameters
	for i := 1; i < len(parts); i++ {
		paramStr := strings.TrimSpace(parts[i])
		if paramStr == "" {
			continue
		}

		if eqIdx := strings.Index(paramStr, "="); eqIdx != -1 {
			k := strings.ToUpper(strings.TrimSpace(paramStr[:eqIdx]))
			v := strings.Trim(strings.TrimSpace(paramStr[eqIdx+1:]), `"`)
			prop.Params[k] = v
		} else {
			// Bare parameter (often TYPE in vCard 2.1 e.g. TEL;CELL;VOICE:)
			upper := strings.ToUpper(paramStr)
			if isEncodingParam(upper) {
				prop.Params["ENCODING"] = upper
			} else if isCharsetParam(upper) {
				prop.Params["CHARSET"] = upper
			} else {
				if existingType, ok := prop.Params["TYPE"]; ok {
					prop.Params["TYPE"] = existingType + "," + upper
				} else {
					prop.Params["TYPE"] = upper
				}
			}
		}
	}

	// Quoted-Printable decoding if requested
	encoding := strings.ToUpper(prop.Params["ENCODING"])
	if encoding == "QUOTED-PRINTABLE" || encoding == "QP" || encoding == "B" {
		if encoding == "QUOTED-PRINTABLE" || encoding == "QP" {
			prop.Value = decodeQuotedPrintable(prop.Value)
		}
	}

	return prop
}

func decodeQuotedPrintable(s string) string {
	r := quotedprintable.NewReader(strings.NewReader(s))
	dec, err := io.ReadAll(r)
	if err != nil {
		return s
	}
	return string(dec)
}

func isEncodingParam(s string) bool {
	return s == "BASE64" || s == "B" || s == "QUOTED-PRINTABLE" || s == "QP" || s == "7BIT" || s == "8BIT"
}

func isCharsetParam(s string) bool {
	return strings.HasPrefix(s, "UTF") || strings.HasPrefix(s, "ISO") || strings.HasPrefix(s, "WINDOWS")
}

func parseStructuredName(val string, contact *Contact) {
	parts := strings.Split(val, ";")
	if len(parts) > 0 {
		contact.LastName = unescapeValue(parts[0])
	}
	if len(parts) > 1 {
		contact.FirstName = unescapeValue(parts[1])
	}
	if len(parts) > 2 {
		contact.MiddleName = unescapeValue(parts[2])
	}
	if len(parts) > 3 {
		contact.Prefix = unescapeValue(parts[3])
	}
	if len(parts) > 4 {
		contact.Suffix = unescapeValue(parts[4])
	}
}

func parseOrg(val string, contact *Contact) {
	parts := strings.Split(val, ";")
	if len(parts) > 0 {
		contact.CompanyName = unescapeValue(parts[0])
	}
	if len(parts) > 1 {
		contact.Department = unescapeValue(parts[1])
	}
}

func parsePhone(prop vcardProp, groupLabel string) Phone {
	typeVal := strings.ToUpper(prop.Params["TYPE"])
	isPrimary := strings.Contains(typeVal, "PREF")

	categoryType := "mobile"
	if strings.Contains(typeVal, "WORK") {
		categoryType = "work"
	} else if strings.Contains(typeVal, "HOME") {
		categoryType = "home"
	} else if strings.Contains(typeVal, "FAX") {
		categoryType = "fax"
	} else if strings.Contains(typeVal, "MAIN") {
		categoryType = "main"
	}

	label := groupLabel
	if label == "" {
		label = prop.Params["X-ABLABEL"]
	}

	cleanNumber := strings.TrimSpace(prop.Value)

	return Phone{
		Type:      categoryType,
		Label:     label,
		Number:    cleanNumber,
		IsPrimary: isPrimary,
	}
}

func parseEmail(prop vcardProp, groupLabel string) Email {
	typeVal := strings.ToUpper(prop.Params["TYPE"])
	isPrimary := strings.Contains(typeVal, "PREF")

	emailType := "work"
	if strings.Contains(typeVal, "HOME") {
		emailType = "home"
	}

	label := groupLabel
	if label == "" {
		label = prop.Params["X-ABLABEL"]
	}

	return Email{
		Type:      emailType,
		Label:     label,
		Address:   strings.TrimSpace(prop.Value),
		IsPrimary: isPrimary,
	}
}

func parseAddress(prop vcardProp, groupLabel string) Address {
	typeVal := strings.ToUpper(prop.Params["TYPE"])
	isPrimary := strings.Contains(typeVal, "PREF")

	addrType := "work"
	if strings.Contains(typeVal, "HOME") {
		addrType = "home"
	} else if strings.Contains(typeVal, "OTHER") {
		addrType = "other"
	}

	label := groupLabel
	if label == "" {
		label = prop.Params["X-ABLABEL"]
	}

	parts := strings.Split(prop.Value, ";")
	// ADR RFC format: postOfficeBox;extended;street;city;region;postalCode;country
	street := ""
	extended := ""
	city := ""
	region := ""
	postalCode := ""
	country := ""

	if len(parts) > 1 {
		extended = unescapeValue(parts[1])
	}
	if len(parts) > 2 {
		street = unescapeValue(parts[2])
	}
	if len(parts) > 3 {
		city = unescapeValue(parts[3])
	}
	if len(parts) > 4 {
		region = unescapeValue(parts[4])
	}
	if len(parts) > 5 {
		postalCode = unescapeValue(parts[5])
	}
	if len(parts) > 6 {
		country = unescapeValue(parts[6])
	}

	return Address{
		Type:       addrType,
		Label:      label,
		Street:     street,
		Extended:   extended,
		City:       city,
		Region:     region,
		PostalCode: postalCode,
		Country:    country,
		IsPrimary:  isPrimary,
	}
}

func parseBirthday(val string) *time.Time {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}

	formats := []string{
		"2006-01-02",
		"20060102",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, val); err == nil {
			return &t
		}
	}

	return nil
}

func parsePhoto(prop vcardProp, contact *Contact) {
	val := strings.TrimSpace(prop.Value)

	// vCard 4.0 data URI: data:image/jpeg;base64,...
	if strings.HasPrefix(val, "data:") {
		idx := strings.Index(val, ";base64,")
		if idx != -1 {
			mime := val[5:idx]
			b64Str := val[idx+8:]
			data, err := base64.StdEncoding.DecodeString(cleanBase64(b64Str))
			if err == nil && len(data) > 0 {
				contact.PhotoBytes = data
				contact.PhotoMimeType = mime
				return
			}
		}
	}

	// Direct URI
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		contact.AvatarURL = val
		return
	}

	// vCard 3.0 / 2.1 inline base64
	clean := cleanBase64(val)
	if data, err := base64.StdEncoding.DecodeString(clean); err == nil && len(data) > 0 {
		contact.PhotoBytes = data
		contact.PhotoMimeType = prop.Params["TYPE"]
		if contact.PhotoMimeType == "" {
			contact.PhotoMimeType = http.DetectContentType(data)
		}
	}
}

func parseSocialOrURL(prop vcardProp, groupLabel string) SocialLink {
	urlVal := strings.TrimSpace(prop.Value)
	if urlVal == "" {
		return SocialLink{}
	}

	label := groupLabel
	if label == "" {
		label = prop.Params["X-ABLABEL"]
	}

	typeVal := strings.ToLower(prop.Params["TYPE"])
	platform := detectPlatform(urlVal, typeVal, label)

	if label == "" {
		label = cases.Title(language.English).String(platform)
	}

	return SocialLink{
		Platform: platform,
		Label:    label,
		URL:      urlVal,
	}
}

func parseXSocialProfile(prop vcardProp, groupLabel string) SocialLink {
	urlVal := strings.TrimSpace(prop.Value)
	if urlVal == "" {
		return SocialLink{}
	}

	typeVal := strings.ToLower(prop.Params["TYPE"])
	platform := detectPlatform(urlVal, typeVal, groupLabel)

	label := groupLabel
	if label == "" {
		label = cases.Title(language.English).String(platform)
	}

	return SocialLink{
		Platform: platform,
		Label:    label,
		URL:      urlVal,
	}
}

func detectPlatform(url, typeVal, label string) string {
	combined := strings.ToLower(url + " " + typeVal + " " + label)

	switch {
	case strings.Contains(combined, "linkedin"):
		return "linkedin"
	case strings.Contains(combined, "twitter") || strings.Contains(combined, "x.com"):
		return "twitter"
	case strings.Contains(combined, "instagram") || strings.Contains(combined, "instagr.am"):
		return "instagram"
	case strings.Contains(combined, "github"):
		return "github"
	case strings.Contains(combined, "facebook") || strings.Contains(combined, "fb.me"):
		return "facebook"
	case strings.Contains(combined, "youtube") || strings.Contains(combined, "youtu.be"):
		return "youtube"
	case strings.Contains(combined, "whatsapp") || strings.Contains(combined, "wa.me"):
		return "whatsapp"
	case strings.Contains(combined, "telegram") || strings.Contains(combined, "t.me"):
		return "telegram"
	case strings.Contains(combined, "tiktok"):
		return "tiktok"
	case strings.Contains(combined, "medium"):
		return "medium"
	case strings.Contains(combined, "dribbble"):
		return "dribbble"
	case strings.Contains(combined, "behance"):
		return "behance"
	default:
		if typeVal != "" && typeVal != "pref" && typeVal != "work" && typeVal != "home" {
			return strings.ToLower(typeVal)
		}
		return "general"
	}
}

func isSocialPlatformName(name string) bool {
	platforms := []string{
		"x-linkedin", "x-twitter", "x-instagram", "x-github",
		"x-facebook", "x-youtube", "x-whatsapp", "x-telegram",
		"x-tiktok", "x-medium",
	}
	for _, p := range platforms {
		if strings.EqualFold(name, p) {
			return true
		}
	}
	return false
}

func cleanBase64(s string) string {
	reg := regexp.MustCompile(`[\r\n\s]+`)
	return reg.ReplaceAllString(s, "")
}

func cleanUID(val string) string {
	val = strings.TrimPrefix(val, "urn:uuid:")
	val = strings.TrimPrefix(val, "uuid:")
	return strings.TrimSpace(val)
}

func unescapeValue(val string) string {
	val = strings.ReplaceAll(val, "\\,", ",")
	val = strings.ReplaceAll(val, "\\;", ";")
	val = strings.ReplaceAll(val, "\\n", "\n")
	val = strings.ReplaceAll(val, "\\N", "\n")
	val = strings.ReplaceAll(val, "\\\\", "\\")
	return strings.TrimSpace(val)
}
