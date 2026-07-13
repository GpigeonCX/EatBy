package pantry

type Location struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}
type Product struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	DefaultUnit   string   `json:"default_unit"`
	TrackingMode  string   `json:"tracking_mode"`
	LowThreshold  *float64 `json:"low_threshold"`
	AfterOpenDays *int     `json:"after_open_days"`
	Barcode       *string  `json:"barcode"`
	Favorite      bool     `json:"favorite"`
	TotalQuantity float64  `json:"total_quantity,omitempty"`
	BatchCount    int      `json:"batch_count,omitempty"`
	StockState    string   `json:"stock_state,omitempty"`
}
type Batch struct {
	ID              string   `json:"id"`
	ProductID       string   `json:"product_id"`
	LocationID      string   `json:"location_id"`
	Quantity        *float64 `json:"quantity"`
	Level           *string  `json:"level"`
	ExpiryDate      *string  `json:"expiry_date"`
	OpenedAt        *string  `json:"opened_at"`
	Note            string   `json:"note"`
	Version         int      `json:"version"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	ProductName     string   `json:"product_name,omitempty"`
	LocationName    string   `json:"location_name,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	EffectiveExpiry *string  `json:"effective_expiry,omitempty"`
	ExpiryStatus    string   `json:"expiry_status,omitempty"`
}
type ShoppingItem struct {
	ID        string   `json:"id"`
	ProductID *string  `json:"product_id"`
	Name      string   `json:"name"`
	Quantity  *float64 `json:"quantity"`
	Unit      *string  `json:"unit"`
	Checked   bool     `json:"checked"`
}
type UserSummary struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
}
