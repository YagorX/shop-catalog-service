package redis

type productCacheValue struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	Stock       int32  `json:"stock"`
	Active      bool   `json:"active"`
}

type productListCacheValue struct {
	Items []productCacheValue `json:"items"`
}
