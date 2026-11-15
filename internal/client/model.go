package client

import (
	"encoding/json"
	"time"
)

type Product struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Handle      string    `json:"handle"`
	BodyHTML    string    `json:"body_html,omitempty"`
	Description string    `json:"description,omitempty"`
	Vendor      string    `json:"vendor"`
	Type        string    `json:"product_type,omitempty"`
	Tags        []string  `json:"tags"`
	Variants    []Variant `json:"variants"`
	Images      []Image   `json:"images,omitempty"`
}

type Variant struct {
	ID        int64           `json:"id"`
	Title     string          `json:"title"`
	SKU       string          `json:"sku"`
	Available bool            `json:"available"`
	Price     json.RawMessage `json:"price"`
	Grams     int             `json:"grams"`
}

type Image struct {
	Src string `json:"src"`
}

func (i *Image) UnmarshalJSON(data []byte) error {
	var src string
	if err := json.Unmarshal(data, &src); err == nil {
		i.Src = src
		return nil
	}
	type image Image
	var value image
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*i = Image(value)
	return nil
}

type Collection struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Handle        string `json:"handle"`
	Description   string `json:"description"`
	ProductsCount int    `json:"products_count"`
}

type productsResp struct {
	Products []Product `json:"products"`
}

type collectionsResp struct {
	Collections []Collection `json:"collections"`
}

type CatalogCache struct {
	SavedAt  time.Time `json:"saved_at"`
	Products []Product `json:"products"`
}
