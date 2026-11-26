package usecase

// Product and card types of the badge (yaka kartı) product (TEC-1320). A badge has an NFC chip
// and goes through the same activation and smart link flow as a card; only its preview is
// vertical (54×85.6 mm).
const (
	ProductTypeBadge = "badge"
	CardTypeBadge    = "badge"
	CardTypeNFC      = "physical_nfc"
)

// cardTypeForProduct is the card_type a provisioned card gets for the ordered product's type.
func cardTypeForProduct(productType string) string {
	if productType == ProductTypeBadge {
		return CardTypeBadge
	}
	return CardTypeNFC
}
