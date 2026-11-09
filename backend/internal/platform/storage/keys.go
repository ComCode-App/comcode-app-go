package storage

import "fmt"

// AppLogoObjectKey builds app/branding/logo.{ext} for system letterhead.
func AppLogoObjectKey(ext string) string {
	return fmt.Sprintf("app/branding/logo.%s", trimExt(ext))
}

// ExportObjectKey builds exports/{jobUUID}/export.{ext}.
func ExportObjectKey(jobUUID, ext string) string {
	return fmt.Sprintf("exports/%s/export.%s", jobUUID, trimExt(ext))
}

// ImportSourceObjectKey builds imports/{jobUUID}/source.{ext}.
func ImportSourceObjectKey(jobUUID, ext string) string {
	return fmt.Sprintf("imports/%s/source.%s", jobUUID, trimExt(ext))
}

// SocialPlatformIconObjectKey builds assets/social-icons/{slug}.{ext}.
func SocialPlatformIconObjectKey(slug, ext string) string {
	return fmt.Sprintf("assets/social-icons/%s.%s", slug, trimExt(ext))
}

// BankIconObjectKey builds assets/bank-icons/{slug}.{ext}.
func BankIconObjectKey(slug, ext string) string {
	return fmt.Sprintf("assets/bank-icons/%s.%s", slug, trimExt(ext))
}

// ProfileAvatarObjectKey builds profiles/{profileUUID}/avatar.{ext}.
func ProfileAvatarObjectKey(profileUUID, ext string) string {
	return fmt.Sprintf("profiles/%s/avatar.%s", profileUUID, trimExt(ext))
}

// ProfileCoverObjectKey builds profiles/{profileUUID}/cover.{ext}.
func ProfileCoverObjectKey(profileUUID, ext string) string {
	return fmt.Sprintf("profiles/%s/cover.%s", profileUUID, trimExt(ext))
}

// ProfileCompanyMediaObjectKey builds profiles/{profileUUID}/company-{field}-{id}.{ext}: a company
// asset (logo) copied into a profile by a bulk action, next to the profile's own media.
func ProfileCompanyMediaObjectKey(profileUUID, field, id, ext string) string {
	return fmt.Sprintf("profiles/%s/company-%s-%s.%s", profileUUID, field, id, trimExt(ext))
}

// CompanyMediaPrefix is accounts/{accountUUID}/media/: the company media library (TEC-946).
func CompanyMediaPrefix(accountUUID string) string {
	return fmt.Sprintf("accounts/%s/media/", accountUUID)
}

// CompanyMediaObjectKey builds accounts/{accountUUID}/media/{mediaUUID}.{ext}.
func CompanyMediaObjectKey(accountUUID, mediaUUID, ext string) string {
	return fmt.Sprintf("%s%s.%s", CompanyMediaPrefix(accountUUID), mediaUUID, trimExt(ext))
}

// ProfileVCardObjectKey builds profiles/{profileUUID}/vcard_{locale}.vcf cached file in storage.
func ProfileVCardObjectKey(profileUUID, locale string) string {
	if locale == "" {
		locale = "default"
	}
	return fmt.Sprintf("profiles/%s/vcard_%s.vcf", profileUUID, locale)
}

// BackgroundThumbnailObjectKey builds backgrounds/{backgroundUUID}/thumb-{hash}.{ext}: the static
// stand-in of a Background Studio design (TEC-599). The content hash changes the URL whenever the
// design does, so cached copies never go stale.
func BackgroundThumbnailObjectKey(backgroundUUID, hash, ext string) string {
	return fmt.Sprintf("backgrounds/%s/thumb-%s.%s", backgroundUUID, hash, trimExt(ext))
}

// ProfileTempAvatarObjectKey builds tmp/avatars/{tempID}.{ext} for staged vCard photos.
func ProfileTempAvatarObjectKey(tempID, ext string) string {
	return fmt.Sprintf("tmp/avatars/%s.%s", tempID, trimExt(ext))
}

// ProductImageObjectKey builds products/{productSlug}/images/{imageID}.{ext}.
func ProductImageObjectKey(productSlug, imageID, ext string) string {
	return fmt.Sprintf("products/%s/images/%s.%s", productSlug, imageID, trimExt(ext))
}

// Product3DModelObjectKey builds products/{productSlug}/models/{modelID}.{ext}.
func Product3DModelObjectKey(productSlug, modelID, ext string) string {
	return fmt.Sprintf("products/%s/models/%s.%s", productSlug, modelID, trimExt(ext))
}

// OrderAssetObjectKey builds orders/{orderNumber}/assets/{assetUUID}.{ext}.
func OrderAssetObjectKey(orderNumber, assetUUID, ext string) string {
	return fmt.Sprintf("orders/%s/assets/%s.%s", orderNumber, assetUUID, trimExt(ext))
}

func trimExt(ext string) string {
	for len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}
	return ext
}
