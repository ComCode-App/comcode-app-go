package resourcemeta

import (
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
)

// ColumnType is the admin DataTable / meta column type contract.
type ColumnType string

const (
	ColumnTypeString   ColumnType = "string"
	ColumnTypeNumber   ColumnType = "number"
	ColumnTypeEnum     ColumnType = "enum"
	ColumnTypeBoolean  ColumnType = "boolean"
	ColumnTypeDatetime ColumnType = "datetime"
	ColumnTypeUUID     ColumnType = "uuid"
)

// FilterVariant describes how admin should render a filter control.
type FilterVariant string

const (
	FilterVariantText    FilterVariant = "text"
	FilterVariantFaceted FilterVariant = "faceted"
)

// Capabilities declares which list/resource operations the API actually supports.
type Capabilities struct {
	Create bool `json:"create"`
	Read   bool `json:"read"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
	Search bool `json:"search"`
	Filter bool `json:"filter"`
	Sort   bool `json:"sort"`
	Export bool `json:"export"`
	Import bool `json:"import"`
	Bulk   bool `json:"bulk"`
}

// Column describes one list-table column the API supports.
type Column struct {
	Key            string        `json:"key"`
	LabelKey       string        `json:"label_key"`
	Type           ColumnType    `json:"type"`
	Sortable       bool          `json:"sortable"`
	Filterable     bool          `json:"filterable"`
	FilterVariant  FilterVariant `json:"filter_variant,omitempty"`
	DefaultVisible bool          `json:"default_visible"`
}

// Filter describes one list filter the API supports.
type Filter struct {
	Key        string        `json:"key"`
	LabelKey   string        `json:"label_key"`
	Variant    FilterVariant `json:"variant"`
	EnumLookup string        `json:"enum_lookup,omitempty"`
}

// ResourceMeta is the table-metadata contract for a primary list resource.
type ResourceMeta struct {
	Resource         string       `json:"resource"`
	DefaultSort      string       `json:"default_sort"`
	DefaultFields    []string     `json:"default_fields"`
	Capabilities     Capabilities `json:"capabilities"`
	SearchableFields []string     `json:"searchable_fields"`
	SortableFields   []string     `json:"sortable_fields"`
	FilterableFields []string     `json:"filterable_fields"`
	Columns          []Column     `json:"columns"`
	Filters          []Filter     `json:"filters"`
	Includes         []string     `json:"includes"`
	BulkActions      []bulkengine.BulkActionDef `json:"bulk_actions"`
}

// PlatformUsers returns meta for GET /v1/platform/users.
func PlatformUsers() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.users",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "email", "name", "surname", "status"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: false,
			Search: true, Filter: true, Sort: true, Export: true, Import: true, Bulk: true,
		},
		SearchableFields: []string{"email", "name", "surname"},
		SortableFields:   []string{"email", "name", "surname", "status", "created_at", "updated_at"},
		FilterableFields: []string{"status", "role"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "users.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "email", LabelKey: "users.email", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "name", LabelKey: "users.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "surname", LabelKey: "users.surname", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "status", LabelKey: "users.status", Type: ColumnTypeEnum, Sortable: true, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
			{Key: "created_at", LabelKey: "users.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: false},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "users.status", Variant: FilterVariantFaceted, EnumLookup: "user_status"},
			{Key: "role", LabelKey: "users.role", Variant: FilterVariantFaceted, EnumLookup: "role_slug"},
		},
		Includes:    []string{"roles"},
		BulkActions: adapters.NewUsers(nil).BulkActions(),
	}
}

// PlatformRoles returns meta for GET /v1/platform/roles.
func PlatformRoles() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.roles",
		DefaultSort:   "name",
		DefaultFields: []string{"uuid", "name", "slug", "is_system"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: false, Sort: true, Export: true, Import: true, Bulk: true,
		},
		SearchableFields: []string{"name", "slug"},
		SortableFields:   []string{"name", "slug", "created_at", "updated_at"},
		FilterableFields: []string{},
		Columns: []Column{
			{Key: "uuid", LabelKey: "roles.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "roles.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "roles.slug", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "is_system", LabelKey: "roles.is_system", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "created_at", LabelKey: "roles.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: false},
		},
		Filters:     []Filter{},
		Includes:    []string{"permission_slugs"},
		BulkActions: adapters.NewRoles(nil).BulkActions(),
	}
}

// Notifications returns meta for GET /v1/notifications (inbox).
func Notifications() ResourceMeta {
	return ResourceMeta{
		Resource:      "notifications",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "channel", "status", "priority", "title", "created_at", "read_at"},
		Capabilities: Capabilities{
			Create: false, Read: true, Update: false, Delete: false,
			Search: true, Filter: true, Sort: true, Export: false, Import: false, Bulk: false,
		},
		SearchableFields: []string{"title", "body", "template_code", "recipient"},
		SortableFields:   []string{"channel", "status", "priority", "created_at", "updated_at", "sent_at"},
		FilterableFields: []string{"status", "channel", "unread"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "notifications.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "channel", LabelKey: "notifications.channel", Type: ColumnTypeEnum, Sortable: true, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
			{Key: "status", LabelKey: "notifications.status", Type: ColumnTypeEnum, Sortable: true, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
			{Key: "priority", LabelKey: "notifications.priority", Type: ColumnTypeEnum, Sortable: true, DefaultVisible: true},
			{Key: "title", LabelKey: "notifications.title", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "created_at", LabelKey: "notifications.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
			{Key: "read_at", LabelKey: "notifications.read_at", Type: ColumnTypeDatetime, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "notifications.status", Variant: FilterVariantFaceted},
			{Key: "channel", LabelKey: "notifications.channel", Variant: FilterVariantFaceted},
			{Key: "unread", LabelKey: "notifications.unread", Variant: FilterVariantFaceted},
		},
		Includes:    []string{},
		BulkActions: []bulkengine.BulkActionDef{},
	}
}

// PlatformNotifications returns meta for GET /v1/platform/notifications.
func PlatformNotifications() ResourceMeta {
	m := Notifications()
	m.Resource = "platform.notifications"
	m.Capabilities.Export = true
	return m
}

// Activity returns meta for GET /v1/platform/activity.
func Activity() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.activity",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"action", "resource", "actor_user_id", "created_at"},
		Capabilities: Capabilities{
			Read: true, Search: true, Filter: true, Sort: true, Export: true,
		},
		SearchableFields: []string{"action", "resource"},
		SortableFields:   []string{"created_at", "action", "resource"},
		Columns: []Column{
			{Key: "action", LabelKey: "activity.action", Sortable: true, DefaultVisible: true},
			{Key: "resource", LabelKey: "activity.resource", Sortable: true, DefaultVisible: true},
			{Key: "actor_user_id", LabelKey: "activity.actor", DefaultVisible: true},
			{Key: "created_at", LabelKey: "activity.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
		},
	}
}

// PlatformStorage returns meta for GET /v1/platform/storage/objects.
func PlatformStorage() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.storage",
		DefaultSort:   "name",
		DefaultFields: []string{"name", "kind", "size", "updated_at", "access"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true, Export: false, Import: false, Bulk: false,
		},
		SearchableFields: []string{"name", "key", "mime_type"},
		SortableFields:   []string{"name", "size", "updated_at", "type"},
		FilterableFields: []string{"kind", "access"},
		Columns: []Column{
			{Key: "name", LabelKey: "storage.col_name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "kind", LabelKey: "storage.col_type", Type: ColumnTypeEnum, Sortable: true, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
			{Key: "size", LabelKey: "storage.col_size", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "updated_at", LabelKey: "storage.col_modified", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
			{Key: "access", LabelKey: "storage.col_access", Type: ColumnTypeEnum, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "kind", LabelKey: "storage.col_type", Variant: FilterVariantFaceted},
			{Key: "access", LabelKey: "storage.col_access", Variant: FilterVariantFaceted},
		},
		BulkActions: []bulkengine.BulkActionDef{},
	}
}

// PlatformLogs returns meta for GET /v1/platform/logs.
func PlatformLogs() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.logs",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"level", "message", "source", "created_at"},
		Capabilities: Capabilities{
			Read: true, Delete: true, Search: true, Filter: true, Sort: true, Bulk: false,
		},
		SearchableFields: []string{"message", "source", "request_id"},
		SortableFields:   []string{"created_at", "level", "source"},
		FilterableFields: []string{"level", "source", "created_from", "created_to"},
		Columns: []Column{
			{Key: "level", LabelKey: "logs.columns.level", Type: ColumnTypeEnum, Sortable: true, Filterable: true, FilterVariant: FilterVariantFaceted, DefaultVisible: true},
			{Key: "message", LabelKey: "logs.columns.message", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "source", LabelKey: "logs.columns.source", Type: ColumnTypeString, Sortable: true, Filterable: true, DefaultVisible: true},
			{Key: "created_at", LabelKey: "logs.columns.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "level", LabelKey: "logs.columns.level", Variant: FilterVariantFaceted},
			{Key: "source", LabelKey: "logs.columns.source", Variant: FilterVariantText},
		},
		BulkActions: []bulkengine.BulkActionDef{},
	}
}

// PlatformLogRules returns meta for GET /v1/platform/log-rules.
func PlatformLogRules() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.log_rules",
		DefaultSort:   "name",
		DefaultFields: []string{"name", "enabled", "levels", "older_than_hours", "interval_minutes"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true, Sort: true,
		},
		SortableFields: []string{"name", "created_at", "updated_at"},
		Columns: []Column{
			{Key: "name", LabelKey: "logs.rules.columns.name", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "enabled", LabelKey: "logs.rules.columns.enabled", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "older_than_hours", LabelKey: "logs.rules.columns.older_than", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "interval_minutes", LabelKey: "logs.rules.columns.schedule", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "last_run_at", LabelKey: "logs.rules.columns.last_run", Type: ColumnTypeDatetime, DefaultVisible: true},
		},
	}
}

// PlatformPlans returns meta for GET /v1/platform/plans.
func PlatformPlans() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.plans",
		DefaultSort:   "sort_order",
		DefaultFields: []string{"uuid", "slug", "name", "billing_interval", "price_minor", "currency", "is_active"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: false, Sort: true,
		},
		SearchableFields: []string{"name", "slug"},
		SortableFields:   []string{"name", "slug", "sort_order", "created_at", "updated_at"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "plans.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "plans.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "plans.slug", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "billing_interval", LabelKey: "plans.billing_interval", Type: ColumnTypeEnum, DefaultVisible: true},
			{Key: "price_minor", LabelKey: "plans.price", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "currency", LabelKey: "plans.currency", Type: ColumnTypeString, DefaultVisible: false},
			{Key: "trial_days", LabelKey: "plans.trial_days", Type: ColumnTypeString, DefaultVisible: false},
			{Key: "is_active", LabelKey: "plans.is_active", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "sort_order", LabelKey: "plans.sort_order", Type: ColumnTypeString, Sortable: true, DefaultVisible: false},
		},
	}
}

// PlatformSubscriptions returns meta for GET /v1/platform/subscriptions.
func PlatformSubscriptions() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.subscriptions",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "status", "plan_name", "subject"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: false,
			Search: true, Filter: true, Sort: false,
		},
		SearchableFields: []string{"plan_name", "plan_slug"},
		FilterableFields: []string{"status"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "subscriptions.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "status", LabelKey: "subscriptions.status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "plan_name", LabelKey: "subscriptions.plan_name", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "subject", LabelKey: "subscriptions.subject", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "starts_at", LabelKey: "subscriptions.starts_at", Type: ColumnTypeDatetime, DefaultVisible: true},
			{Key: "ends_at", LabelKey: "subscriptions.ends_at", Type: ColumnTypeDatetime, DefaultVisible: false},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "subscriptions.status", Variant: FilterVariantFaceted},
		},
	}
}

// PlatformAccounts returns meta for GET /v1/platform/accounts.
func PlatformAccounts() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.accounts",
		DefaultSort:   "name",
		DefaultFields: []string{"uuid", "name", "slug", "type", "status", "member_count"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true,
			Search: true, Filter: true,
		},
		SearchableFields: []string{"name", "slug"},
		SortableFields:   []string{"name"},
		FilterableFields: []string{"type", "status"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "accounts.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "accounts.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "accounts.slug", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "type", LabelKey: "accounts.type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "status", LabelKey: "accounts.status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "member_count", LabelKey: "accounts.member_count", Type: ColumnTypeNumber, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "type", LabelKey: "accounts.type", Variant: FilterVariantFaceted},
			{Key: "status", LabelKey: "accounts.status", Variant: FilterVariantFaceted},
		},
	}
}

// PlatformSocialPlatforms returns meta for GET /v1/platform/social-platforms.
func PlatformSocialPlatforms() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.social_platforms",
		DefaultSort:   "display_order",
		DefaultFields: []string{"uuid", "name", "slug", "category", "url_pattern", "is_active", "display_order"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"name", "slug", "category"},
		SortableFields:   []string{"name", "slug", "display_order", "created_at"},
		FilterableFields: []string{"category", "is_active"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "social_platforms.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "social_platforms.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "social_platforms.slug", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "category", LabelKey: "social_platforms.category", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "url_pattern", LabelKey: "social_platforms.url_pattern", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "is_active", LabelKey: "social_platforms.is_active", Type: ColumnTypeBoolean, Filterable: true, DefaultVisible: true},
			{Key: "display_order", LabelKey: "social_platforms.display_order", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "category", LabelKey: "social_platforms.category", Variant: FilterVariantFaceted},
			{Key: "is_active", LabelKey: "social_platforms.is_active", Variant: FilterVariantFaceted},
		},
	}
}

// PlatformBanks returns meta for GET /v1/platform/banks.
func PlatformBanks() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.banks",
		DefaultSort:   "display_order",
		DefaultFields: []string{"uuid", "name", "slug", "code", "country_code", "bic_swift", "is_active", "display_order"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"name", "slug", "code", "bic_swift"},
		SortableFields:   []string{"name", "slug", "code", "country_code", "display_order", "created_at"},
		FilterableFields: []string{"country_code", "is_active"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "banks.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "banks.fields.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "code", LabelKey: "banks.fields.code", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "country_code", LabelKey: "banks.fields.country_code", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "bic_swift", LabelKey: "banks.fields.bic_swift", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "is_active", LabelKey: "banks.fields.is_active", Type: ColumnTypeBoolean, Filterable: true, DefaultVisible: true},
			{Key: "display_order", LabelKey: "banks.fields.display_order", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "country_code", LabelKey: "banks.fields.country_code", Variant: FilterVariantFaceted},
			{Key: "is_active", LabelKey: "banks.fields.is_active", Variant: FilterVariantFaceted},
		},
	}
}

// AppBankAccounts returns meta for GET /v1/bank-accounts.
func AppBankAccounts() ResourceMeta {
	return ResourceMeta{
		Resource:      "app.bank_accounts",
		DefaultSort:   "display_order",
		DefaultFields: []string{"uuid", "account_holder", "iban", "bank_name", "currency", "is_primary", "is_active"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"account_holder", "iban", "title", "bank_name"},
		SortableFields:   []string{"account_holder", "iban", "display_order", "created_at"},
		FilterableFields: []string{"currency", "is_active"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "bank_accounts.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "account_holder", LabelKey: "bank_accounts.fields.account_holder", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "bank_name", LabelKey: "bank_accounts.fields.bank_name", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "iban", LabelKey: "bank_accounts.fields.iban", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "currency", LabelKey: "bank_accounts.fields.currency", Type: ColumnTypeEnum, DefaultVisible: true},
			{Key: "is_primary", LabelKey: "bank_accounts.fields.is_primary", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "is_active", LabelKey: "bank_accounts.fields.is_active", Type: ColumnTypeBoolean, DefaultVisible: true},
		},
	}
}

// AppLeads returns meta for GET /v1/leads/meta (TEC-552).
func AppLeads() ResourceMeta {
	return ResourceMeta{
		Resource:      "app.leads",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "kind", "name", "email", "phone", "company", "source_type", "route_path", "read_at", "created_at"},
		Capabilities: Capabilities{
			Read: true, Delete: true, Search: true, Filter: true, Export: true,
		},
		SearchableFields: []string{"name", "email", "phone", "company"},
		SortableFields:   []string{"created_at"},
		FilterableFields: []string{"kind", "source_type", "unread"},
		Columns: []Column{
			{Key: "name", LabelKey: "leads.fields.name", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "email", LabelKey: "leads.fields.email", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "phone", LabelKey: "leads.fields.phone", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "company", LabelKey: "leads.fields.company", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "kind", LabelKey: "leads.fields.kind", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "source_type", LabelKey: "leads.fields.source_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "route_path", LabelKey: "leads.fields.route_path", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "read_at", LabelKey: "leads.fields.read_at", Type: ColumnTypeDatetime, DefaultVisible: true},
			{Key: "created_at", LabelKey: "leads.fields.created_at", Type: ColumnTypeDatetime, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "kind", LabelKey: "leads.fields.kind", Variant: FilterVariantFaceted},
			{Key: "source_type", LabelKey: "leads.fields.source_type", Variant: FilterVariantFaceted},
			{Key: "unread", LabelKey: "leads.filters.unread", Variant: FilterVariantFaceted},
		},
	}
}

// AppProfiles returns meta for GET /v1/profiles.
func AppProfiles() ResourceMeta {
	return ResourceMeta{
		Resource:      "app.profiles",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "first_name", "last_name", "title", "company_name", "primary_locale", "is_primary", "completion_rate"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"first_name", "last_name", "title", "company_name"},
		SortableFields:   []string{"first_name", "last_name", "created_at", "updated_at"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "profiles.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "first_name", LabelKey: "profiles.fields.first_name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "last_name", LabelKey: "profiles.fields.last_name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "title", LabelKey: "profiles.fields.title", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "company_name", LabelKey: "profiles.fields.company_name", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "primary_locale", LabelKey: "profiles.fields.primary_locale", Type: ColumnTypeEnum, DefaultVisible: true},
			{Key: "is_primary", LabelKey: "profiles.fields.is_primary", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "completion_rate", LabelKey: "profiles.fields.completion_rate", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "created_at", LabelKey: "profiles.fields.created_at", Type: ColumnTypeDatetime, DefaultVisible: true},
		},
	}
}

// PlatformCategories returns meta for GET /v1/platform/categories/meta.
func PlatformCategories() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.categories",
		DefaultSort:   "display_order",
		DefaultFields: []string{"uuid", "name", "slug", "icon", "is_active", "display_order"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"name", "slug", "description"},
		SortableFields:   []string{"name", "slug", "display_order", "created_at"},
		FilterableFields: []string{"is_active"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "categories.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "categories.fields.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "categories.fields.slug", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "icon", LabelKey: "categories.fields.icon", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "is_active", LabelKey: "categories.fields.is_active", Type: ColumnTypeBoolean, Filterable: true, DefaultVisible: true},
			{Key: "display_order", LabelKey: "categories.fields.display_order", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "is_active", LabelKey: "categories.fields.is_active", Variant: FilterVariantFaceted},
		},
	}
}

// PlatformProducts returns meta for GET /v1/platform/products/meta.
func PlatformProducts() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.products",
		DefaultSort:   "display_order",
		DefaultFields: []string{"uuid", "name", "slug", "category_name", "status", "product_type", "base_price", "currency", "stock_quantity"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"name", "slug", "sku", "short_description"},
		SortableFields:   []string{"name", "slug", "base_price", "stock_quantity", "display_order", "created_at"},
		FilterableFields: []string{"category_id", "status", "product_type"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "products.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "products.fields.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "slug", LabelKey: "products.fields.slug", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "category_name", LabelKey: "products.fields.category", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "status", LabelKey: "products.fields.status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "product_type", LabelKey: "products.fields.type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "base_price", LabelKey: "products.fields.base_price", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "currency", LabelKey: "products.fields.currency", Type: ColumnTypeString, DefaultVisible: false},
			{Key: "stock_quantity", LabelKey: "products.fields.stock", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "created_at", LabelKey: "products.fields.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: false},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "products.fields.status", Variant: FilterVariantFaceted},
			{Key: "product_type", LabelKey: "products.fields.type", Variant: FilterVariantFaceted},
		},
	}
}

// PlatformOrders returns meta for GET /v1/platform/orders/meta.
func PlatformOrders() ResourceMeta {
	return ResourceMeta{
		Resource:      "platform.orders",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "order_number", "order_type", "items_count", "customer_name", "status", "payment_status", "fulfillment_status", "grand_total", "currency", "created_at"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"order_number", "customer_email", "customer_name", "tracking_number"},
		SortableFields:   []string{"order_number", "grand_total", "created_at", "updated_at"},
		FilterableFields: []string{"status", "payment_status", "fulfillment_status", "order_type", "user_id"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "orders.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "order_number", LabelKey: "orders.fields.order_number", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "order_type", LabelKey: "orders.fields.order_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "items_count", LabelKey: "orders.fields.items_count", Type: ColumnTypeNumber, DefaultVisible: true},
			{Key: "customer_name", LabelKey: "orders.fields.customer", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "status", LabelKey: "orders.fields.status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "payment_status", LabelKey: "orders.fields.payment_status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "fulfillment_status", LabelKey: "orders.fields.fulfillment_status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "grand_total", LabelKey: "orders.fields.grand_total", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "currency", LabelKey: "orders.fields.currency", Type: ColumnTypeString, DefaultVisible: false},
			{Key: "tracking_number", LabelKey: "orders.fields.tracking_number", Type: ColumnTypeString, DefaultVisible: false},
			{Key: "created_at", LabelKey: "orders.fields.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "orders.fields.status", Variant: FilterVariantFaceted},
			{Key: "payment_status", LabelKey: "orders.fields.payment_status", Variant: FilterVariantFaceted},
			{Key: "fulfillment_status", LabelKey: "orders.fields.fulfillment_status", Variant: FilterVariantFaceted},
			{Key: "order_type", LabelKey: "orders.fields.order_type", Variant: FilterVariantFaceted},
		},
	}
}

// AppCards returns meta for GET /v1/cards/meta.
func AppCards() ResourceMeta {
	return ResourceMeta{
		Resource:      "app.cards",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "name", "smart_code", "target_type", "card_type", "material", "is_active", "is_locked", "created_at"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"name", "smart_code", "card_uid"},
		SortableFields:   []string{"name", "created_at"},
		FilterableFields: []string{"is_active", "target_type", "card_type"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "cards.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "name", LabelKey: "cards.fields.name", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "smart_code", LabelKey: "cards.fields.smart_code", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "target_type", LabelKey: "cards.fields.target_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "card_type", LabelKey: "cards.fields.card_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "material", LabelKey: "cards.fields.material", Type: ColumnTypeString, DefaultVisible: true},
			{Key: "is_active", LabelKey: "cards.fields.is_active", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "is_locked", LabelKey: "cards.fields.is_locked", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "created_at", LabelKey: "cards.fields.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "is_active", LabelKey: "cards.fields.is_active", Variant: FilterVariantFaceted},
			{Key: "target_type", LabelKey: "cards.fields.target_type", Variant: FilterVariantFaceted},
		},
	}
}

// AppRoutes returns meta for GET /v1/routes/meta.
func AppRoutes() ResourceMeta {
	return ResourceMeta{
		Resource:      "app.routes",
		DefaultSort:   "-created_at",
		DefaultFields: []string{"uuid", "path", "route_type", "status", "target_type", "is_immutable", "created_at"},
		Capabilities: Capabilities{
			Create: true, Read: true, Update: true, Delete: true,
			Search: true, Filter: true, Sort: true,
		},
		SearchableFields: []string{"path", "direct_url"},
		SortableFields:   []string{"path", "created_at"},
		FilterableFields: []string{"status", "route_type", "target_type"},
		Columns: []Column{
			{Key: "uuid", LabelKey: "routes.fields.uuid", Type: ColumnTypeUUID, DefaultVisible: true},
			{Key: "path", LabelKey: "routes.fields.path", Type: ColumnTypeString, Sortable: true, DefaultVisible: true},
			{Key: "route_type", LabelKey: "routes.fields.route_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "status", LabelKey: "routes.fields.status", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "target_type", LabelKey: "routes.fields.target_type", Type: ColumnTypeEnum, Filterable: true, DefaultVisible: true},
			{Key: "is_immutable", LabelKey: "routes.fields.is_immutable", Type: ColumnTypeBoolean, DefaultVisible: true},
			{Key: "created_at", LabelKey: "routes.fields.created_at", Type: ColumnTypeDatetime, Sortable: true, DefaultVisible: true},
		},
		Filters: []Filter{
			{Key: "status", LabelKey: "routes.fields.status", Variant: FilterVariantFaceted},
			{Key: "route_type", LabelKey: "routes.fields.route_type", Variant: FilterVariantFaceted},
			{Key: "target_type", LabelKey: "routes.fields.target_type", Variant: FilterVariantFaceted},
		},
	}
}




