package vcard

import (
	"strings"
	"testing"
	"time"
)

func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator()
	bday := time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC)

	contact := Contact{
		UID:         "123e4567-e89b-12d3-a456-426614174000",
		FirstName:   "John",
		LastName:    "Doe",
		MiddleName:  "William",
		Prefix:      "Dr.",
		Suffix:      "PhD",
		Nickname:    "Johnny",
		DisplayName: "Dr. John William Doe PhD",
		CompanyName: "Acme Corp",
		Department:  "Engineering",
		Title:       "Principal Architect",
		Role:        "Lead Developer",
		Bio:         "Passionate software engineer building resilient cloud platforms.",
		Note:        "Met at GopherCon 2026",
		Birthday:    &bday,
		Gender:      "male",
		AvatarURL:   "https://s3.amazonaws.com/bucket/avatars/john-doe.jpg",
		Phones: []Phone{
			{Type: "mobile", Number: "+905551234567", IsPrimary: true, Label: "Olex Films"},
			{Type: "work", Number: "+902129876543"},
		},
		Emails: []Email{
			{Type: "work", Address: "john.doe@acme.com", IsPrimary: true, Label: "Work Email"},
			{Type: "home", Address: "john@personal.com"},
		},
		Addresses: []Address{
			{
				Type:       "work",
				Street:     "Buyukdere Cad. No: 100",
				Extended:   "Floor 5, Suite 502",
				City:       "Istanbul",
				Region:     "Sariyer",
				PostalCode: "34394",
				Country:    "Turkey",
				IsPrimary:  true,
			},
		},
		SocialLinks: []SocialLink{
			{Platform: "linkedin", URL: "https://linkedin.com/in/johndoe", Label: "LinkedIn Profile"},
			{Platform: "github", URL: "https://github.com/johndoe"},
		},
		Version: Version30,
	}

	result := g.Generate(contact)

	if !strings.HasPrefix(result, "BEGIN:VCARD\r\nVERSION:3.0\r\n") {
		t.Fatalf("unexpected vcard header: %s", result)
	}
	if !strings.HasSuffix(result, "END:VCARD\r\n") {
		t.Fatalf("unexpected vcard footer: %s", result)
	}

	expectedSubstrings := []string{
		"UID:urn:uuid:123e4567-e89b-12d3-a456-426614174000",
		"FN:Dr. John William Doe PhD",
		"N:Doe;John;William;Dr.;PhD",
		"NICKNAME:Johnny",
		"ORG:Acme Corp;Engineering",
		"TITLE:Principal Architect",
		"ROLE:Lead Developer",
		"item1.TEL;type=pref:+905551234567",
		"item1.X-ABLabel:Olex Films",
		"TEL;TYPE=WORK,VOICE:+902129876543",
		"item2.EMAIL;type=pref:john.doe@acme.com",
		"item2.X-ABLabel:Work Email",
		"EMAIL;TYPE=HOME:john@personal.com",
		"ADR;TYPE=WORK,PREF:;;Buyukdere Cad. No: 100\\, Floor 5\\, Suite 502;Istanbul;Sariyer;34394;Turkey",
		"PHOTO;VALUE=URI:https://s3.amazonaws.com/bucket/avatars/john-doe.jpg",
		"BDAY:1990-05-15",
		"NOTE:Passionate software engineer building resilient cloud platforms.\\n\\nMet at GopherCon 2026",
		"item3.URL:https://linkedin.com/in/johndoe",
		"item3.X-ABLabel:LinkedIn Profile",
		"X-SOCIALPROFILE;type=linkedin:https://linkedin.com/in/johndoe",
		"URL;TYPE=GITHUB:https://github.com/johndoe",
		"X-SOCIALPROFILE;type=github:https://github.com/johndoe",
	}

	for _, exp := range expectedSubstrings {
		if !strings.Contains(result, exp) {
			t.Errorf("expected vcard to contain %q, but was missing in:\n%s", exp, result)
		}
	}
}

func TestGenerator_Generate_Base64Photo(t *testing.T) {
	g := NewGenerator()
	dummyBytes := []byte("fake-image-binary-payload-data-for-testing")

	// Test 3.0
	contact30 := Contact{
		UID:           "123e4567-e89b-12d3-a456-426614174000",
		FirstName:     "Jane",
		LastName:      "Doe",
		PhotoBytes:    dummyBytes,
		PhotoMimeType: "image/png",
		Version:       Version30,
	}
	result30 := g.Generate(contact30)
	if !strings.Contains(result30, "PHOTO;TYPE=PNG;ENCODING=b:\r\n") {
		t.Errorf("expected base64 photo for vCard 3.0, got:\n%s", result30)
	}

	// Test 4.0
	contact40 := Contact{
		UID:           "123e4567-e89b-12d3-a456-426614174000",
		FirstName:     "Jane",
		LastName:      "Doe",
		PhotoBytes:    dummyBytes,
		PhotoMimeType: "image/jpeg",
		Version:       Version40,
	}
	result40 := g.Generate(contact40)
	if !strings.Contains(result40, "PHOTO:data:image/jpeg;base64,") {
		t.Errorf("expected base64 photo data URI for vCard 4.0, got:\n%s", result40)
	}
}
