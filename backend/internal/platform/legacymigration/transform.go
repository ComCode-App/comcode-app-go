package legacymigration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	profiles "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/charmap"
)

// LegacyUser is populated only from the explicit non-secret source allow-list.
type LegacyUser map[string]string

type MediaRef struct{ Field, Path string }
type LegacyBank struct {
	Bank   string `json:"bank"`
	IBAN   string `json:"iban"`
	Holder string `json:"account_name"`
}
type Transformed struct {
	Row                       Row
	ID, Username, ContentHash string
	Profile                   profiles.UpsertProfileInput
	Published                 bool
	Theme, Layout             string
	Background                json.RawMessage
	Media                     []MediaRef
	Banks                     []LegacyBank
	// SectorIcons are company image names in field order; SectorIcon is the ikon2 one.
	SectorIcons []string
	SectorIcon  string
	Metadata    map[string]any
	Warnings    []string
}

var legacyColor = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)

var userColumns = strings.Fields("user_id username uniqid_id email name description avatar firmalogo arma1 ikon2 yer3 logo4 user_card_img user_card_img_back background_type background_value template main_link main_link_type buttons occupations location companies knowledge phonenumber gender dateofbirth vcard vcard_en vcard_de vcard_ru vcard_ar vcard_fl vcard_fr vcard_it vcard_es vcard_zh vcard_el vcard_ja vcard_pt bank_accounts active deactivated pro pro_due_date date last_activity")

// RepairText only reverses a reversible mojibake conversion when the candidate is valid
// UTF-8 and has fewer suspicious sequences. HTML entities are decoded afterwards.
func RepairText(s string) string { return html.UnescapeString(repairEncoding(s)) }

func repairEncoding(s string) string {
	score := func(v string) int {
		return strings.Count(v, "Ã") + strings.Count(v, "Ä") + strings.Count(v, "Å") + strings.Count(v, "â€") + strings.Count(v, "Â")
	}
	for range 2 {
		improved := false
		for _, enc := range []*charmap.Charmap{charmap.Windows1252, charmap.ISO8859_1} {
			b, err := enc.NewEncoder().Bytes([]byte(s))
			if err == nil && utf8.Valid(b) && score(string(b)) < score(s) {
				s = string(b)
				improved = true
				break
			}
		}
		if !improved {
			break
		}
	}
	return s
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func short(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
func jsonObject(s string) map[string]string {
	m := map[string]string{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]string{}
	}
	for key, value := range m {
		m[key] = RepairText(value)
	}
	return m
}
func imageName(s string) bool {
	switch strings.ToLower(path.Ext(s)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg":
		return true
	}
	return false
}

var buttonPlatforms = map[string][2]string{
	"instagram": {"instagram", "https://instagram.com/"}, "linkedin": {"linkedin", "https://linkedin.com/in/"},
	"facebook": {"facebook", "https://facebook.com/"}, "twitter": {"twitter", "https://twitter.com/"}, "x": {"twitter", "https://x.com/"},
	"youtube": {"youtube", "https://youtube.com/@"}, "tiktok": {"tiktok", "https://tiktok.com/@"},
	"telegram": {"telegram", "https://t.me/"}, "whatsapp": {"whatsapp", "https://wa.me/"},
	"pinterest": {"pinterest", "https://pinterest.com/"}, "github": {"github", "https://github.com/"},
	"webpage": {"custom", "https://"}, "website": {"custom", "https://"},
	"location":   {"custom", "https://www.google.com/maps/search/?api=1&query="},
	"sahibinden": {"custom", "https://"}, "remax": {"custom", "https://"},
}

func buttonLink(key, value string) profiles.CreateSocialLinkInput {
	platform, base := "custom", ""
	if m, ok := buttonPlatforms[strings.ToLower(key)]; ok {
		platform, base = m[0], m[1]
	}
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "://") && base != "" {
		value = strings.TrimPrefix(value, "@")
		if strings.EqualFold(key, "Location") {
			value = url.QueryEscape(value)
		}
		if platform == "whatsapp" {
			value = strings.NewReplacer("+", "", " ", "", "-", "", "(", "", ")", "").Replace(value)
		}
		value = base + value
	}
	u, err := url.Parse(value)
	safe := err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil
	if !safe {
		return profiles.CreateSocialLinkInput{Platform: "custom", Label: short(key, 100), URL: value, IsActive: false}
	}
	return profiles.CreateSocialLinkInput{Platform: platform, Label: short(key, 100), URL: value, IsActive: true}
}

// Transform never copies arbitrary source columns; login credentials cannot enter provenance.
func Transform(source LegacyUser, mainTypes map[string]string) (Transformed, error) {
	u := LegacyUser{}
	for _, k := range userColumns {
		u[k] = RepairText(source[k])
		if k == "vcard" || k == "buttons" || k == "bank_accounts" || strings.HasPrefix(k, "vcard_") {
			u[k] = repairEncoding(source[k])
		}
	}
	t := Transformed{ID: u["user_id"], Username: strings.ToLower(u["username"]), Published: u["deactivated"] != "1" && u["active"] != "0", Metadata: map[string]any{}, Media: []MediaRef{}, Banks: []LegacyBank{}, Warnings: []string{}}
	if t.ID == "" || !hashPattern.MatchString(u["uniqid_id"]) {
		return t, fmt.Errorf("invalid legacy user identity")
	}
	for _, k := range []string{"vcard", "buttons"} {
		if u[k] != "" && u[k] != "null" && !json.Valid([]byte(u[k])) {
			t.Metadata[k+"_unparsed"] = u[k]
			t.Warnings = append(t.Warnings, "JSON_INVALID:"+k)
		}
	}
	v := jsonObject(u["vcard"])
	tr := profiles.UpsertTranslationInput{Locale: "tr", FirstName: short(first(v["firstname"], u["name"], u["username"]), 100), LastName: short(v["lastname"], 100), CompanyName: short(first(v["organization"], u["companies"]), 255), Title: short(first(v["title"], u["occupations"]), 255), Bio: u["description"], Note: u["knowledge"], IsDefault: true}
	t.Profile = profiles.UpsertProfileInput{PrimaryLocale: "tr", Gender: short(u["gender"], 32), Translations: []profiles.UpsertTranslationInput{tr}, ContactDetails: []profiles.CreateContactDetailInput{}, SocialLinks: []profiles.CreateSocialLinkInput{}, Addresses: []profiles.CreateAddressInput{}}
	if birthday, err := time.Parse("2006-01-02", u["dateofbirth"]); err == nil {
		value := birthday.Format("2006-01-02")
		t.Profile.Birthday = &value
	} else if u["dateofbirth"] != "" {
		t.Metadata["dateofbirth"] = u["dateofbirth"]
		t.Warnings = append(t.Warnings, "BIRTHDAY_METADATA")
	}
	for _, c := range [][3]string{{"phone", "work", v["workphone"]}, {"phone", "mobile", first(v["cellphone"], u["phonenumber"])}, {"email", "work", v["workemail"]}, {"email", "home", v["personalemail"]}, {"url", "work", v["workurl"]}} {
		if c[2] != "" {
			t.Profile.ContactDetails = append(t.Profile.ContactDetails, profiles.CreateContactDetailInput{Category: c[0], Type: c[1], Value: c[2], IsPrimary: len(t.Profile.ContactDetails) == 0})
		}
	}
	street := first(v["workstreet"], u["location"])
	if street != "" {
		t.Profile.Addresses = append(t.Profile.Addresses, profiles.CreateAddressInput{Type: "work", Street: street, City: short(v["workcity"], 100), Country: short(v["workcountry"], 100), IsPrimary: true})
	}
	buttons := jsonObject(u["buttons"])
	keys := make([]string, 0, len(buttons))
	for k := range buttons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.TrimSpace(buttons[k]) != "" {
			l := buttonLink(k, buttons[k])
			t.Profile.SocialLinks = append(t.Profile.SocialLinks, l)
			if !l.IsActive {
				t.Warnings = append(t.Warnings, "LINK_REVIEW:"+k)
			}
		}
	}
	if u["main_link"] != "" {
		l := buttonLink("primary", u["main_link"])
		l.Label = short(first(RepairText(mainTypes[u["main_link_type"]]), "Primary action"), 100)
		t.Profile.SocialLinks = append([]profiles.CreateSocialLinkInput{l}, t.Profile.SocialLinks...)
	}
	for _, field := range sectorIconFields {
		if value := strings.TrimSpace(u[field]); imageName(value) {
			t.SectorIcons = append(t.SectorIcons, path.Base(value))
			if field == "ikon2" {
				t.SectorIcon = path.Base(value)
			}
		}
	}
	for _, field := range []string{"avatar", "firmalogo", "arma1", "ikon2", "yer3", "logo4", "user_card_img", "user_card_img_back"} {
		value := strings.TrimPrefix(u[field], "/")
		if value == "" {
			continue
		}
		if !imageName(value) {
			t.Metadata[field] = value
			continue
		}
		switch field {
		case "avatar":
			// The legacy app stores the placeholder name for users without a photo.
			if strings.EqualFold(path.Base(value), "default.jpg") {
				continue
			}
			value = "uploads/avatars/" + path.Base(value)
		case "user_card_img", "user_card_img_back":
			if !strings.HasPrefix(value, "usercard/") {
				continue
			}
		default:
			value = "uploads/comps/" + path.Base(value)
		}
		if validMediaPath(value) {
			t.Media = append(t.Media, MediaRef{Field: field, Path: value})
			if field != "avatar" && field != "user_card_img" {
				t.Warnings = append(t.Warnings, "MEDIA_METADATA:"+field)
			}
		}
	}
	if u["bank_accounts"] != "" && u["bank_accounts"] != "null" {
		if err := json.Unmarshal([]byte(u["bank_accounts"]), &t.Banks); err != nil {
			t.Warnings = append(t.Warnings, "BANK_JSON_INVALID")
			t.Metadata["bank_accounts_unparsed"] = u["bank_accounts"]
		}
	}
	for _, k := range []string{"active", "deactivated", "pro", "pro_due_date", "date", "last_activity", "main_link_type"} {
		t.Metadata[k] = u[k]
	}
	for i := range t.Banks {
		t.Banks[i].Bank = RepairText(t.Banks[i].Bank)
		t.Banks[i].Holder = RepairText(t.Banks[i].Holder)
	}
	t.Metadata["bank_count"] = len(t.Banks)
	t.Metadata["owner_email"] = u["email"]
	t.Theme = "classic_light"
	t.Layout = "standard"
	if u["template"] == "two" {
		t.Theme = "compact_card"
		t.Layout = "compact"
	}
	if u["background_type"] == "color" && legacyColor.MatchString(u["background_value"]) {
		t.Background, _ = json.Marshal(map[string]any{"version": "1.0", "kind": "solid", "params": map[string]string{"color": u["background_value"]}})
	}
	t.Metadata["template"] = t.Theme
	t.Metadata["legacy_template"] = u["template"]
	t.Metadata["background_type"] = u["background_type"]
	t.Metadata["background_value"] = u["background_value"]
	t.Warnings = append(t.Warnings, "THEME_APPROXIMATION: legacy background retained privately")
	for _, k := range userColumns {
		if strings.HasPrefix(k, "vcard_") && u[k] != "" && u[k] != "null" {
			t.Metadata[k] = jsonObject(u[k])
			t.Warnings = append(t.Warnings, "TRANSLATION_METADATA:"+k)
		}
	}
	b, err := json.Marshal(struct {
		User     LegacyUser
		MainType string
	}{u, mainTypes[u["main_link_type"]]})
	if err != nil {
		return t, err
	}
	sum := sha256.Sum256(b)
	t.ContentHash = hex.EncodeToString(sum[:])
	profileUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("comcode:legacy:user:"+t.ID))
	t.Row = Row{LegacyHash: u["uniqid_id"], TargetType: "profile", TargetValue: profileUUID.String(), OwnerEmail: u["email"], OwnerName: tr.FirstName, ExtraJSON: json.RawMessage(`{}`)}
	return t, nil
}
