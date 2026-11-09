package vcard

import (
	"testing"
	"time"
)

func TestParser_ParseSingle_VCard30(t *testing.T) {
	vcf := `BEGIN:VCARD
VERSION:3.0
PRODID:-//ComCode//Identity & vCard Engine//EN
UID:urn:uuid:123e4567-e89b-12d3-a456-426614174000
FN:Furkan Meclis
N:Meclis;Furkan;;;
ORG:Technowide;Software Engineering
TITLE:Lead Software Engineer
ROLE:Architect
item1.TEL;type=pref:+905551234567
item1.X-ABLabel:Work Mobile
item2.EMAIL:+90555@technowide.com
item2.X-ABLabel:Work
ADR;TYPE=WORK:;;Büyükdere Cad. No:100;İstanbul;Marmara;34394;Turkey
NOTE:Full stack engineer specializing in Go and TypeScript.
BDAY:1995-05-15
URL;TYPE=LINKEDIN:https://linkedin.com/in/furkanmeclis
X-SOCIALPROFILE;type=github:https://github.com/furkanmeclis
END:VCARD`

	p := NewParser()
	contact, err := p.ParseSingle(vcf)
	if err != nil {
		t.Fatalf("unexpected error parsing vcard: %v", err)
	}

	if contact.DisplayName != "Furkan Meclis" {
		t.Errorf("expected DisplayName 'Furkan Meclis', got '%s'", contact.DisplayName)
	}
	if contact.FirstName != "Furkan" || contact.LastName != "Meclis" {
		t.Errorf("expected First/Last 'Furkan' 'Meclis', got '%s' '%s'", contact.FirstName, contact.LastName)
	}
	if contact.CompanyName != "Technowide" || contact.Department != "Software Engineering" {
		t.Errorf("expected Org 'Technowide' 'Software Engineering', got '%s' '%s'", contact.CompanyName, contact.Department)
	}
	if contact.Title != "Lead Software Engineer" {
		t.Errorf("expected Title 'Lead Software Engineer', got '%s'", contact.Title)
	}

	if len(contact.Phones) != 1 {
		t.Fatalf("expected 1 phone, got %d", len(contact.Phones))
	}
	if contact.Phones[0].Number != "+905551234567" || contact.Phones[0].Label != "Work Mobile" {
		t.Errorf("unexpected phone: %+v", contact.Phones[0])
	}

	if len(contact.Emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(contact.Emails))
	}
	if contact.Emails[0].Address != "+90555@technowide.com" {
		t.Errorf("unexpected email: %+v", contact.Emails[0])
	}

	if len(contact.Addresses) != 1 {
		t.Fatalf("expected 1 address, got %d", len(contact.Addresses))
	}
	if contact.Addresses[0].City != "İstanbul" || contact.Addresses[0].Country != "Turkey" {
		t.Errorf("unexpected address: %+v", contact.Addresses[0])
	}

	if contact.Birthday == nil || contact.Birthday.Format("2006-01-02") != "1995-05-15" {
		t.Errorf("unexpected birthday: %v", contact.Birthday)
	}

	if len(contact.SocialLinks) != 2 {
		t.Fatalf("expected 2 social links, got %d", len(contact.SocialLinks))
	}
}

func TestParser_RoundTrip_GeneratorAndParser(t *testing.T) {
	bday := time.Date(1990, 8, 20, 0, 0, 0, 0, time.UTC)
	original := Contact{
		FirstName:   "Ahmet",
		LastName:    "Yılmaz",
		DisplayName: "Ahmet Yılmaz",
		CompanyName: "Acme Corp",
		Title:       "Product Designer",
		Bio:         "Passionate about UI/UX.",
		Birthday:    &bday,
		Phones: []Phone{
			{Type: "mobile", Label: "Kişisel", Number: "+905321112233", IsPrimary: true},
		},
		Emails: []Email{
			{Type: "work", Label: "Şirket", Address: "ahmet@acme.com", IsPrimary: true},
		},
		Addresses: []Address{
			{Type: "work", Street: "İstiklal Cad. No:1", City: "İstanbul", Country: "Türkiye", IsPrimary: true},
		},
		SocialLinks: []SocialLink{
			{Platform: "linkedin", Label: "LinkedIn", URL: "https://linkedin.com/in/ahmetyilmaz"},
		},
	}

	gen := NewGenerator()
	vcfOutput := gen.Generate(original)

	parser := NewParser()
	parsed, err := parser.ParseSingle(vcfOutput)
	if err != nil {
		t.Fatalf("failed to parse generated vcf: %v", err)
	}

	if parsed.FirstName != original.FirstName || parsed.LastName != original.LastName {
		t.Errorf("name mismatch: got %s %s, want %s %s", parsed.FirstName, parsed.LastName, original.FirstName, original.LastName)
	}
	if parsed.CompanyName != original.CompanyName {
		t.Errorf("company mismatch: got %s, want %s", parsed.CompanyName, original.CompanyName)
	}
	if parsed.Title != original.Title {
		t.Errorf("title mismatch: got %s, want %s", parsed.Title, original.Title)
	}
	if len(parsed.Phones) == 0 || parsed.Phones[0].Number != original.Phones[0].Number {
		t.Errorf("phone mismatch: got %+v, want %+v", parsed.Phones, original.Phones)
	}
	if len(parsed.Emails) == 0 || parsed.Emails[0].Address != original.Emails[0].Address {
		t.Errorf("email mismatch: got %+v, want %+v", parsed.Emails, original.Emails)
	}
}
