package domain

type Producer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Farm struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Region  string `json:"region"`
}

type Coffee struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Process      string   `json:"process"`
	RoastLevel   string   `json:"roast_level"`
	TastingNotes []string `json:"tasting_notes"`
	Description  string   `json:"description"`
	Producer     Producer `json:"producer"`
	Farm         Farm     `json:"farm"`
	BagSizeGrams int      `json:"bag_size_grams"`
	PriceCents   int      `json:"price_cents"`
	Currency     string   `json:"currency"`
	Available    bool     `json:"available"`
	StockBags    int      `json:"stock_bags"`
}
