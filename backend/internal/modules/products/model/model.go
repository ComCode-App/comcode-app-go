package model

import "time"

type CameraOrbit struct {
	Pitch    float64 `json:"pitch"`
	Yaw      float64 `json:"yaw"`
	Distance float64 `json:"distance"`
}

type StageBounds struct {
	Radius          float64 `json:"radius"`
	Height          float64 `json:"height"`
	ShadowIntensity float64 `json:"shadow_intensity"`
}

type OrbitLimits struct {
	MinPitch float64 `json:"min_pitch"`
	MaxPitch float64 `json:"max_pitch"`
	MinYaw   float64 `json:"min_yaw"`
	MaxYaw   float64 `json:"max_yaw"`
	MinZoom  float64 `json:"min_zoom"`
	MaxZoom  float64 `json:"max_zoom"`
}

type Model3DViewbox struct {
	CameraOrbit       CameraOrbit  `json:"camera_orbit"`
	FOV               float64      `json:"fov"`
	AutoRotate        bool         `json:"auto_rotate"`
	RotationSpeed     float64      `json:"rotation_speed"`
	StageBounds       StageBounds  `json:"stage_bounds"`
	OrbitLimits       *OrbitLimits `json:"orbit_limits,omitempty"`
	EnvironmentPreset string       `json:"environment_preset,omitempty"`
	BackgroundColor   string       `json:"background_color,omitempty"`
}

type ProductVariantSummary struct {
	UUID           string         `json:"uuid"`
	Name           string         `json:"name"`
	SKU            string         `json:"sku"`
	PriceDelta     float64        `json:"price_delta"`
	StockQuantity  int            `json:"stock_quantity"`
	Material       string         `json:"material"`
	ColorCode      string         `json:"color_code"`
	ImageURL       string         `json:"image_url"`
	Model3DFileURL string         `json:"model_3d_file_url"`
	Model3DUSDZURL string         `json:"model_3d_usdz_url"`
	Model3DViewbox Model3DViewbox `json:"model_3d_viewbox"`
	IsActive       bool           `json:"is_active"`
	DisplayOrder   int            `json:"display_order"`
}

type ProductSummary struct {
	UUID             string    `json:"uuid"`
	CategoryName     string    `json:"category_name,omitempty"`
	CategorySlug     string    `json:"category_slug,omitempty"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	ShortDescription string    `json:"short_description"`
	Status           string    `json:"status"`
	ProductType      string    `json:"product_type"`
	BasePrice        float64   `json:"base_price"`
	Currency         string    `json:"currency"`
	StockQuantity    int       `json:"stock_quantity"`
	SKU              string    `json:"sku"`
	AllowCustomPrint bool      `json:"allow_custom_print"`
	Model3DFileURL   string    `json:"model_3d_file_url"`
	Model3DUSDZURL   string    `json:"model_3d_usdz_url"`
	Images           []string  `json:"images"`
	DisplayOrder     int       `json:"display_order"`
	CreatedAt        time.Time `json:"created_at"`
}

type ProductDetail struct {
	ID               int64                   `json:"-"`
	UUID             string                  `json:"uuid"`
	CategoryID       *int64                  `json:"-"`
	CategoryUUID     string                  `json:"category_uuid,omitempty"`
	CategoryName     string                  `json:"category_name,omitempty"`
	CategorySlug     string                  `json:"category_slug,omitempty"`
	Name             string                  `json:"name"`
	Slug             string                  `json:"slug"`
	ShortDescription string                  `json:"short_description"`
	Description      string                  `json:"description"`
	Status           string                  `json:"status"`
	ProductType      string                  `json:"product_type"`
	BasePrice        float64                 `json:"base_price"`
	Currency         string                  `json:"currency"`
	TaxRate          float64                 `json:"tax_rate"`
	StockQuantity    int                     `json:"stock_quantity"`
	SKU              string                  `json:"sku"`
	AllowCustomPrint bool                    `json:"allow_custom_print"`
	Model3DFileURL   string                  `json:"model_3d_file_url"`
	Model3DUSDZURL   string                  `json:"model_3d_usdz_url"`
	Model3DViewbox   Model3DViewbox          `json:"model_3d_viewbox"`
	Images           []string                `json:"images"`
	MetaTitle        string                  `json:"meta_title"`
	MetaDescription  string                  `json:"meta_description"`
	MetaKeywords     string                  `json:"meta_keywords"`
	CanonicalURL     string                  `json:"canonical_url"`
	OGImageURL       string                  `json:"og_image_url"`
	DisplayOrder     int                     `json:"display_order"`
	Variants         []ProductVariantSummary `json:"variants"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

type UpsertProductInput struct {
	CategoryUUID     *string         `json:"category_uuid,omitempty"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug,omitempty"`
	ShortDescription string          `json:"short_description,omitempty"`
	Description      string          `json:"description,omitempty"`
	Status           string          `json:"status,omitempty"`
	ProductType      string          `json:"product_type,omitempty"`
	BasePrice        float64         `json:"base_price"`
	Currency         string          `json:"currency,omitempty"`
	TaxRate          float64         `json:"tax_rate,omitempty"`
	StockQuantity    int             `json:"stock_quantity"`
	SKU              string          `json:"sku,omitempty"`
	AllowCustomPrint bool            `json:"allow_custom_print"`
	Model3DFileURL   string          `json:"model_3d_file_url,omitempty"`
	Model3DUSDZURL   string          `json:"model_3d_usdz_url,omitempty"`
	Model3DViewbox   *Model3DViewbox `json:"model_3d_viewbox,omitempty"`
	Images           []string        `json:"images,omitempty"`
	MetaTitle        string          `json:"meta_title,omitempty"`
	MetaDescription  string          `json:"meta_description,omitempty"`
	MetaKeywords     string          `json:"meta_keywords,omitempty"`
	CanonicalURL     string          `json:"canonical_url,omitempty"`
	OGImageURL       string          `json:"og_image_url,omitempty"`
	DisplayOrder     int             `json:"display_order"`
}

type Update3DViewboxInput struct {
	Model3DViewbox Model3DViewbox `json:"model_3d_viewbox"`
	Model3DFileURL string         `json:"model_3d_file_url,omitempty"`
	Model3DUSDZURL string         `json:"model_3d_usdz_url,omitempty"`
}

type UpsertVariantInput struct {
	Name           string          `json:"name"`
	SKU            string          `json:"sku,omitempty"`
	PriceDelta     float64         `json:"price_delta"`
	StockQuantity  int             `json:"stock_quantity"`
	Material       string          `json:"material,omitempty"`
	ColorCode      string          `json:"color_code,omitempty"`
	ImageURL       string          `json:"image_url,omitempty"`
	Model3DFileURL string          `json:"model_3d_file_url,omitempty"`
	Model3DUSDZURL string          `json:"model_3d_usdz_url,omitempty"`
	Model3DViewbox *Model3DViewbox `json:"model_3d_viewbox,omitempty"`
	IsActive       bool            `json:"is_active"`
	DisplayOrder   int             `json:"display_order"`
}
