package services

import (
	"context"
	"testing"

	"villageconnect/internal/models"
	"villageconnect/internal/repository"
)

type serviceTestRepository struct {
	repository.Repository
	users      map[string]models.User
	products   map[string]models.Product
	order      *models.Order
	cart       *models.Cart
	villages   []models.Village
	query      models.ProductQuery
	deliveries []models.DeliveryAssignment
}

func (r *serviceTestRepository) GetUserByEmailForAuth(_ context.Context, email string) (*models.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			copy := user
			return &copy, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *serviceTestRepository) GetUserByID(_ context.Context, id string) (*models.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := user
	return &copy, nil
}

func (r *serviceTestRepository) CreateUser(_ context.Context, user models.User) error {
	r.users[user.ID] = user
	return nil
}

func (r *serviceTestRepository) ListUsers(context.Context) ([]models.User, error) {
	users := make([]models.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, nil
}

func (r *serviceTestRepository) ListVillages(context.Context) ([]models.Village, error) {
	if r.villages != nil {
		return r.villages, nil
	}
	return []models.Village{{ID: "village-1", Name: "Kudlu"}}, nil
}
func (r *serviceTestRepository) ListMarketplaceProducts(_ context.Context, query models.ProductQuery) (models.ProductPage, error) {
	r.query = query
	return models.ProductPage{}, nil
}

func (r *serviceTestRepository) GetProductByID(_ context.Context, id string) (*models.Product, error) {
	product, ok := r.products[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := product
	return &copy, nil
}

func (r *serviceTestRepository) CreateOrder(_ context.Context, order models.Order) error {
	copy := order
	r.order = &copy
	return nil
}
func (r *serviceTestRepository) GetOrderByID(_ context.Context, id string) (*models.Order, error) {
	if r.order == nil || r.order.ID != id {
		return nil, repository.ErrNotFound
	}
	copy := *r.order
	return &copy, nil
}
func (r *serviceTestRepository) ListOrders(context.Context) ([]models.Order, error) {
	if r.order == nil {
		return nil, nil
	}
	return []models.Order{*r.order}, nil
}
func (r *serviceTestRepository) ListAgentDeliveries(_ context.Context, agentID string) ([]models.DeliveryAssignment, error) {
	result := make([]models.DeliveryAssignment, 0)
	for _, delivery := range r.deliveries {
		if delivery.AgentID == agentID {
			result = append(result, delivery)
		}
	}
	return result, nil
}
func (r *serviceTestRepository) GetCart(context.Context, string) (*models.Cart, error) {
	if r.cart == nil {
		return nil, repository.ErrNotFound
	}
	copy := *r.cart
	copy.Items = append([]models.CartItem(nil), r.cart.Items...)
	return &copy, nil
}
func (r *serviceTestRepository) SaveCart(_ context.Context, cart models.Cart, _ string) error {
	copy := cart
	r.cart = &copy
	return nil
}
func (r *serviceTestRepository) ClearCart(context.Context, string) error {
	r.cart = nil
	return nil
}
func (r *serviceTestRepository) CheckoutOrder(_ context.Context, order models.Order, _ models.Payment, _ string) error {
	copy := order
	r.order = &copy
	r.cart = nil
	return nil
}
func (r *serviceTestRepository) ListOrderPayments(context.Context) ([]models.Payment, error) {
	return nil, nil
}
func (r *serviceTestRepository) TransitionOrder(_ context.Context, id string, from, to models.OrderStatus, actorID string,
	event models.OrderStatusEvent) (*models.Order, error) {
	if r.order == nil || r.order.ID != id || r.order.Status != from {
		return nil, repository.ErrConflict
	}
	r.order.Status = to
	r.order.UpdatedAt = event.CreatedAt
	r.order.StatusHistory = append(r.order.StatusHistory, event)
	copy := *r.order
	return &copy, nil
}

func TestRegisterRejectsAdminRole(t *testing.T) {
	repo := &serviceTestRepository{users: map[string]models.User{}}
	service := NewUserService(repo)

	_, err := service.Register(context.Background(), "Root", "root@example.com", "", "password123", "", "admin")
	if err != ErrInvalidInput {
		t.Fatalf("Register error = %v, want %v", err, ErrInvalidInput)
	}
	if len(repo.users) != 0 {
		t.Fatal("registration created an account with an unsupported role")
	}
}

func TestRegisterNormalizesEmailAndHashesPassword(t *testing.T) {
	repo := &serviceTestRepository{users: map[string]models.User{}}
	service := NewUserService(repo)

	user, err := service.Register(context.Background(), "  Customer ", " PERSON@EXAMPLE.COM ", "", "password123", "v1", "")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if user.Email != "person@example.com" {
		t.Fatalf("email = %q, want normalized email", user.Email)
	}
	if user.PasswordHash == "" || user.PasswordHash == "password123" {
		t.Fatal("password was not stored as a hash")
	}
}

func TestCreateOrderUsesCatalogPricesAndValidatesStock(t *testing.T) {
	repo := &serviceTestRepository{
		users: map[string]models.User{
			"customer-1": {ID: "customer-1", Role: models.RoleCustomer},
			"seller-1":   {ID: "seller-1", Name: "Seller", Role: models.RoleSeller, Verified: true},
		},
		products: map[string]models.Product{
			"product-1": {ID: "product-1", Name: "Tomato", VillageID: "village-1", Village: "Kudlu",
				SellerID: "seller-1", Price: 38, Available: true, Stock: 5},
		},
		cart: &models.Cart{CustomerID: "customer-1", VillageID: "village-1", SellerID: "seller-1",
			Items: []models.CartItem{{ProductID: "product-1", Quantity: 2}}},
	}

	service := NewOrderService(repo)
	created, err := service.Checkout(context.Background(), "customer-1", "village-1",
		"delivery", "Kudlu Road", "simulated")
	if err != nil {
		t.Fatalf("Checkout returned error: %v", err)
	}
	if repo.order.Total != 101 || repo.order.Subtotal != 76 || repo.order.DeliveryFee != 25 ||
		repo.order.Items[0].Price != 38 || repo.order.Items[0].Name != "Tomato" ||
		created.SellerID != "seller-1" || created.Total != 101 || created.Payment.Status != "paid" {
		t.Fatalf("order was not calculated from catalog: %+v", repo.order)
	}

	repo.cart = &models.Cart{CustomerID: "customer-1", VillageID: "village-1", SellerID: "seller-1",
		Items: []models.CartItem{{ProductID: "product-1", Quantity: 6}}}
	if _, err := service.Checkout(context.Background(), "customer-1", "village-1",
		"delivery", "Kudlu Road", "simulated"); err != ErrConflict {
		t.Fatalf("Checkout error = %v, want %v for insufficient stock", err, ErrConflict)
	}
}

func TestCartTotalsAndCheckoutValidateVillageAndSeller(t *testing.T) {
	repo := &serviceTestRepository{
		users: map[string]models.User{
			"customer-1": {ID: "customer-1", Role: models.RoleCustomer},
			"seller-1":   {ID: "seller-1", Name: "Seller", Role: models.RoleSeller, Verified: true},
		},
		products: map[string]models.Product{
			"product-1": {ID: "product-1", Name: "Tomato", SellerID: "seller-1",
				VillageID: "village-1", Village: "Kudlu", Price: 38, Available: true, Stock: 5},
		},
		villages: []models.Village{{ID: "village-1", Name: "Kudlu"}, {ID: "other-village", Name: "Other"}},
		cart: &models.Cart{CustomerID: "customer-1", VillageID: "village-1", SellerID: "seller-1",
			Items: []models.CartItem{{ProductID: "product-1", Quantity: 2}}},
	}
	service := NewOrderService(repo)
	delivery, err := service.GetCart(context.Background(), "customer-1", "delivery")
	if err != nil {
		t.Fatal(err)
	}
	pickup, err := service.GetCart(context.Background(), "customer-1", "pickup")
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Subtotal != 76 || delivery.DeliveryFee != 25 || delivery.Total != 101 ||
		pickup.Subtotal != 76 || pickup.DeliveryFee != 0 || pickup.Total != 76 {
		t.Fatalf("server cart totals were not calculated correctly: delivery=%+v pickup=%+v", delivery, pickup)
	}
	if err := service.AddCartItem(context.Background(), "customer-1", "product-1", "other-village", 1); err != ErrConflict {
		t.Fatalf("adding a product from another village error = %v, want conflict", err)
	}
	if _, err := service.Checkout(context.Background(), "customer-1", "other-village",
		"delivery", "Address", "simulated"); err != ErrConflict {
		t.Fatalf("checkout to a different village error = %v, want conflict", err)
	}
	seller := repo.users["seller-1"]
	seller.Verified = false
	repo.users["seller-1"] = seller
	if _, err := service.Checkout(context.Background(), "customer-1", "village-1",
		"pickup", "", "cash_on_pickup"); err != ErrConflict {
		t.Fatalf("checkout with unverified seller error = %v, want conflict", err)
	}
	seller.Verified = true
	repo.users["seller-1"] = seller
	pickupOrder, err := service.Checkout(context.Background(), "customer-1", "village-1",
		"pickup", "", "cash_on_pickup")
	if err != nil {
		t.Fatalf("valid pickup checkout returned error: %v", err)
	}
	if pickupOrder.FulfillmentType != "pickup" || pickupOrder.DeliveryFee != 0 ||
		pickupOrder.Total != 76 || pickupOrder.Payment.Method != "cash_on_pickup" ||
		pickupOrder.Payment.Status != "pending" {
		t.Fatalf("pickup order details are incorrect: %+v", pickupOrder)
	}
}

func TestOrderStatusTransitionValidation(t *testing.T) {
	lifecycle := []models.OrderStatus{
		models.OrderPending, models.OrderConfirmed, models.OrderPreparing, models.OrderPacked,
		models.OrderReadyForPickup, models.OrderOutForDelivery, models.OrderDelivered,
	}
	for i := 0; i < len(lifecycle)-1; i++ {
		if !validOrderTransition(lifecycle[i], lifecycle[i+1]) {
			t.Fatalf("lifecycle transition %s -> %s should be allowed", lifecycle[i], lifecycle[i+1])
		}
	}
	for _, invalid := range [][2]models.OrderStatus{
		{models.OrderPending, models.OrderPacked},
		{models.OrderConfirmed, models.OrderReadyForPickup},
		{models.OrderDelivered, models.OrderCancelled},
		{models.OrderRejected, models.OrderConfirmed},
		{models.OrderCancelled, models.OrderPending},
	} {
		if validOrderTransition(invalid[0], invalid[1]) {
			t.Fatalf("invalid lifecycle transition %s -> %s was accepted", invalid[0], invalid[1])
		}
	}
}

func TestSellerAndDeliveryAgentCompleteOrderLifecycleWithOwnership(t *testing.T) {
	repo := &serviceTestRepository{
		users: map[string]models.User{
			"seller-1": {ID: "seller-1", Role: models.RoleSeller, Verified: true},
			"seller-2": {ID: "seller-2", Role: models.RoleSeller, Verified: true},
			"agent-1":  {ID: "agent-1", Role: models.RoleDeliveryAgent, Verified: true},
		},
		order: &models.Order{ID: "order-1", SellerID: "seller-1", CustomerID: "customer-1",
			Status: models.OrderPending, FulfillmentType: "delivery", InventoryReserved: true},
		deliveries: []models.DeliveryAssignment{{OrderID: "order-1", AgentID: "agent-1"}},
	}
	service := NewOrderService(repo)
	for _, status := range []models.OrderStatus{
		models.OrderConfirmed, models.OrderPreparing, models.OrderPacked, models.OrderReadyForPickup,
	} {
		if _, err := service.UpdateStatus(context.Background(), "order-1", status, "seller-2", string(models.RoleSeller)); err != ErrForbidden {
			t.Fatalf("other seller transition to %s returned %v, want forbidden", status, err)
		}
		if _, err := service.UpdateStatus(context.Background(), "order-1", status, "seller-1", string(models.RoleSeller)); err != nil {
			t.Fatalf("seller transition to %s returned error: %v", status, err)
		}
	}
	if _, err := service.UpdateStatus(context.Background(), "order-1", models.OrderOutForDelivery, "agent-2", string(models.RoleDeliveryAgent)); err != ErrForbidden {
		t.Fatalf("unassigned delivery agent error = %v, want forbidden", err)
	}
	for _, status := range []models.OrderStatus{models.OrderOutForDelivery, models.OrderDelivered} {
		if _, err := service.UpdateStatus(context.Background(), "order-1", status, "agent-1", string(models.RoleDeliveryAgent)); err != nil {
			t.Fatalf("assigned agent transition to %s returned error: %v", status, err)
		}
	}
	if repo.order.Status != models.OrderDelivered || len(repo.order.StatusHistory) != 6 {
		t.Fatalf("order did not record the full state history: %+v", repo.order)
	}
	if _, err := service.UpdateStatus(context.Background(), "order-1", models.OrderCancelled, "customer-1", string(models.RoleCustomer)); err != ErrConflict {
		t.Fatalf("delivered order cancellation error = %v, want conflict", err)
	}
}

func TestMarketplaceQueryDoesNotUseAmbiguousLegacyVillageNames(t *testing.T) {
	repo := &serviceTestRepository{
		villages: []models.Village{
			{ID: "village-1", Name: "Springfield"},
			{ID: "village-2", Name: "Springfield"},
		},
	}
	service := NewCatalogService(repo)
	_, err := service.Products(context.Background(), models.ProductQuery{
		VillageID: "village-1", Page: 1, PageSize: 8,
	})
	if err != nil {
		t.Fatalf("Products returned error: %v", err)
	}
	if repo.query.VillageID != "village-1" || repo.query.VillageName != "" {
		t.Fatalf("ambiguous village name was used for legacy fallback: %+v", repo.query)
	}
}
