package model

import "time"

type CategorySummary struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	Icon         string `json:"icon"`
	ImageURL     string `json:"image_url"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}

type CategoryDetail struct {
	ID           int64     `json:"-"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	Icon         string    `json:"icon"`
	ImageURL     string    `json:"image_url"`
	DisplayOrder int       `json:"display_order"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpsertCategoryInput struct {
	Name         string `json:"name"`
	Slug         string `json:"slug,omitempty"`
	Description  string `json:"description,omitempty"`
	Icon         string `json:"icon,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	DisplayOrder int    `json:"display_order"`
	IsActive     bool   `json:"is_active"`
}
