package vcard

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Version represents the vCard specification version.
type Version string

const (
	Version30 Version = "3.0"
	Version40 Version = "4.0"
)

// Address represents a physical postal address.
type Address struct {
	Type       string // work, home, other
	Label      string
	Street     string
	Extended   string // unit, suite, apt
	City       string
	Region     string // state, province
	PostalCode string
	Country    string
	IsPrimary  bool
}

// Phone represents a phone contact.
type Phone struct {
	Type      string // work, home, cell/mobile, fax, preferred
	Label     string
	Number    string
	IsPrimary bool
}

// Email represents an email contact.
type Email struct {
	Type      string // work, home, preferred
	Label     string
	Address   string
	IsPrimary bool
}

// SocialLink represents a web URL or social profile.
type SocialLink struct {
	Platform string
	Label    string
	URL      string
}

// Contact represents complete contact information for generating a vCard.
type Contact struct {
	UID           string
	FirstName     string
	LastName      string
	MiddleName    string
	Prefix        string
	Suffix        string
	Nickname      string
	DisplayName   string
	CompanyName   string
	Department    string
	Title         string
	Role          string
	Bio           string
	Note          string
	Birthday      *time.Time
	Gender        string
	AvatarURL     string
	PhotoBytes    []byte
	PhotoMimeType string
	Phones        []Phone
	Emails        []Email
	Addresses     []Address
	SocialLinks   []SocialLink
	CustomFields  map[string]string
	Version       Version
}

// Generator generates RFC-compliant vCard files with Apple Address Book (X-ABLabel) parity.
type Generator struct{}

// NewGenerator returns a new Generator instance.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate formats the contact into a valid vCard string (defaults to RFC 6350 / vCard 4.0).
func (g *Generator) Generate(contact Contact) string {
	version := contact.Version
	if version == "" {
		version = Version40
	}

	var sb strings.Builder
	sb.WriteString("BEGIN:VCARD\r\n")
	fmt.Fprintf(&sb, "VERSION:%s\r\n", version)
	sb.WriteString("PRODID:-//ComCode//Identity & vCard Engine//EN\r\n")

	// UID
	if contact.UID != "" {
		fmt.Fprintf(&sb, "UID:urn:uuid:%s\r\n", contact.UID)
	}

	// Formatted Name (FN) - Mandatory in both 3.0 and 4.0
	fn := contact.DisplayName
	if fn == "" {
		fn = strings.TrimSpace(fmt.Sprintf("%s %s %s %s %s",
			contact.Prefix, contact.FirstName, contact.MiddleName, contact.LastName, contact.Suffix))
		fn = strings.Join(strings.Fields(fn), " ")
	}
	if fn == "" {
		fn = "Unknown Contact"
	}
	fmt.Fprintf(&sb, "FN:%s\r\n", escapeValue(fn))

	// Structured Name (N): Family, Given, Additional, Prefix, Suffix
	fmt.Fprintf(&sb, "N:%s;%s;%s;%s;%s\r\n",
		escapeValue(contact.LastName),
		escapeValue(contact.FirstName),
		escapeValue(contact.MiddleName),
		escapeValue(contact.Prefix),
		escapeValue(contact.Suffix),
	)

	// Nickname
	if contact.Nickname != "" {
		fmt.Fprintf(&sb, "NICKNAME:%s\r\n", escapeValue(contact.Nickname))
	}

	// Organization & Department
	if contact.CompanyName != "" || contact.Department != "" {
		if contact.Department != "" {
			fmt.Fprintf(&sb, "ORG:%s;%s\r\n", escapeValue(contact.CompanyName), escapeValue(contact.Department))
		} else {
			fmt.Fprintf(&sb, "ORG:%s\r\n", escapeValue(contact.CompanyName))
		}
	}

	// Title & Role
	if contact.Title != "" {
		fmt.Fprintf(&sb, "TITLE:%s\r\n", escapeValue(contact.Title))
	}
	if contact.Role != "" {
		fmt.Fprintf(&sb, "ROLE:%s\r\n", escapeValue(contact.Role))
	}

	itemIndex := 1

	// Phones
	for _, phone := range contact.Phones {
		if phone.Number == "" {
			continue
		}
		label := strings.TrimSpace(phone.Label)
		if label != "" && !isStandardType(label) {
			prefix := fmt.Sprintf("item%d", itemIndex)
			itemIndex++
			typeParam := ""
			if phone.IsPrimary {
				typeParam = ";type=pref"
			}
			fmt.Fprintf(&sb, "%s.TEL%s:%s\r\n", prefix, typeParam, escapeValue(phone.Number))
			fmt.Fprintf(&sb, "%s.X-ABLabel:%s\r\n", prefix, escapeValue(label))
		} else {
			phoneType := normalizePhoneType(phone.Type)
			if phone.IsPrimary {
				phoneType += ",PREF"
			}
			fmt.Fprintf(&sb, "TEL;TYPE=%s:%s\r\n", phoneType, escapeValue(phone.Number))
		}
	}

	// Emails
	for _, email := range contact.Emails {
		if email.Address == "" {
			continue
		}
		label := strings.TrimSpace(email.Label)
		if label != "" && !isStandardType(label) {
			prefix := fmt.Sprintf("item%d", itemIndex)
			itemIndex++
			typeParam := ""
			if email.IsPrimary {
				typeParam = ";type=pref"
			}
			fmt.Fprintf(&sb, "%s.EMAIL%s:%s\r\n", prefix, typeParam, escapeValue(email.Address))
			fmt.Fprintf(&sb, "%s.X-ABLabel:%s\r\n", prefix, escapeValue(label))
		} else {
			emailType := normalizeContactType(email.Type, "INTERNET")
			if email.IsPrimary {
				emailType += ",PREF"
			}
			fmt.Fprintf(&sb, "EMAIL;TYPE=%s:%s\r\n", emailType, escapeValue(email.Address))
		}
	}

	// Addresses
	for _, addr := range contact.Addresses {
		addrType := normalizeContactType(addr.Type, "WORK")
		if addr.IsPrimary {
			addrType += ",PREF"
		}
		adrValue := fmt.Sprintf(";;%s;%s;%s;%s;%s",
			escapeValue(joinStreet(addr.Street, addr.Extended)),
			escapeValue(addr.City),
			escapeValue(addr.Region),
			escapeValue(addr.PostalCode),
			escapeValue(addr.Country),
		)

		label := strings.TrimSpace(addr.Label)
		if label != "" && !isStandardType(label) {
			prefix := fmt.Sprintf("item%d", itemIndex)
			itemIndex++
			fmt.Fprintf(&sb, "%s.ADR;TYPE=%s:%s\r\n", prefix, addrType, adrValue)
			fmt.Fprintf(&sb, "%s.X-ABLabel:%s\r\n", prefix, escapeValue(label))
		} else {
			fmt.Fprintf(&sb, "ADR;TYPE=%s:%s\r\n", addrType, adrValue)
		}
	}

	// Avatar Photo (Base64 or Direct URI)
	if len(contact.PhotoBytes) > 0 {
		b64 := base64.StdEncoding.EncodeToString(contact.PhotoBytes)
		mimeType := strings.ToLower(strings.TrimSpace(contact.PhotoMimeType))
		if mimeType == "" {
			mimeType = http.DetectContentType(contact.PhotoBytes)
		}
		if mimeType == "application/octet-stream" || !strings.HasPrefix(mimeType, "image/") {
			mimeType = "image/jpeg"
		}

		if version == Version40 {
			// vCard 4.0: PHOTO:data:image/jpeg;base64,...
			fmt.Fprintf(&sb, "PHOTO:data:%s;base64,%s\r\n", mimeType, b64)
		} else {
			// vCard 3.0: PHOTO;TYPE=JPEG;ENCODING=b:\r\n <folded base64>\r\n
			photoType := "JPEG"
			if strings.Contains(mimeType, "png") {
				photoType = "PNG"
			} else if strings.Contains(mimeType, "gif") {
				photoType = "GIF"
			} else if strings.Contains(mimeType, "webp") {
				photoType = "WEBP"
			}

			fmt.Fprintf(&sb, "PHOTO;TYPE=%s;ENCODING=b:\r\n", photoType)
			for len(b64) > 0 {
				chunkSize := 72
				if len(b64) < chunkSize {
					chunkSize = len(b64)
				}
				sb.WriteByte(' ')
				sb.WriteString(b64[:chunkSize])
				sb.WriteString("\r\n")
				b64 = b64[chunkSize:]
			}
		}
	} else if contact.AvatarURL != "" {
		if version == Version40 {
			fmt.Fprintf(&sb, "PHOTO;MEDIATYPE=image/jpeg:%s\r\n", contact.AvatarURL)
		} else {
			fmt.Fprintf(&sb, "PHOTO;VALUE=URI:%s\r\n", contact.AvatarURL)
		}
	}

	// Birthday
	if contact.Birthday != nil && !contact.Birthday.IsZero() {
		fmt.Fprintf(&sb, "BDAY:%s\r\n", contact.Birthday.Format("2006-01-02"))
	}

	// Gender (vCard 4.0)
	if contact.Gender != "" && version == Version40 {
		fmt.Fprintf(&sb, "GENDER:%s\r\n", mapGender(contact.Gender))
	}

	// Note / Bio
	combinedNote := strings.TrimSpace(contact.Bio)
	if contact.Note != "" {
		if combinedNote != "" {
			combinedNote += "\n\n" + strings.TrimSpace(contact.Note)
		} else {
			combinedNote = strings.TrimSpace(contact.Note)
		}
	}
	if combinedNote != "" {
		fmt.Fprintf(&sb, "NOTE:%s\r\n", escapeValue(combinedNote))
	}

	// Social links & URLs
	for _, link := range contact.SocialLinks {
		if link.URL == "" {
			continue
		}
		label := strings.TrimSpace(link.Label)
		if label != "" {
			prefix := fmt.Sprintf("item%d", itemIndex)
			itemIndex++
			fmt.Fprintf(&sb, "%s.URL:%s\r\n", prefix, link.URL)
			fmt.Fprintf(&sb, "%s.X-ABLabel:%s\r\n", prefix, escapeValue(label))
		} else {
			fmt.Fprintf(&sb, "URL;TYPE=%s:%s\r\n", strings.ToUpper(link.Platform), link.URL)
		}
		// Also write X-SOCIALPROFILE for broader mobile compatibility
		fmt.Fprintf(&sb, "X-SOCIALPROFILE;type=%s:%s\r\n", strings.ToLower(link.Platform), link.URL)
	}

	// Custom fields
	for k, v := range contact.CustomFields {
		if k != "" && v != "" {
			fmt.Fprintf(&sb, "%s:%s\r\n", strings.ToUpper(k), escapeValue(v))
		}
	}

	fmt.Fprintf(&sb, "REV:%s\r\n", time.Now().UTC().Format("20060102T150405Z"))
	sb.WriteString("END:VCARD\r\n")

	return sb.String()
}

func isStandardType(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "work", "home", "cell", "mobile", "main", "other", "fax", "preferred", "pref", "is", "ev", "cep":
		return true
	default:
		return false
	}
}

func escapeValue(val string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\n", "\\n",
		"\r", "",
	)
	return r.Replace(val)
}

func joinStreet(street, extended string) string {
	if extended == "" {
		return street
	}
	if street == "" {
		return extended
	}
	return street + ", " + extended
}

func normalizeContactType(t, defaultType string) string {
	t = strings.TrimSpace(strings.ToUpper(t))
	if t == "" {
		return defaultType
	}
	return t
}

func normalizePhoneType(t string) string {
	t = strings.TrimSpace(strings.ToUpper(t))
	switch t {
	case "MOBILE", "CELL":
		return "CELL,VOICE"
	case "WORK":
		return "WORK,VOICE"
	case "HOME":
		return "HOME,VOICE"
	case "FAX":
		return "FAX"
	default:
		if t == "" {
			return "VOICE"
		}
		return t + ",VOICE"
	}
}

func mapGender(g string) string {
	g = strings.TrimSpace(strings.ToLower(g))
	switch g {
	case "male", "m", "erkek":
		return "M"
	case "female", "f", "kadın", "kadin":
		return "F"
	case "other", "diğer", "diger":
		return "O"
	default:
		return "U"
	}
}
