// Package completion is the single source of the profile completion score ("Profil Doluluğu")
// and of the "complete your profile" suggestions derived from it (TEC-549).
//
// Every rule awards points out of 100. The stored profiles.completion_rate and the suggestions
// returned with a profile come from the same rule table, so the score plus the points of every
// open suggestion is always 100.
package completion

import (
	"sort"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
)

// Editor sections a suggestion deep-links to (the profile form tabs).
const (
	SectionIdentity  = "identity"
	SectionMedia     = "media"
	SectionContacts  = "contacts"
	SectionSocial    = "social"
	SectionAddresses = "addresses"
)

// Rule keys. They are stable API values (the frontend maps them to copy).
const (
	KeyName    = "name"
	KeyTitle   = "title"
	KeyCompany = "company"
	KeyAvatar  = "avatar"
	KeyPhone   = "phone"
	KeyEmail   = "email"
	KeyAddress = "address"
	KeySocial  = "social"
)

// Fields is the neutral view of a profile the rules score. Build it with FromInput or FromDetail.
type Fields struct {
	// Names holds (first, last) per translation.
	Names     [][2]string
	Titles    []string
	Companies []string
	AvatarURL string
	// ContactCategories holds the category of every contact with a non-empty value.
	ContactCategories []string
	// FirstAddressCity is the city of the first address ("" when there is none).
	FirstAddressCity string
	SocialLinks      int
}

type rule struct {
	key     string
	section string
	points  int
	// earned returns the points this profile already has for the rule (0..points).
	earned func(f Fields) int
}

// rules is the score table. Order is the tie-break order of suggestions.
var rules = []rule{
	{key: KeyName, section: SectionIdentity, points: 25, earned: func(f Fields) int {
		// Full name 25, only first or last name 15 (first translation with a name decides).
		for _, n := range f.Names {
			first, last := strings.TrimSpace(n[0]) != "", strings.TrimSpace(n[1]) != ""
			switch {
			case first && last:
				return 25
			case first || last:
				return 15
			}
		}
		return 0
	}},
	{key: KeyAvatar, section: SectionMedia, points: 15, earned: func(f Fields) int {
		return award(strings.TrimSpace(f.AvatarURL) != "", 15)
	}},
	{key: KeyTitle, section: SectionIdentity, points: 10, earned: func(f Fields) int {
		return award(anyNonEmpty(f.Titles), 10)
	}},
	{key: KeyCompany, section: SectionIdentity, points: 10, earned: func(f Fields) int {
		return award(anyNonEmpty(f.Companies), 10)
	}},
	{key: KeyPhone, section: SectionContacts, points: 10, earned: func(f Fields) int {
		return award(hasCategory(f.ContactCategories, "phone", "whatsapp"), 10)
	}},
	{key: KeyEmail, section: SectionContacts, points: 10, earned: func(f Fields) int {
		return award(hasCategory(f.ContactCategories, "email"), 10)
	}},
	{key: KeyAddress, section: SectionAddresses, points: 10, earned: func(f Fields) int {
		return award(strings.TrimSpace(f.FirstAddressCity) != "", 10)
	}},
	{key: KeySocial, section: SectionSocial, points: 10, earned: func(f Fields) int {
		return award(f.SocialLinks > 0, 10)
	}},
}

// Suggestion is one actionable step that raises the score (model.CompletionSuggestion).
type Suggestion = model.CompletionSuggestion

// Result is a profile's score and its open suggestions, highest gain first.
type Result struct {
	Rate        int          `json:"rate"`
	Suggestions []Suggestion `json:"suggestions"`
}

// Evaluate scores f and lists what is missing.
func Evaluate(f Fields) Result {
	res := Result{Suggestions: []Suggestion{}}
	for _, r := range rules {
		got := r.earned(f)
		res.Rate += got
		if missing := r.points - got; missing > 0 {
			res.Suggestions = append(res.Suggestions, Suggestion{
				Key: r.key, Points: missing, Section: r.section,
				I18nKey: "profiles.completion.suggestions." + r.key,
			})
		}
	}
	if res.Rate > 100 {
		res.Rate = 100
	}
	sort.SliceStable(res.Suggestions, func(i, j int) bool {
		return res.Suggestions[i].Points > res.Suggestions[j].Points
	})
	return res
}

// Rate is Evaluate(f).Rate.
func Rate(f Fields) int { return Evaluate(f).Rate }

// FromInput builds Fields from a create/update payload.
func FromInput(in model.UpsertProfileInput) Fields {
	var f Fields
	for _, tr := range in.Translations {
		f.Names = append(f.Names, [2]string{tr.FirstName, tr.LastName})
		f.Titles = append(f.Titles, tr.Title)
		f.Companies = append(f.Companies, tr.CompanyName)
	}
	f.AvatarURL = in.AvatarURL
	for _, c := range in.ContactDetails {
		if strings.TrimSpace(c.Value) != "" {
			f.ContactCategories = append(f.ContactCategories, c.Category)
		}
	}
	if len(in.Addresses) > 0 {
		f.FirstAddressCity = in.Addresses[0].City
	}
	f.SocialLinks = len(in.SocialLinks)
	return f
}

// FromDetail builds Fields from a stored profile.
func FromDetail(d *model.ProfileDetail) Fields {
	var f Fields
	if d == nil {
		return f
	}
	for _, tr := range d.Translations {
		f.Names = append(f.Names, [2]string{tr.FirstName, tr.LastName})
		f.Titles = append(f.Titles, tr.Title)
		f.Companies = append(f.Companies, tr.CompanyName)
	}
	f.AvatarURL = d.AvatarURL
	for _, c := range d.ContactDetails {
		if strings.TrimSpace(c.Value) != "" {
			f.ContactCategories = append(f.ContactCategories, c.Category)
		}
	}
	if len(d.Addresses) > 0 {
		f.FirstAddressCity = d.Addresses[0].City
	}
	f.SocialLinks = len(d.SocialLinks)
	return f
}

func award(ok bool, points int) int {
	if ok {
		return points
	}
	return 0
}

func anyNonEmpty(values []string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func hasCategory(categories []string, want ...string) bool {
	for _, c := range categories {
		for _, w := range want {
			if c == w {
				return true
			}
		}
	}
	return false
}
