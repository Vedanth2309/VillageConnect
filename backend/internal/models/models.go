package models

import "time"

// Village represents a mapped village community within the marketplace.
type Village struct {
	ID       string `json:"id" bson:"_id"`
	Name     string `json:"name" bson:"name"`
	District string `json:"district" bson:"district"`
	Taluk    string `json:"taluk" bson:"taluk"`
	State    string `json:"state" bson:"state"`
	Active   *bool  `json:"active,omitempty" bson:"active,omitempty"`
}

// Category represents the product grouping used in marketplace discovery.
type Category struct {
	ID     string `json:"id" bson:"_id"`
	Name   string `json:"name" bson:"name"`
	Active *bool  `json:"active,omitempty" bson:"active,omitempty"`
}

func (v Village) IsActive() bool  { return v.Active == nil || *v.Active }
func (c Category) IsActive() bool { return c.Active == nil || *c.Active }

// Product represents a catalog item sold by a local seller.
type Product struct {
	ID                string          `json:"id" bson:"_id"`
	Name              string          `json:"name" bson:"name"`
	Category          string          `json:"category" bson:"category"`
	Price             float64         `json:"price" bson:"price"`
	Rating            float64         `json:"rating" bson:"rating"`
	RatingCount       int             `json:"ratingCount" bson:"ratingCount,omitempty"`
	Unit              string          `json:"unit" bson:"unit"`
	Village           string          `json:"village" bson:"village"`
	VillageID         string          `json:"villageId,omitempty" bson:"villageId,omitempty"`
	SellerID          string          `json:"sellerId,omitempty" bson:"sellerId,omitempty"`
	Seller            string          `json:"seller" bson:"seller"`
	Available         bool            `json:"available" bson:"available"`
	Stock             int             `json:"stock" bson:"stock"`
	Description       string          `json:"description" bson:"description"`
	History           []ProductChange `json:"-" bson:"history,omitempty"`
	RestockedOrderIDs []string        `json:"-" bson:"restockedOrderIds,omitempty"`
	ExpectedStock     *int            `json:"-" bson:"-"`
	Change            *ProductChange  `json:"-" bson:"-"`
}

type ProductChange struct {
	ActorID       string   `json:"actorId" bson:"actorId"`
	ChangedAt     string   `json:"changedAt" bson:"changedAt"`
	PreviousPrice *float64 `json:"previousPrice,omitempty" bson:"previousPrice,omitempty"`
	NewPrice      *float64 `json:"newPrice,omitempty" bson:"newPrice,omitempty"`
	PreviousStock *int     `json:"previousStock,omitempty" bson:"previousStock,omitempty"`
	NewStock      *int     `json:"newStock,omitempty" bson:"newStock,omitempty"`
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
	ID             string   `json:"id" bson:"_id"`
	Name           string   `json:"name" bson:"name"`
	Email          string   `json:"email" bson:"email"`
	Phone          string   `json:"phone" bson:"phone"`
	VillageID      string   `json:"villageId" bson:"villageId"`
	Role           UserRole `json:"role" bson:"role"`
	Verified       bool     `json:"verified" bson:"verified"`
	Active         *bool    `json:"active,omitempty" bson:"active,omitempty"`
	SellerStatus   string   `json:"sellerStatus,omitempty" bson:"sellerStatus,omitempty"`
	Rating         float64  `json:"rating,omitempty" bson:"rating,omitempty"`
	RatingCount    int      `json:"ratingCount,omitempty" bson:"ratingCount,omitempty"`
	AgentOnline    bool     `json:"agentOnline" bson:"agentOnline,omitempty"`
	AgentAvailable bool     `json:"agentAvailable" bson:"agentAvailable,omitempty"`
	PasswordHash   string   `json:"-" bson:"passwordHash"`
}

func (u User) IsActive() bool { return u.Active == nil || *u.Active }

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

type Review struct {
	ID            string `json:"id" bson:"_id"`
	OrderID       string `json:"orderId" bson:"orderId"`
	ProductID     string `json:"productId" bson:"productId"`
	SellerID      string `json:"sellerId" bson:"sellerId"`
	CustomerID    string `json:"-" bson:"customerId"`
	ProductRating int    `json:"productRating" bson:"productRating"`
	SellerRating  int    `json:"sellerRating" bson:"sellerRating"`
	Comment       string `json:"comment,omitempty" bson:"comment,omitempty"`
	CreatedAt     string `json:"createdAt" bson:"createdAt"`
}

type Notification struct {
	ID        string    `json:"id" bson:"_id"`
	UserID    string    `json:"userId" bson:"userId"`
	Type      string    `json:"type" bson:"type"`
	Title     string    `json:"title" bson:"title"`
	Message   string    `json:"message" bson:"message"`
	EntityID  string    `json:"entityId,omitempty" bson:"entityId,omitempty"`
	Read      bool      `json:"read" bson:"read"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
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
	ID                  string                `json:"id" bson:"_id"`
	OrderID             string                `json:"orderId" bson:"orderId"`
	AgentID             string                `json:"agentId" bson:"agentId"`
	Status              string                `json:"status" bson:"status"`
	PickupCode          string                `json:"-" bson:"pickupCode"`
	CreatedAt           string                `json:"createdAt" bson:"createdAt"`
	UpdatedAt           string                `json:"updatedAt" bson:"updatedAt"`
	AcceptedAt          string                `json:"acceptedAt,omitempty" bson:"acceptedAt,omitempty"`
	PickedUpAt          string                `json:"pickedUpAt,omitempty" bson:"pickedUpAt,omitempty"`
	InTransitAt         string                `json:"inTransitAt,omitempty" bson:"inTransitAt,omitempty"`
	DeliveredAt         string                `json:"deliveredAt,omitempty" bson:"deliveredAt,omitempty"`
	FailedAt            string                `json:"failedAt,omitempty" bson:"failedAt,omitempty"`
	CancelledAt         string                `json:"cancelledAt,omitempty" bson:"cancelledAt,omitempty"`
	CustomerConfirmedAt string                `json:"customerConfirmedAt,omitempty" bson:"customerConfirmedAt,omitempty"`
	History             []DeliveryStatusEvent `json:"history" bson:"history,omitempty"`
	Order               *DeliveryOrderSummary `json:"order,omitempty" bson:"-"`
}

const (
	DeliveryAssigned  = "assigned"
	DeliveryAccepted  = "accepted"
	DeliveryPickedUp  = "picked_up"
	DeliveryInTransit = "in_transit"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
	DeliveryCancelled = "cancelled"
)

type DeliveryStatusEvent struct {
	Status    string   `json:"status" bson:"status"`
	ActorID   string   `json:"actorId" bson:"actorId"`
	ActorRole UserRole `json:"actorRole" bson:"actorRole"`
	Note      string   `json:"note,omitempty" bson:"note,omitempty"`
	CreatedAt string   `json:"createdAt" bson:"createdAt"`
}

type DeliveryOrderSummary struct {
	ID              string      `json:"id"`
	VillageID       string      `json:"villageId"`
	Items           []OrderItem `json:"items"`
	Address         string      `json:"address,omitempty"`
	FulfillmentType string      `json:"fulfillmentType"`
	Status          OrderStatus `json:"status"`
}

var DefaultVillages = []Village{
	{ID: "v1", Name: "Kudlu", District: "Udupi", Taluk: "Udupi", State: "Karnataka"},
	{ID: "v2", Name: "Anekal", District: "Bengaluru Rural", Taluk: "Anekal", State: "Karnataka"},
	{ID: "v3", Name: "Nandigama", District: "Krishna", Taluk: "Nandigama", State: "Andhra Pradesh"},
	{ID: "v4", Name: "Kumbakonam", District: "Thanjavur", Taluk: "Kumbakonam", State: "Tamil Nadu"},
	{ID: "v5", Name: "Pollachi", District: "Coimbatore", Taluk: "Pollachi", State: "Tamil Nadu"},
	{ID: "v6", Name: "Kottayam", District: "Kottayam", Taluk: "Kottayam", State: "Kerala"},
	{ID: "v7", Name: "Mysuru Rural", District: "Mysuru", Taluk: "Mysuru", State: "Karnataka"},
	{ID: "v8", Name: "Nashik Rural", District: "Nashik", Taluk: "Nashik", State: "Maharashtra"},
}
var DefaultCategories = []Category{
	{ID: "c1", Name: "Vegetables"},
	{ID: "c2", Name: "Fruits"},
	{ID: "c3", Name: "Grains"},
	{ID: "c4", Name: "Dairy"},
	{ID: "c5", Name: "Spices"},
	{ID: "c6", Name: "Pickles"},
	{ID: "c7", Name: "Oils"},
	{ID: "c8", Name: "Traditional Foods"},
}
var DefaultProducts = []Product{
	// Kudlu
	{ID: "p1", Name: "Fresh Tomato", Category: "Vegetables", Price: 38, Rating: 4.8, RatingCount: 24, Unit: "kg", Village: "Kudlu", VillageID: "v1", SellerID: "u5", Seller: "Green Valley Farm", Available: true, Stock: 22, Description: "Locally grown, bright red tomatoes suited for daily cooking."},
	{ID: "p2", Name: "Fresh Spinach", Category: "Vegetables", Price: 30, Rating: 4.6, RatingCount: 15, Unit: "bunch", Village: "Kudlu", VillageID: "v1", SellerID: "u5", Seller: "Green Valley Farm", Available: true, Stock: 35, Description: "Fresh leafy spinach harvested in the morning."},
	{ID: "p3", Name: "Red Onions", Category: "Vegetables", Price: 42, Rating: 4.5, RatingCount: 18, Unit: "kg", Village: "Kudlu", VillageID: "v1", SellerID: "u10", Seller: "Kudlu Fresh Farms", Available: true, Stock: 50, Description: "Locally grown red onions with strong flavor."},
	{ID: "p4", Name: "Farm Milk", Category: "Dairy", Price: 58, Rating: 4.9, RatingCount: 31, Unit: "litre", Village: "Kudlu", VillageID: "v1", SellerID: "u11", Seller: "Kudlu Dairy Co-op", Available: true, Stock: 18, Description: "Fresh farm milk collected from local dairy farmers."},

	// Anekal
	{ID: "p5", Name: "Village Mango", Category: "Fruits", Price: 72, Rating: 4.9, RatingCount: 42, Unit: "kg", Village: "Anekal", VillageID: "v2", SellerID: "u6", Seller: "Sunrise Orchard", Available: true, Stock: 18, Description: "Sweet seasonal mangoes from nearby orchard clusters."},
	{ID: "p6", Name: "Banana", Category: "Fruits", Price: 48, Rating: 4.7, RatingCount: 21, Unit: "dozen", Village: "Anekal", VillageID: "v2", SellerID: "u6", Seller: "Sunrise Orchard", Available: true, Stock: 25, Description: "Naturally ripened village bananas."},
	{ID: "p7", Name: "Cucumber", Category: "Vegetables", Price: 26, Rating: 4.5, RatingCount: 17, Unit: "kg", Village: "Anekal", VillageID: "v2", SellerID: "u9", Seller: "Hill View Greens", Available: false, Stock: 0, Description: "Fresh crunchy cucumbers for household meals."},
	{ID: "p8", Name: "Green Beans", Category: "Vegetables", Price: 55, Rating: 4.4, RatingCount: 12, Unit: "kg", Village: "Anekal", VillageID: "v2", SellerID: "u9", Seller: "Hill View Greens", Available: true, Stock: 9, Description: "Freshly harvested green beans."},

	// Nandigama
	{ID: "p9", Name: "Rice Paddy", Category: "Grains", Price: 44, Rating: 4.6, RatingCount: 28, Unit: "kg", Village: "Nandigama", VillageID: "v3", SellerID: "u7", Seller: "Riverbank Mills", Available: true, Stock: 30, Description: "Traditional grain harvested from local paddy plots."},
	{ID: "p10", Name: "Brown Rice", Category: "Grains", Price: 68, Rating: 4.8, RatingCount: 19, Unit: "kg", Village: "Nandigama", VillageID: "v3", SellerID: "u7", Seller: "Riverbank Mills", Available: true, Stock: 16, Description: "Naturally processed brown rice from local farms."},
	{ID: "p11", Name: "Turmeric Powder", Category: "Spices", Price: 160, Rating: 4.9, RatingCount: 33, Unit: "kg", Village: "Nandigama", VillageID: "v3", SellerID: "u12", Seller: "Golden Spice Farm", Available: true, Stock: 7, Description: "Village-grown turmeric processed into fine powder."},
	{ID: "p12", Name: "Red Chilli Powder", Category: "Spices", Price: 180, Rating: 4.7, RatingCount: 16, Unit: "kg", Village: "Nandigama", VillageID: "v3", SellerID: "u12", Seller: "Golden Spice Farm", Available: true, Stock: 11, Description: "Sun-dried red chillies ground locally."},

	// Kumbakonam
	{ID: "p13", Name: "Groundnut Oil", Category: "Oils", Price: 185, Rating: 4.8, RatingCount: 25, Unit: "litre", Village: "Kumbakonam", VillageID: "v4", SellerID: "u13", Seller: "Kaveri Oils", Available: true, Stock: 14, Description: "Cold-pressed groundnut oil made locally."},
	{ID: "p14", Name: "Traditional Rice", Category: "Grains", Price: 95, Rating: 4.9, RatingCount: 36, Unit: "kg", Village: "Kumbakonam", VillageID: "v4", SellerID: "u13", Seller: "Kaveri Oils", Available: true, Stock: 20, Description: "Traditional rice variety sourced from local farmers."},
	{ID: "p15", Name: "Mango Pickle", Category: "Pickles", Price: 140, Rating: 4.7, RatingCount: 22, Unit: "jar", Village: "Kumbakonam", VillageID: "v4", SellerID: "u14", Seller: "Village Taste Foods", Available: true, Stock: 12, Description: "Homemade spicy mango pickle."},

	// Pollachi
	{ID: "p16", Name: "Tender Coconut", Category: "Fruits", Price: 45, Rating: 4.8, RatingCount: 29, Unit: "piece", Village: "Pollachi", VillageID: "v5", SellerID: "u15", Seller: "Coconut Valley Farms", Available: true, Stock: 45, Description: "Fresh tender coconuts harvested locally."},
	{ID: "p17", Name: "Coconut Oil", Category: "Oils", Price: 210, Rating: 4.9, RatingCount: 27, Unit: "litre", Village: "Pollachi", VillageID: "v5", SellerID: "u15", Seller: "Coconut Valley Farms", Available: true, Stock: 11, Description: "Traditional locally processed coconut oil."},

	// Kottayam
	{ID: "p18", Name: "Kerala Banana", Category: "Fruits", Price: 65, Rating: 4.7, RatingCount: 20, Unit: "kg", Village: "Kottayam", VillageID: "v6", SellerID: "u16", Seller: "Green Kerala Farms", Available: true, Stock: 23, Description: "Fresh traditional Kerala bananas."},
	{ID: "p19", Name: "Homemade Pickle", Category: "Pickles", Price: 120, Rating: 4.6, RatingCount: 14, Unit: "jar", Village: "Kottayam", VillageID: "v6", SellerID: "u17", Seller: "Grandma's Kitchen", Available: true, Stock: 6, Description: "Traditional homemade village pickle."},

	// Mysuru
	{ID: "p20", Name: "Ragi Flour", Category: "Traditional Foods", Price: 75, Rating: 4.8, RatingCount: 23, Unit: "kg", Village: "Mysuru Rural", VillageID: "v7", SellerID: "u21", Seller: "Mysuru Millet Farm", Available: true, Stock: 17, Description: "Stone-ground ragi flour from local millet farms."},
	{ID: "p21", Name: "Jaggery", Category: "Traditional Foods", Price: 90, Rating: 4.9, RatingCount: 30, Unit: "kg", Village: "Mysuru Rural", VillageID: "v7", SellerID: "u21", Seller: "Mysuru Millet Farm", Available: true, Stock: 24, Description: "Natural jaggery prepared by local producers."},

	// Nashik
	{ID: "p22", Name: "Green Grapes", Category: "Fruits", Price: 110, Rating: 4.8, RatingCount: 26, Unit: "kg", Village: "Nashik Rural", VillageID: "v8", SellerID: "u22", Seller: "Nashik Fresh Orchards", Available: true, Stock: 28, Description: "Fresh locally grown table grapes."},
	{ID: "p23", Name: "Onion Pickle", Category: "Pickles", Price: 130, Rating: 4.5, RatingCount: 11, Unit: "jar", Village: "Nashik Rural", VillageID: "v8", SellerID: "u23", Seller: "Nashik Village Foods", Available: true, Stock: 8, Description: "Homestyle onion pickle prepared in small batches."},
}

var DefaultUsers = []User{
	{ID: "u1", Name: "Demo Customer", Email: "customer@villageconnect.local", Phone: "9876543210", VillageID: "v1", Role: RoleCustomer, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u2", Name: "Demo Seller", Email: "seller@villageconnect.local", Phone: "9123456780", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u3", Name: "Demo Agent", Email: "agent@villageconnect.local", Phone: "9988776655", VillageID: "v1", Role: RoleDeliveryAgent, Verified: true, AgentOnline: true, AgentAvailable: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u4", Name: "Demo Admin", Email: "admin@villageconnect.local", Phone: "9090909090", VillageID: "v1", Role: RoleAdmin, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u5", Name: "Green Valley Farm", Email: "green-valley@villageconnect.local", Phone: "9000000005", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u6", Name: "Sunrise Orchard", Email: "sunrise@villageconnect.local", Phone: "9000000006", VillageID: "v2", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u7", Name: "Riverbank Mills", Email: "riverbank@villageconnect.local", Phone: "9000000007", VillageID: "v3", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u8", Name: "Happy Hen Co-op", Email: "happy-hen@villageconnect.local", Phone: "9000000008", VillageID: "v1", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
	{ID: "u9", Name: "Hill View Greens", Email: "hill-view@villageconnect.local", Phone: "9000000009", VillageID: "v2", Role: RoleSeller, Verified: true, PasswordHash: "$2a$10$N/SX32qMeRglNnVB05ezX.9Ktq4.yUcvC/o7OMIFFIbKgAhfQs1Wi"},
}

var DefaultOrders = []Order{
	{
		ID: "o1", CustomerID: "u1", SellerID: "u5", VillageID: "v1",
		Items:    []OrderItem{{ProductID: "p1", Name: "Fresh Tomato", Unit: "kg", SellerID: "u5", Quantity: 2, Price: 38, LineTotal: 76}},
		Subtotal: 76, DeliveryFee: 0, Total: 76, FulfillmentType: "pickup", Address: "Kudlu Village Market",
		Payment: OrderPayment{Method: "cash", Status: "pending", Amount: 76}, Status: OrderPending,
		StatusHistory: []OrderStatusEvent{{Status: OrderPending, ActorID: "u1", ActorRole: RoleCustomer, Note: "Order placed", CreatedAt: "2026-10-01T09:00:00Z"}},
		CreatedAt:     "2026-10-01T09:00:00Z", UpdatedAt: "2026-10-01T09:00:00Z",
	},
	{
		ID: "o2", CustomerID: "u1", SellerID: "u6", VillageID: "v2",
		Items:    []OrderItem{{ProductID: "p5", Name: "Village Mango", Unit: "kg", SellerID: "u6", Quantity: 1, Price: 72, LineTotal: 72}},
		Subtotal: 72, DeliveryFee: 25, Total: 97, FulfillmentType: "delivery", Address: "Anekal Main Road, House 18",
		Payment: OrderPayment{Method: "upi", Status: "paid", Amount: 97}, PaymentID: "pay2", Status: OrderOutForDelivery,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u1", ActorRole: RoleCustomer, Note: "Order placed", CreatedAt: "2026-10-02T12:30:00Z"},
			{Status: OrderConfirmed, ActorID: "u6", ActorRole: RoleSeller, Note: "Seller confirmed", CreatedAt: "2026-10-02T12:35:00Z"},
			{Status: OrderPreparing, ActorID: "u6", ActorRole: RoleSeller, Note: "Preparing mangoes", CreatedAt: "2026-10-02T12:40:00Z"},
			{Status: OrderOutForDelivery, ActorID: "u3", ActorRole: RoleDeliveryAgent, Note: "Out for delivery", CreatedAt: "2026-10-02T12:45:00Z"},
		},
		CreatedAt: "2026-10-02T12:30:00Z", UpdatedAt: "2026-10-02T12:45:00Z",
	},
	{
		ID: "o3", CustomerID: "u18", SellerID: "u13", VillageID: "v4",
		Items: []OrderItem{
			{ProductID: "p13", Name: "Groundnut Oil", Unit: "litre", SellerID: "u13", Quantity: 2, Price: 185, LineTotal: 370},
			{ProductID: "p14", Name: "Traditional Rice", Unit: "kg", SellerID: "u13", Quantity: 2, Price: 95, LineTotal: 190},
		},
		Subtotal: 560, DeliveryFee: 30, Total: 590, FulfillmentType: "delivery", Address: "Kumbakonam East Street, House 42",
		Payment: OrderPayment{Method: "upi", Status: "paid", Amount: 590}, PaymentID: "pay3", Status: OrderDelivered,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u18", ActorRole: RoleCustomer, CreatedAt: "2026-10-03T08:10:00Z"},
			{Status: OrderConfirmed, ActorID: "u13", ActorRole: RoleSeller, CreatedAt: "2026-10-03T08:20:00Z"},
			{Status: OrderPreparing, ActorID: "u13", ActorRole: RoleSeller, CreatedAt: "2026-10-03T09:00:00Z"},
			{Status: OrderPacked, ActorID: "u13", ActorRole: RoleSeller, CreatedAt: "2026-10-03T09:30:00Z"},
			{Status: OrderDelivered, ActorID: "u20", ActorRole: RoleDeliveryAgent, CreatedAt: "2026-10-03T12:15:00Z"},
		},
		CreatedAt: "2026-10-03T08:10:00Z", UpdatedAt: "2026-10-03T12:15:00Z",
	},
	{
		ID: "o4", CustomerID: "u19", SellerID: "u15", VillageID: "v5",
		Items:    []OrderItem{{ProductID: "p16", Name: "Tender Coconut", Unit: "piece", SellerID: "u15", Quantity: 4, Price: 45, LineTotal: 180}},
		Subtotal: 180, DeliveryFee: 20, Total: 200, FulfillmentType: "delivery", Address: "Pollachi Bus Stand Road",
		Payment: OrderPayment{Method: "cash", Status: "pending", Amount: 200}, Status: OrderConfirmed,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u19", ActorRole: RoleCustomer, CreatedAt: "2026-10-04T10:00:00Z"},
			{Status: OrderConfirmed, ActorID: "u15", ActorRole: RoleSeller, CreatedAt: "2026-10-04T10:15:00Z"},
		},
		CreatedAt: "2026-10-04T10:00:00Z", UpdatedAt: "2026-10-04T10:15:00Z",
	},
	{
		ID: "o5", CustomerID: "u1", SellerID: "u5", VillageID: "v1",
		Items: []OrderItem{
			{ProductID: "p2", Name: "Fresh Spinach", Unit: "bunch", SellerID: "u5", Quantity: 3, Price: 30, LineTotal: 90},
			{ProductID: "p2", Name: "Fresh Spinach", Unit: "bunch", SellerID: "u5", Quantity: 2, Price: 30, LineTotal: 60},
		},
		Subtotal: 150, DeliveryFee: 25, Total: 175, FulfillmentType: "delivery", Address: "Kudlu Lake Road",
		Payment: OrderPayment{Method: "upi", Status: "paid", Amount: 175}, PaymentID: "pay5", Status: OrderPreparing,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u1", ActorRole: RoleCustomer, CreatedAt: "2026-10-04T15:00:00Z"},
			{Status: OrderConfirmed, ActorID: "u5", ActorRole: RoleSeller, CreatedAt: "2026-10-04T15:10:00Z"},
			{Status: OrderPreparing, ActorID: "u5", ActorRole: RoleSeller, CreatedAt: "2026-10-04T15:30:00Z"},
		},
		CreatedAt: "2026-10-04T15:00:00Z", UpdatedAt: "2026-10-04T15:30:00Z",
	},
	{
		ID: "o6", CustomerID: "u18", SellerID: "u12", VillageID: "v3",
		Items:    []OrderItem{{ProductID: "p11", Name: "Turmeric Powder", Unit: "kg", SellerID: "u12", Quantity: 1, Price: 160, LineTotal: 160}},
		Subtotal: 160, DeliveryFee: 20, Total: 180, FulfillmentType: "delivery", Address: "Nandigama Market Road",
		Payment: OrderPayment{Method: "card", Status: "paid", Amount: 180}, PaymentID: "pay6", Status: OrderDelivered,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u18", ActorRole: RoleCustomer, CreatedAt: "2026-10-01T11:00:00Z"},
			{Status: OrderConfirmed, ActorID: "u12", ActorRole: RoleSeller, CreatedAt: "2026-10-01T11:10:00Z"},
			{Status: OrderDelivered, ActorID: "u3", ActorRole: RoleDeliveryAgent, CreatedAt: "2026-10-01T14:00:00Z"},
		},
		CreatedAt: "2026-10-01T11:00:00Z", UpdatedAt: "2026-10-01T14:00:00Z",
	},
	{
		ID: "o7", CustomerID: "u19", SellerID: "u16", VillageID: "v6",
		Items:    []OrderItem{{ProductID: "p18", Name: "Kerala Banana", Unit: "kg", SellerID: "u16", Quantity: 2, Price: 65, LineTotal: 130}},
		Subtotal: 130, DeliveryFee: 15, Total: 145, FulfillmentType: "delivery", Address: "Kottayam Central Road",
		Payment: OrderPayment{Method: "upi", Status: "paid", Amount: 145}, PaymentID: "pay7", Status: OrderCancelled,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u19", ActorRole: RoleCustomer, CreatedAt: "2026-10-02T16:00:00Z"},
			{Status: OrderCancelled, ActorID: "u19", ActorRole: RoleCustomer, Note: "Customer cancelled before preparation", CreatedAt: "2026-10-02T16:20:00Z"},
		},
		CreatedAt: "2026-10-02T16:00:00Z", UpdatedAt: "2026-10-02T16:20:00Z",
	},
	{
		ID: "o8", CustomerID: "u1", SellerID: "u21", VillageID: "v7",
		Items:    []OrderItem{{ProductID: "p20", Name: "Ragi Flour", Unit: "kg", SellerID: "u21", Quantity: 2, Price: 75, LineTotal: 150}},
		Subtotal: 150, DeliveryFee: 20, Total: 170, FulfillmentType: "pickup", Address: "Mysuru Rural Collection Point",
		Payment: OrderPayment{Method: "cash", Status: "paid", Amount: 170}, PaymentID: "pay8", Status: OrderReadyForPickup,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u1", ActorRole: RoleCustomer, CreatedAt: "2026-10-03T17:00:00Z"},
			{Status: OrderConfirmed, ActorID: "u21", ActorRole: RoleSeller, CreatedAt: "2026-10-03T17:10:00Z"},
			{Status: OrderReadyForPickup, ActorID: "u21", ActorRole: RoleSeller, CreatedAt: "2026-10-03T18:00:00Z"},
		},
		CreatedAt: "2026-10-03T17:00:00Z", UpdatedAt: "2026-10-03T18:00:00Z",
	},
	{
		ID: "o9", CustomerID: "u18", SellerID: "u22", VillageID: "v8",
		Items:    []OrderItem{{ProductID: "p22", Name: "Green Grapes", Unit: "kg", SellerID: "u22", Quantity: 3, Price: 110, LineTotal: 330}},
		Subtotal: 330, DeliveryFee: 25, Total: 355, FulfillmentType: "delivery", Address: "Nashik Rural Main Road",
		Payment: OrderPayment{Method: "upi", Status: "paid", Amount: 355}, PaymentID: "pay9", Status: OrderPacked,
		StatusHistory: []OrderStatusEvent{
			{Status: OrderPending, ActorID: "u18", ActorRole: RoleCustomer, CreatedAt: "2026-10-04T08:00:00Z"},
			{Status: OrderConfirmed, ActorID: "u22", ActorRole: RoleSeller, CreatedAt: "2026-10-04T08:15:00Z"},
			{Status: OrderPreparing, ActorID: "u22", ActorRole: RoleSeller, CreatedAt: "2026-10-04T09:00:00Z"},
			{Status: OrderPacked, ActorID: "u22", ActorRole: RoleSeller, CreatedAt: "2026-10-04T10:00:00Z"},
		},
		CreatedAt: "2026-10-04T08:00:00Z", UpdatedAt: "2026-10-04T10:00:00Z",
	},
	{
		ID: "o10", CustomerID: "u19", SellerID: "u14", VillageID: "v4",
		Items:    []OrderItem{{ProductID: "p15", Name: "Mango Pickle", Unit: "jar", SellerID: "u14", Quantity: 2, Price: 140, LineTotal: 280}},
		Subtotal: 280, DeliveryFee: 0, Total: 280, FulfillmentType: "pickup", Address: "Kumbakonam Village Market",
		Payment: OrderPayment{Method: "cash", Status: "pending", Amount: 280}, Status: OrderPending,
		StatusHistory: []OrderStatusEvent{{Status: OrderPending, ActorID: "u19", ActorRole: RoleCustomer, Note: "Pickup order placed", CreatedAt: "2026-10-05T09:00:00Z"}},
		CreatedAt:     "2026-10-05T09:00:00Z", UpdatedAt: "2026-10-05T09:00:00Z",
	},
}

var DefaultDeliveries = []DeliveryAssignment{
	{ID: "d1", OrderID: "o2", AgentID: "u3", Status: "assigned", PickupCode: "VC-2048", CreatedAt: "2026-10-02T13:00:00Z", UpdatedAt: "2026-10-02T13:00:00Z"},
}
