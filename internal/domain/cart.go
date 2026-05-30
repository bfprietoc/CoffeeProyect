package domain

type Cart struct {
	ID         string     `json:"id"`
	Items      []CartItem `json:"items"`
	TotalCents int        `json:"total_cents"`
	Currency   string     `json:"currency"`
}

type CartItem struct {
	CoffeeID       string `json:"coffee_id"`
	CoffeeName     string `json:"coffee_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int    `json:"unit_price_cents"`
	BagSizeGrams   int    `json:"bag_size_grams"`
	SubtotalCents  int    `json:"subtotal_cents"`
}

// CartIdentity identifica de quién es el carrito y si es invitado o usuario.
type CartIdentity struct {
	ID      string
	IsGuest bool
}
