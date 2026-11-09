package domain

// Entry sources stored on the aggregate rows (TEC-953): how a visitor reached a profile, card or
// link. Events that are not entries (page views, clicks, ...) have no source ("").
const (
	SourceQR     = "qr"
	SourceNFC    = "nfc"
	SourceLink   = "link"
	SourceLegacy = "legacy"
)

// Entry event types.
const (
	EventQRScan            = "card.qr.scan"
	EventRouteScan         = "route.scan"
	EventLegacyProfileView = "legacy.profile.view"
)

// EntrySourceProperty marks a route.scan that was opened from a branded QR code (?src=qr), so the
// scan is not counted again as an NFC tap or link click.
const EntrySourceProperty = "entry_source"

// EntrySource classifies an event for the source breakdown. QR scans are always "qr"; route scans
// follow their route type (card route = NFC tap, legacy hash route = "eski kart", smart link =
// link) unless the scan was QR-marked; imported legacy page hits are "legacy".
func EntrySource(e Event) string {
	switch e.Type {
	case EventQRScan:
		return SourceQR
	case EventLegacyProfileView:
		return SourceLegacy
	case EventRouteScan:
		if s, _ := e.Properties[EntrySourceProperty].(string); s == SourceQR {
			return SourceQR
		}
		routeType, _ := e.Properties["route_type"].(string)
		return RouteTypeSource(routeType)
	}
	return ""
}

// RouteTypeSource maps a route type to its entry source; other route types have none.
func RouteTypeSource(routeType string) string {
	switch routeType {
	case "card_route":
		return SourceNFC
	case "legacy_route":
		return SourceLegacy
	case "custom_smart_link":
		return SourceLink
	}
	return ""
}
