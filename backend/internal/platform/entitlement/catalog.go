package entitlement

// Feature keys are code-defined; DB feature_definitions must stay in sync via migrations. Every
// plan defines a value for every feature, so a migration adding one also writes it for all plans.
const (
	// FeatureCardsMax caps NFC cards that are active in an account (checked on activation).
	FeatureCardsMax = "cards.max"
	// FeatureProfilesMax caps profiles per account (checked on create).
	FeatureProfilesMax = "profiles.max"
	// FeatureSmartLinksMax caps custom smart links per account (checked on create).
	FeatureSmartLinksMax = "smart_links.max"
	// FeatureAnalyticsHistoryDays is how far back account analytics can be queried.
	FeatureAnalyticsHistoryDays = "analytics.history_days"
	// FeatureLeadsMonthlyMax caps the lead form submissions an account sees per calendar month;
	// more are stored locked (TEC-941). 0 turns the lead form off.
	FeatureLeadsMonthlyMax = "leads.monthly_max"
	// FeatureExchangeMonthlyMax is the same for contact exchanges. 0 turns exchange off.
	FeatureExchangeMonthlyMax = "exchange.monthly_max"
	// FeatureLeadsExportMonthlyMax caps lead exports per calendar month. 0 turns export off.
	FeatureLeadsExportMonthlyMax = "leads.export.monthly_max"
	// FeatureDesignPremium unlocks premium themes and animated backgrounds (TEC-947). Without it
	// saving one is refused and public pages render the basic theme / background instead.
	FeatureDesignPremium = "design.premium"
)

const FreePlanSlug = "free"

// FeatureDef describes a limitable feature in code.
type FeatureDef struct {
	Key       string
	ValueType string // bool | int
	// Quota marks int features whose value caps a counted resource (see Service.usage).
	Quota bool
	// Monthly marks int features counted per calendar month (see monthly.go).
	Monthly bool
}

// Catalog returns the canonical feature catalog.
func Catalog() []FeatureDef {
	return []FeatureDef{
		{Key: FeatureCardsMax, ValueType: "int", Quota: true},
		{Key: FeatureProfilesMax, ValueType: "int", Quota: true},
		{Key: FeatureSmartLinksMax, ValueType: "int", Quota: true},
		{Key: FeatureAnalyticsHistoryDays, ValueType: "int"},
		{Key: FeatureLeadsMonthlyMax, ValueType: "int", Monthly: true},
		{Key: FeatureExchangeMonthlyMax, ValueType: "int", Monthly: true},
		{Key: FeatureLeadsExportMonthlyMax, ValueType: "int", Monthly: true},
		{Key: FeatureDesignPremium, ValueType: "bool"},
	}
}
