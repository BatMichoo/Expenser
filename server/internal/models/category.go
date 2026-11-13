package models

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// General marks a catch-all category (Vegetables, Meat, ...) that groups
	// different products, so its price per unit is only a rough guide.
	General bool `json:"general"`
}
