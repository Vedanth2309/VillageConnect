package models

import "time"

// Village represents a mapped village community within the marketplace.
type Village struct {
	ID       string `json:"id" bson:"_id"`
	Name     string `json:"name" bson:"name"`
	District string `json:"district" bson:"district"`
	Taluk    string `json:"taluk" bson:"taluk"`
	State    string `json:"state" bson:"state"`
}

// Category represents the product grouping used in marketplace discovery.
type Category struct {
	ID   string `json:"id" bson:"_id"`
	Name string `json:"name" bson:"name"`
}

// Product represents a catalog item sold by a local seller.
type Product struct {
	ID          string  `json:"id" bson:"_id"`
	Name        string  `json:"name" bson:"name"`
	Category    string  `json:"category" bson:"category"`
	Price       float64 `json:"price" bson:"price"`
	Rating      float64 `json:"rating" bson:"rating"`
	Unit        string  `json:"unit" bson:"unit"`
	Village     string  `json:"village" bson:"village"`
	VillageID   string  `json:"villageId,omitempty" bson:"villageId,omitempty"`
	SellerID    string  `json:"sellerId,omitempty" bson:"sellerId,omitempty"`
	Seller      string  `json:"seller" bson:"seller"`
	Available   bool    `json:"available" bson:"available"`
	Stock       int     `json:"stock" bson:"stock"`
	Description string  `json:"description" bson:"description"`
}

type ProductQuery struct {
	VillageID   string
	VillageName string
	Query       string
	Category    string
	MinPrice    *float64
	MaxPrice    *float64
	Available   *bool
	Sort        string
	Page        int
	PageSize    int
}

type ProductPage struct {
	Products   []Product `json:"products"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"pageSize"`
	TotalPages int       `json:"totalPages"`
}

// UserRole describes the access level granted to a platform account.
type UserRole string

const (
	RoleCustomer      UserRole = "customer"
	RoleSeller        UserRole = "seller"
	RoleDeliveryAgent UserRole = "delivery_agent"
	RoleAdmin         UserRole = "admin"
)

// User is the base identity model used by authentication and authorization.
type User struct {
	ID           string   `json:"id" bson:"_id"`
	Name         string   `json:"name" bson:"name"`
	Email        string   `json:"email" bson:"email"`
	Phone        string   `json:"phone" bson:"phone"`
	VillageID    string   `json:"villageId" bson:"villageId"`
	Role         UserRole `json:"role" bson:"role"`
	Verified     bool     `json:"verified" bson:"verified"`
	PasswordHash string   `json:"-" bson:"passwordHash"`
}

// AuthSession stores revocation state for a short-lived JWT session.
type AuthSession struct {
	ID        string     `json:"-" bson:"_id"`
	UserID    string     `json:"userId" bson:"userId"`
	ExpiresAt time.Time  `json:"expiresAt" bson:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty" bson:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt" bson:"createdAt"`
}

// OrderStatus describes the lifecycle of a customer purchase.
type OrderStatus string

const (
	OrderPending        OrderStatus = "pending"
	OrderPlaced         OrderStatus = OrderPending
	OrderConfirmed      OrderStatus = "confirmed"
	OrderPreparing      OrderStatus = "preparing"
	OrderPacked         OrderStatus = "packed"
	OrderReadyForPickup OrderStatus = "ready_for_pickup"
	OrderOutForDelivery OrderStatus = "out_for_delivery"
	OrderDelivered      OrderStatus = "delivered"
	OrderCancelled      OrderStatus = "cancelled"
	OrderRejected       OrderStatus = "rejected"

	OrderOutForPickup OrderStatus = OrderReadyForPickup
	OrderInTransit    OrderStatus = OrderOutForDelivery
)

// Order stores the full customer order and its fulfillment state.
type Order struct {
	ID                string             `json:"id" bson:"_id"`
	CustomerID        string             `json:"customerId" bson:"customerId"`
	SellerID          string             `json:"sellerId" bson:"sellerId"`
	VillageID         string             `json:"villageId" bson:"villageId"`
	Items             []OrderItem        `json:"items" bson:"items"`
	Subtotal          float64            `json:"subtotal" bson:"subtotal"`
	DeliveryFee       float64            `json:"deliveryFee" bson:"deliveryFee"`
	Total             float64            `json:"total" bson:"total"`
	FulfillmentType   string             `json:"fulfillmentType" bson:"fulfillmentType"`
	Address           string             `json:"address,omitempty" bson:"address,omitempty"`
	Payment           OrderPayment       `json:"payment" bson:"payment"`
	PaymentID         string             `json:"paymentId" bson:"paymentId"`
	Status            OrderStatus        `json:"status" bson:"status"`
	StatusHistory     []OrderStatusEvent `json:"statusHistory" bson:"statusHistory"`
	InventoryReserved bool               `json:"-" bson:"inventoryReserved,omitempty"`
	InventoryRestored bool               `json:"-" bson:"inventoryRestored,omitempty"`
	CreatedAt         string             `json:"createdAt" bson:"createdAt"`
	UpdatedAt         string             `json:"updatedAt" bson:"updatedAt"`
}

type OrderPayment struct {
	Method string  `json:"method" bson:"method"`
	Status string  `json:"status" bson:"status"`
	Amount float64 `json:"amount" bson:"amount"`
}

type Payment struct {
	ID         string  `json:"id" bson:"_id"`
	OrderID    string  `json:"orderId" bson:"orderId"`
	CustomerID string  `json:"customerId" bson:"customerId"`
	Method     string  `json:"method" bson:"method"`
	Status     string  `json:"status" bson:"status"`
	Amount     float64 `json:"amount" bson:"amount"`
	CreatedAt  string  `json:"createdAt" bson:"createdAt"`
	UpdatedAt  string  `json:"updatedAt" bson:"updatedAt"`
}

type OrderStatusEvent struct {
	Status    OrderStatus `json:"status" bson:"status"`
	ActorID   string      `json:"actorId" bson:"actorId"`
	ActorRole UserRole    `json:"actorRole" bson:"actorRole"`
	Note      string      `json:"note,omitempty" bson:"note,omitempty"`
	CreatedAt string      `json:"createdAt" bson:"createdAt"`
}

// OrderItem models a single line item inside an order.
type OrderItem struct {
	ProductID string  `json:"productId" bson:"productId"`
	Name      string  `json:"name" bson:"name"`
	Unit      string  `json:"unit" bson:"unit"`
	SellerID  string  `json:"sellerId" bson:"sellerId"`
	Quantity  int     `json:"quantity" bson:"quantity"`
	Price     float64 `json:"price" bson:"price"`
	LineTotal float64 `json:"lineTotal" bson:"lineTotal"`
}

type Cart struct {
	ID         string     `json:"-" bson:"_id"`
	CustomerID string     `json:"customerId" bson:"customerId"`
	VillageID  string     `json:"villageId" bson:"villageId"`
	SellerID   string     `json:"sellerId,omitempty" bson:"sellerId,omitempty"`
	Items      []CartItem `json:"items" bson:"items"`
	UpdatedAt  time.Time  `json:"updatedAt" bson:"updatedAt"`
	Revision   string     `json:"-" bson:"revision"`
}

type CartItem struct {
	ProductID string `json:"productId" bson:"productId"`
	Quantity  int    `json:"quantity" bson:"quantity"`
}

type CartLine struct {
	ProductID string  `json:"productId"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Unit      string  `json:"unit"`
	Quantity  int     `json:"quantity"`
	Stock     int     `json:"stock"`
	Available bool    `json:"available"`
	SellerID  string  `json:"sellerId"`
	VillageID string  `json:"villageId"`
	Valid     bool    `json:"valid"`
	Issue     string  `json:"issue,omitempty"`
}

type CartView struct {
	VillageID       string     `json:"villageId"`
	Items           []CartLine `json:"items"`
	Subtotal        float64    `json:"subtotal"`
	DeliveryFee     float64    `json:"deliveryFee"`
	Total           float64    `json:"total"`
	FulfillmentType string     `json:"fulfillmentType"`
}

// DeliveryAssignment tracks the dispatch of an order to a local agent.
type DeliveryAssignment struct {
	ID         string `json:"id" bson:"_id"`
	OrderID    string `json:"orderId" bson:"orderId"`
	AgentID    string `json:"agentId" bson:"agentId"`
	Status     string `json:"status" bson:"status"`
	PickupCode string `json:"pickupCode" bson:"pickupCode"`
	CreatedAt  string `json:"createdAt" bson:"createdAt"`
	UpdatedAt  string `json:"updatedAt" bson:"updatedAt"`
}

var DefaultVillages = []Village{
	{ID: "v1", Name: "Kudlu", District: "Udupi", State: "Karnataka"},
	{ID: "v2", Name: "Anekal", District: "Bengaluru Rural", State: "Karnataka"},
	{ID: "v3", Name: "Nandigama", District: "Krishna", State: "Andhra Pradesh"},
}

var DefaultCategories = []Category{
	{ID: "c1", Name: "Vegetables"},
	{ID: "c2", Name: "Fruits"},
	{ID: "c3", Name: "Grains"},
	{ID: "c4", Name: "Dairy"},
}

var DefaultProducts = []Product{
	{ID: "p1", Name: "Fresh Tomato", Category: "Vegetables", Price: 38, Rating: 4.8, Unit: "kg", Village: "Kudlu", VillageID: "v1", SellerID: "u5", Seller: "Green Valley Farm", Available: true, Stock: 22, Description: "Locally grown, bright red tomatoes suited for daily cooking."},
	{ID: "p2", Name: "Village Mango", Category: "Fruits", Price: 72, Rating: 4.9, Unit: "kg", Village: "Anekal", VillageID: "v2", SellerID: "u6", Seller: "Sunrise Orchard", Available: true, Stock: 18, Description: "Sweet, seasonal mangoes from nearby orchard clusters."},
	{ID: "p3", Name: "Rice Paddy", Category: "Grains", Price: 44, Rating: 4.6, Unit: "kg", Village: "Nandigama", VillageID: "v3", SellerID: "u7", Seller: "Riverbank Mills", Available: true, Stock: 30, Description: "Traditional grain harvested from local paddy plots."},
	{ID: "p4", Name: "Farm Eggs", Category: "Dairy", Price: 12, Rating: 4.7, Unit: "dozen", Village: "Kudlu", VillageID: "v1", SellerID: "u8", Seller: "Happy Hen Co-op", Available: true, Stock: 40, Description: "Free-range eggs collected daily from village poultry units."},
	{ID: "p5", Name: "Cucumber", Category: "Vegetables", Price: 26, Rating: 4.5, Unit: "kg", Village: "Anekal", VillageID: "v2", SellerID: "u9", Seller: "Hill View Greens", Available: false, Stock: 0, Description: "Fresh, crunchy cucumbers ready for quick household meals."},
}

var DefaultUsers = []User{
	{ID: "u1", Name: "Demo Customer", Email: "customer@villageconnect.local", Phone: "9876543210", VillageID: "v1", Role: RoleCustomer, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u2", Name: "Demo Seller", Email: "seller@villageconnect.local", Phone: "9123456780", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u3", Name: "Demo Agent", Email: "agent@villageconnect.local", Phone: "9988776655", VillageID: "v1", Role: RoleDeliveryAgent, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u4", Name: "Demo Admin", Email: "admin@villageconnect.local", Phone: "9090909090", VillageID: "v1", Role: RoleAdmin, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u5", Name: "Green Valley Farm", Email: "green-valley@villageconnect.local", Phone: "9000000005", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u6", Name: "Sunrise Orchard", Email: "sunrise@villageconnect.local", Phone: "9000000006", VillageID: "v2", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u7", Name: "Riverbank Mills", Email: "riverbank@villageconnect.local", Phone: "9000000007", VillageID: "v3", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u8", Name: "Happy Hen Co-op", Email: "happy-hen@villageconnect.local", Phone: "9000000008", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u9", Name: "Hill View Greens", Email: "hill-view@villageconnect.local", Phone: "9000000009", VillageID: "v2", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
}

var DefaultOrders = []Order{
	{
		ID:         "o1",
		CustomerID: "u1",
		SellerID:   "u2",
		VillageID:  "v1",
		Items:      []OrderItem{{ProductID: "p1", Name: "Fresh Tomato", Unit: "kg", SellerID: "u2", Quantity: 2, Price: 38, LineTotal: 76}},
		Subtotal:   76,
		Total:      76,
		Status:     OrderPending,
		CreatedAt:  "2026-10-01T09:00:00Z",
		UpdatedAt:  "2026-10-01T09:00:00Z",
	},
	{
		ID:          "o2",
		CustomerID:  "u1",
		SellerID:    "u2",
		VillageID:   "v2",
		Items:       []OrderItem{{ProductID: "p2", Name: "Village Mango", Unit: "kg", SellerID: "u2", Quantity: 1, Price: 72, LineTotal: 72}},
		Subtotal:    72,
		DeliveryFee: 25,
		Total:       97,
		Status:      OrderOutForDelivery,
		CreatedAt:   "2026-10-02T12:30:00Z",
		UpdatedAt:   "2026-10-02T12:45:00Z",
	},
}

var DefaultDeliveries = []DeliveryAssignment{
	{ID: "d1", OrderID: "o2", AgentID: "u3", Status: "assigned", PickupCode: "VC-2048", CreatedAt: "2026-10-02T13:00:00Z", UpdatedAt: "2026-10-02T13:00:00Z"},
}
