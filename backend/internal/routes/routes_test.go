package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"villageconnect/internal/auth"
	"villageconnect/internal/handlers"
	"villageconnect/internal/models"
	"villageconnect/internal/repository"
	"villageconnect/internal/search"
	"villageconnect/internal/services"
)

const testJWTSecret = "test-secret-that-is-long-enough-for-auth-tests"

type routeTestRepository struct {
	repository.Repository
	users         map[string]models.User
	villages      []models.Village
	categories    []models.Category
	sessions      map[string]models.AuthSession
	products      map[string]models.Product
	orders        map[string]models.Order
	reviews       []models.Review
	notifications []models.Notification
	payments      []models.Payment
	deliveries    map[string]models.DeliveryAssignment
	carts         map[string]models.Cart
}

func newRouteTestRepository() *routeTestRepository {
	users := map[string]models.User{
		"customer-1": {ID: "customer-1", Name: "Customer One", Email: "customer@example.com", Role: models.RoleCustomer, Verified: true},
		"customer-2": {ID: "customer-2", Name: "Customer Two", Email: "customer2@example.com", Role: models.RoleCustomer, Verified: true},
		"seller-1":   {ID: "seller-1", Name: "Seller One", Email: "seller@example.com", Role: models.RoleSeller, Verified: true},
		"seller-2":   {ID: "seller-2", Name: "Seller Two", Email: "seller2@example.com", Role: models.RoleSeller, Verified: true},
		"agent-1":    {ID: "agent-1", Name: "Agent One", Email: "agent@example.com", Role: models.RoleDeliveryAgent, Verified: true},
		"agent-2":    {ID: "agent-2", Name: "Agent Two", Email: "agent2@example.com", Role: models.RoleDeliveryAgent, Verified: true},
		"admin-1":    {ID: "admin-1", Name: "Admin", Email: "admin@example.com", Role: models.RoleAdmin, Verified: true},
	}
	products := map[string]models.Product{
		"product-1": {ID: "product-1", Name: "Tomato", Category: "Vegetables", Price: 38, Rating: 4, Unit: "kg", Village: "Kudlu", VillageID: "v1", SellerID: "seller-1", Seller: "Seller One", Available: true, Stock: 10, Description: "Fresh tomatoes"},
		"product-2": {ID: "product-2", Name: "Mango", Category: "Fruits", Price: 72, Rating: 4, Unit: "kg", Village: "Kudlu", VillageID: "v1", SellerID: "seller-2", Seller: "Seller Two", Available: true, Stock: 10, Description: "Fresh mangoes"},
	}
	return &routeTestRepository{
		users: users, sessions: map[string]models.AuthSession{}, products: products,
		villages:   []models.Village{{ID: "v1", Name: "Kudlu", District: "Udupi", State: "Karnataka"}},
		categories: []models.Category{{ID: "c1", Name: "Vegetables"}},
		carts:      map[string]models.Cart{},
		orders: map[string]models.Order{
			"order-1": {ID: "order-1", CustomerID: "customer-1", SellerID: "seller-1", VillageID: "v1", Status: models.OrderPlaced},
			"order-2": {ID: "order-2", CustomerID: "customer-2", SellerID: "seller-1", VillageID: "v1", Status: models.OrderPlaced},
			"order-3": {ID: "order-3", CustomerID: "customer-2", SellerID: "seller-2", VillageID: "v1", Status: models.OrderPlaced},
		},
		deliveries: map[string]models.DeliveryAssignment{
			"delivery-1": {ID: "delivery-1", OrderID: "order-1", AgentID: "agent-1", Status: "assigned"},
			"delivery-2": {ID: "delivery-2", OrderID: "order-2", AgentID: "agent-2", Status: "assigned"},
		},
	}
}

func (r *routeTestRepository) ListVillages(context.Context) ([]models.Village, error) {
	return append([]models.Village(nil), r.villages...), nil
}
func (r *routeTestRepository) ListCategories(context.Context) ([]models.Category, error) {
	return append([]models.Category(nil), r.categories...), nil
}

func (r *routeTestRepository) CreateVillage(_ context.Context, village models.Village) error {
	for _, existing := range r.villages {
		if existing.ID == village.ID {
			return repository.ErrConflict
		}
	}
	r.villages = append(r.villages, village)
	return nil
}
func (r *routeTestRepository) UpdateVillage(_ context.Context, village models.Village) error {
	for i := range r.villages {
		if r.villages[i].ID == village.ID {
			r.villages[i] = village
			return nil
		}
	}
	return repository.ErrNotFound
}
func (r *routeTestRepository) SetVillageActive(_ context.Context, id string, active bool) error {
	for i := range r.villages {
		if r.villages[i].ID == id {
			r.villages[i].Active = &active
			return nil
		}
	}
	return repository.ErrNotFound
}
func (r *routeTestRepository) CreateCategory(_ context.Context, category models.Category) error {
	for _, existing := range r.categories {
		if existing.ID == category.ID {
			return repository.ErrConflict
		}
	}
	r.categories = append(r.categories, category)
	return nil
}
func (r *routeTestRepository) UpdateCategory(_ context.Context, category models.Category) error {
	for i := range r.categories {
		if r.categories[i].ID == category.ID {
			r.categories[i] = category
			return nil
		}
	}
	return repository.ErrNotFound
}
func (r *routeTestRepository) SetCategoryActive(_ context.Context, id string, active bool) error {
	for i := range r.categories {
		if r.categories[i].ID == id {
			r.categories[i].Active = &active
			return nil
		}
	}
	return repository.ErrNotFound
}
func (r *routeTestRepository) ListProducts(context.Context) ([]models.Product, error) {
	result := make([]models.Product, 0, len(r.products))
	for _, product := range r.products {
		result = append(result, product)
	}
	return result, nil
}
func (r *routeTestRepository) ListMarketplaceProducts(_ context.Context, query models.ProductQuery) (models.ProductPage, error) {
	products, _ := r.ListProducts(context.Background())
	filtered := make([]models.Product, 0, len(products))
	for _, product := range products {
		if query.VillageID != "" && product.VillageID != query.VillageID {
			continue
		}
		if query.Category != "" && product.Category != query.Category {
			continue
		}
		if query.MinPrice != nil && product.Price < *query.MinPrice {
			continue
		}
		if query.MaxPrice != nil && product.Price > *query.MaxPrice {
			continue
		}
		if query.Available != nil && *query.Available != (product.Available && product.Stock > 0) {
			continue
		}
		if query.Query != "" && !strings.Contains(strings.ToLower(product.Name), strings.ToLower(query.Query)) &&
			!strings.Contains(strings.ToLower(product.Description), strings.ToLower(query.Query)) {
			continue
		}
		filtered = append(filtered, product)
	}
	total := len(filtered)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return models.ProductPage{Products: filtered[start:end], Total: int64(total), Page: query.Page,
		PageSize: query.PageSize, TotalPages: totalPages}, nil
}
func (r *routeTestRepository) GetProductByID(_ context.Context, id string) (*models.Product, error) {
	product, ok := r.products[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := product
	return &copy, nil
}
func (r *routeTestRepository) SearchProducts(ctx context.Context, _ string) ([]models.Product, error) {
	return r.ListProducts(ctx)
}
func (r *routeTestRepository) ListUsers(context.Context) ([]models.User, error) {
	result := make([]models.User, 0, len(r.users))
	for _, user := range r.users {
		user.PasswordHash = ""
		result = append(result, user)
	}
	return result, nil
}
func (r *routeTestRepository) GetUserByEmailForAuth(_ context.Context, email string) (*models.User, error) {
	for _, user := range r.users {
		if strings.EqualFold(user.Email, email) {
			copy := user
			return &copy, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (r *routeTestRepository) GetUserByID(_ context.Context, id string) (*models.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	user.PasswordHash = ""
	return &user, nil
}
func (r *routeTestRepository) CreateUser(_ context.Context, user models.User) error {
	r.users[user.ID] = user
	return nil
}
func (r *routeTestRepository) SetUserActive(_ context.Context, id string, active bool) error {
	user, exists := r.users[id]
	if !exists {
		return repository.ErrNotFound
	}
	user.Active = &active
	r.users[id] = user
	return nil
}
func (r *routeTestRepository) SetSellerState(_ context.Context, id, status string, verified, active bool) error {
	user, exists := r.users[id]
	if !exists || user.Role != models.RoleSeller {
		return repository.ErrNotFound
	}
	user.SellerStatus = status
	user.Verified = verified
	user.Active = &active
	r.users[id] = user
	return nil
}
func (r *routeTestRepository) CreateSession(_ context.Context, session models.AuthSession) error {
	if _, exists := r.sessions[session.ID]; exists {
		return repository.ErrConflict
	}
	r.sessions[session.ID] = session
	return nil
}
func (r *routeTestRepository) GetSession(_ context.Context, id string) (*models.AuthSession, error) {
	session, ok := r.sessions[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &session, nil
}
func (r *routeTestRepository) RevokeSession(_ context.Context, id string, when time.Time) error {
	session, ok := r.sessions[id]
	if !ok {
		return repository.ErrNotFound
	}
	if session.RevokedAt == nil {
		session.RevokedAt = &when
		r.sessions[id] = session
	}
	return nil
}
func (r *routeTestRepository) ListOrders(context.Context) ([]models.Order, error) {
	result := make([]models.Order, 0, len(r.orders))
	for _, order := range r.orders {
		result = append(result, order)
	}
	return result, nil
}
func (r *routeTestRepository) GetOrderByID(_ context.Context, id string) (*models.Order, error) {
	order, ok := r.orders[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &order, nil
}
func (r *routeTestRepository) CreateOrder(_ context.Context, order models.Order) error {
	r.orders[order.ID] = order
	return nil
}
func (r *routeTestRepository) GetCart(_ context.Context, customerID string) (*models.Cart, error) {
	cart, ok := r.carts[customerID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	copy := cart
	copy.Items = append([]models.CartItem(nil), cart.Items...)
	return &copy, nil
}
func (r *routeTestRepository) SaveCart(_ context.Context, cart models.Cart, expectedRevision string) error {
	current, exists := r.carts[cart.CustomerID]
	if expectedRevision == "" && exists || expectedRevision != "" && (!exists || current.Revision != expectedRevision) {
		return repository.ErrConflict
	}
	cart.Revision = time.Now().UTC().Format(time.RFC3339Nano)
	r.carts[cart.CustomerID] = cart
	return nil
}
func (r *routeTestRepository) ClearCart(_ context.Context, customerID string) error {
	delete(r.carts, customerID)
	return nil
}
func (r *routeTestRepository) CheckoutOrder(_ context.Context, order models.Order, payment models.Payment, expectedCartRevision string) error {
	if r.carts[order.CustomerID].Revision != expectedCartRevision {
		return repository.ErrConflict
	}
	for _, item := range order.Items {
		product := r.products[item.ProductID]
		if !product.Available || product.Stock < item.Quantity {
			return repository.ErrConflict
		}
	}
	for _, item := range order.Items {
		product := r.products[item.ProductID]
		product.Stock -= item.Quantity
		r.products[item.ProductID] = product
	}
	r.orders[order.ID] = order
	r.payments = append(r.payments, payment)
	delete(r.carts, order.CustomerID)
	return nil
}
func (r *routeTestRepository) ListOrderPayments(context.Context) ([]models.Payment, error) {
	return append([]models.Payment(nil), r.payments...), nil
}
func (r *routeTestRepository) TransitionOrder(_ context.Context, id string, from, to models.OrderStatus, _ string,
	event models.OrderStatusEvent) (*models.Order, error) {
	order, ok := r.orders[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if order.Status != from {
		return nil, repository.ErrConflict
	}
	order.Status = to
	order.StatusHistory = append(order.StatusHistory, event)
	r.orders[id] = order
	return &order, nil
}
func (r *routeTestRepository) ListDeliveries(context.Context) ([]models.DeliveryAssignment, error) {
	result := make([]models.DeliveryAssignment, 0, len(r.deliveries))
	for _, delivery := range r.deliveries {
		result = append(result, delivery)
	}
	return result, nil
}
func (r *routeTestRepository) GetDeliveryByID(_ context.Context, id string) (*models.DeliveryAssignment, error) {
	delivery, ok := r.deliveries[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &delivery, nil
}
func (r *routeTestRepository) ListAgentDeliveries(ctx context.Context, agentID string) ([]models.DeliveryAssignment, error) {
	all, _ := r.ListDeliveries(ctx)
	result := make([]models.DeliveryAssignment, 0)
	for _, delivery := range all {
		if delivery.AgentID == agentID {
			result = append(result, delivery)
		}
	}
	return result, nil
}
func (r *routeTestRepository) CreateDelivery(_ context.Context, delivery models.DeliveryAssignment) error {
	r.deliveries[delivery.ID] = delivery
	return nil
}
func (r *routeTestRepository) UpdateDeliveryStatus(_ context.Context, id, fromStatus, status string) (*models.DeliveryAssignment, error) {
	delivery, ok := r.deliveries[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if delivery.Status != fromStatus {
		return nil, repository.ErrConflict
	}
	delivery.Status = status
	r.deliveries[id] = delivery
	return &delivery, nil
}
func (r *routeTestRepository) CreateProduct(_ context.Context, product models.Product) error {
	if _, exists := r.products[product.ID]; exists {
		return repository.ErrConflict
	}
	r.products[product.ID] = product
	return nil
}
func (r *routeTestRepository) UpdateProduct(_ context.Context, product models.Product) error {
	if _, exists := r.products[product.ID]; !exists {
		return repository.ErrNotFound
	}
	r.products[product.ID] = product
	return nil
}
func (r *routeTestRepository) SetProductAvailability(_ context.Context, id string, available bool) error {
	product, exists := r.products[id]
	if !exists {
		return repository.ErrNotFound
	}
	product.Available = available && product.Stock > 0
	r.products[id] = product
	return nil
}
func (r *routeTestRepository) DeleteProduct(_ context.Context, id string) error {
	if _, exists := r.products[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.products, id)
	return nil
}
func (r *routeTestRepository) ListSellerProducts(ctx context.Context, sellerID string) ([]models.Product, error) {
	all, _ := r.ListProducts(ctx)
	result := make([]models.Product, 0)
	for _, product := range all {
		if product.SellerID == sellerID {
			result = append(result, product)
		}
	}
	return result, nil
}
func (r *routeTestRepository) GetAnalytics(context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"users": len(r.users)}, nil
}

func (r *routeTestRepository) ListReviewsByOrder(_ context.Context, orderID string) ([]models.Review, error) {
	result := make([]models.Review, 0)
	for _, review := range r.reviews {
		if review.OrderID == orderID {
			result = append(result, review)
		}
	}
	return result, nil
}

func (r *routeTestRepository) ListReviewsByProduct(_ context.Context, productID string) ([]models.Review, error) {
	result := make([]models.Review, 0)
	for _, review := range r.reviews {
		if review.ProductID == productID {
			result = append(result, review)
		}
	}
	return result, nil
}

func (r *routeTestRepository) ListReviewsBySeller(_ context.Context, sellerID string) ([]models.Review, error) {
	result := make([]models.Review, 0)
	for _, review := range r.reviews {
		if review.SellerID == sellerID {
			result = append(result, review)
		}
	}
	return result, nil
}

func (r *routeTestRepository) CreateReview(_ context.Context, review models.Review) error {
	for _, existing := range r.reviews {
		if existing.OrderID == review.OrderID && existing.ProductID == review.ProductID {
			return repository.ErrConflict
		}
	}
	r.reviews = append(r.reviews, review)
	productTotal, productCount := 0, 0
	sellerTotal, sellerCount := 0, 0
	for _, current := range r.reviews {
		if current.ProductID == review.ProductID {
			productTotal += current.ProductRating
			productCount++
		}
		if current.SellerID == review.SellerID {
			sellerTotal += current.SellerRating
			sellerCount++
		}
	}
	product := r.products[review.ProductID]
	product.Rating = float64(productTotal) / float64(productCount)
	product.RatingCount = productCount
	r.products[review.ProductID] = product
	seller := r.users[review.SellerID]
	seller.Rating = float64(sellerTotal) / float64(sellerCount)
	seller.RatingCount = sellerCount
	r.users[review.SellerID] = seller
	return nil
}

func (r *routeTestRepository) CreateNotification(_ context.Context, notification models.Notification) error {
	r.notifications = append(r.notifications, notification)
	return nil
}

func (r *routeTestRepository) ListNotifications(_ context.Context, userID string, limit int) ([]models.Notification, error) {
	result := make([]models.Notification, 0)
	for i := len(r.notifications) - 1; i >= 0 && len(result) < limit; i-- {
		if r.notifications[i].UserID == userID {
			result = append(result, r.notifications[i])
		}
	}
	return result, nil
}

func (r *routeTestRepository) CountUnreadNotifications(_ context.Context, userID string) (int64, error) {
	var count int64
	for _, notification := range r.notifications {
		if notification.UserID == userID && !notification.Read {
			count++
		}
	}
	return count, nil
}

func (r *routeTestRepository) MarkNotificationRead(_ context.Context, userID, notificationID string) error {
	for i := range r.notifications {
		if r.notifications[i].ID == notificationID && r.notifications[i].UserID == userID {
			r.notifications[i].Read = true
			return nil
		}
	}
	return repository.ErrNotFound
}

func newTestRouter(t *testing.T) (*gin.Engine, *routeTestRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newRouteTestRepository()
	userService := services.NewUserService(repo)
	catalogService := services.NewCatalogService(repo)
	orderService := services.NewOrderService(repo)
	deliveryService := services.NewDeliveryService(repo)
	reviewService := services.NewReviewService(repo)
	notificationService := services.NewNotificationService(repo)
	adminService := services.NewAdminService(repo)
	searchService := search.NewService(repo, "", "")
	api := handlers.NewAPI(userService, catalogService, orderService, deliveryService,
		reviewService, notificationService, adminService, searchService, testJWTSecret)
	return New(api, func() string { return "test" }, "http://localhost"), repo
}

func tokenFor(t *testing.T, repo *routeTestRepository, user models.User) string {
	t.Helper()
	token, claims, err := auth.GenerateSessionToken(testJWTSecret, user, services.SessionLifetime)
	if err != nil {
		t.Fatalf("generate token for %s: %v", user.ID, err)
	}
	repo.sessions[claims.ID] = models.AuthSession{
		ID: claims.ID, UserID: user.ID, ExpiresAt: claims.ExpiresAt.Time, CreatedAt: time.Now().UTC(),
	}
	return token
}

func request(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestPublicAndAuthenticatedRouteClassification(t *testing.T) {
	router, repo := newTestRouter(t)
	for _, path := range []string{"/api/health", "/api/villages", "/api/categories", "/api/products", "/api/products/product-1", "/api/search/products?q=tomato", "/api/search/sellers?q=seller", "/api/sellers", "/api/reviews?productId=product-1", "/api/reviews/products/product-1", "/api/reviews/sellers/seller-1"} {
		response := request(router, http.MethodGet, path, "", "")
		if response.Code != http.StatusOK && response.Code != http.StatusNotImplemented {
			t.Errorf("public GET %s status = %d, body %s", path, response.Code, response.Body.String())
		}
	}
	for _, route := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/auth/me", ""},
		{http.MethodPost, "/api/auth/logout", ""},
		{http.MethodGet, "/api/users/me", ""},
		{http.MethodGet, "/api/orders", ""},
		{http.MethodGet, "/api/orders/order-1/reviews", ""},
		{http.MethodPost, "/api/orders", `{"villageId":"v1","items":[{"productId":"product-1","quantity":1}]}`},
		{http.MethodGet, "/api/deliveries", ""},
		{http.MethodGet, "/api/agents/agent-1/deliveries", ""},
		{http.MethodGet, "/api/sellers/seller-1/products", ""},
		{http.MethodGet, "/api/admin/users", ""},
		{http.MethodPost, "/api/admin/search/reindex", ""},
		{http.MethodGet, "/api/cart", ""},
		{http.MethodPost, "/api/cart/items", `{"productId":"product-1","villageId":"v1","quantity":1}`},
		{http.MethodPost, "/api/cart/checkout", `{"villageId":"v1","fulfillmentType":"delivery","address":"Kudlu","paymentMethod":"simulated"}`},
		{http.MethodPost, "/api/payments", `{}`},
		{http.MethodPost, "/api/reviews", `{}`},
	} {
		response := request(router, route.method, route.path, "", route.body)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s %s status = %d, want 401", route.method, route.path, response.Code)
		}
	}
	_ = repo
}

func TestMarketplaceFiltersPaginationAndSellerPrivacy(t *testing.T) {
	router, _ := newTestRouter(t)
	response := request(router, http.MethodGet,
		"/api/products?villageId=v1&category=Vegetables&maxPrice=50&available=true&page=1&pageSize=1",
		"", "")
	if response.Code != http.StatusOK {
		t.Fatalf("marketplace status = %d, body %s", response.Code, response.Body.String())
	}
	var page struct {
		Products   []models.Product `json:"products"`
		Pagination struct {
			Total      int64 `json:"total"`
			Page       int   `json:"page"`
			PageSize   int   `json:"pageSize"`
			TotalPages int   `json:"totalPages"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Products) != 1 || page.Products[0].ID != "product-1" ||
		page.Pagination.Total != 1 || page.Pagination.PageSize != 1 {
		t.Fatalf("marketplace result did not apply location/category/price/availability/pagination: %+v", page)
	}
	if invalid := request(router, http.MethodGet, "/api/products?pageSize=101", "", ""); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid page size status = %d, want 400", invalid.Code)
	}

	seller := request(router, http.MethodGet, "/api/sellers/seller-1", "", "")
	if seller.Code != http.StatusOK {
		t.Fatalf("seller details status = %d, body %s", seller.Code, seller.Body.String())
	}
	if strings.Contains(seller.Body.String(), "seller@example.com") || strings.Contains(seller.Body.String(), "passwordHash") {
		t.Fatalf("seller details exposed private account fields: %s", seller.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/reviews", "", ""); response.Code != http.StatusBadRequest {
		t.Errorf("unfiltered public reviews status = %d, want 400", response.Code)
	}
}

func TestRegistrationLoginLogoutAndSessionValidation(t *testing.T) {
	router, repo := newTestRouter(t)
	register := request(router, http.MethodPost, "/api/auth/register", "",
		`{"name":"New Customer","email":"new@example.com","password":"secure-pass-123","role":"customer"}`)
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", register.Code, register.Body.String())
	}
	var registered struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}
	if err := json.Unmarshal(register.Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	if registered.User.Role != models.RoleCustomer || registered.User.PasswordHash != "" {
		t.Fatalf("unsafe public registration response: %+v", registered.User)
	}
	me := request(router, http.MethodGet, "/api/auth/me", registered.Token, "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d body %s", me.Code, me.Body.String())
	}
	login := request(router, http.MethodPost, "/api/auth/login", "",
		`{"email":" NEW@example.com ","password":"secure-pass-123"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body %s", login.Code, login.Body.String())
	}
	for _, credentials := range []string{
		`{"email":"new@example.com","password":"wrong-password"}`,
		`{"email":"missing@example.com","password":"wrong-password"}`,
	} {
		failedLogin := request(router, http.MethodPost, "/api/auth/login", "", credentials)
		if failedLogin.Code != http.StatusUnauthorized {
			t.Errorf("invalid login returned %d, want 401; body %s", failedLogin.Code, failedLogin.Body.String())
		}
	}
	for _, role := range []string{"admin", "seller", "delivery_agent"} {
		response := request(router, http.MethodPost, "/api/auth/register", "",
			`{"name":"Escalation","email":"`+role+`@example.com","password":"secure-pass-123","role":"`+role+`"}`)
		if response.Code != http.StatusBadRequest {
			t.Errorf("self-registration as %s returned %d, want 400", role, response.Code)
		}
	}
	logout := request(router, http.MethodPost, "/api/auth/logout", registered.Token, "")
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d body %s", logout.Code, logout.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/auth/me", registered.Token, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d, want 401", response.Code)
	}
	if len(repo.sessions) < 2 {
		t.Fatal("registration/login sessions were not persisted")
	}
}

func TestAdminManagementRoutesRequireAdminAndPersistActions(t *testing.T) {
	router, repo := newTestRouter(t)
	admin := tokenFor(t, repo, repo.users["admin-1"])
	customer := tokenFor(t, repo, repo.users["customer-1"])
	seller := tokenFor(t, repo, repo.users["seller-1"])
	agent := tokenFor(t, repo, repo.users["agent-1"])
	protected := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/admin/users", ""},
		{http.MethodPatch, "/api/admin/users/customer-1/active", `{"active":false}`},
		{http.MethodGet, "/api/admin/sellers", ""},
		{http.MethodPatch, "/api/admin/sellers/seller-1/status", `{"action":"approve"}`},
		{http.MethodGet, "/api/admin/agents", ""},
		{http.MethodPatch, "/api/admin/agents/agent-1/active", `{"active":false}`},
		{http.MethodGet, "/api/admin/villages", ""},
		{http.MethodPost, "/api/admin/villages", `{"name":"Hilltop","district":"Udupi","state":"Karnataka"}`},
		{http.MethodGet, "/api/admin/categories", ""},
		{http.MethodPost, "/api/admin/categories", `{"name":"Spices"}`},
		{http.MethodGet, "/api/admin/products", ""},
		{http.MethodPatch, "/api/admin/products/product-1/availability", `{"available":false}`},
		{http.MethodGet, "/api/admin/orders", ""},
		{http.MethodGet, "/api/admin/analytics", ""},
	}
	for _, route := range protected {
		for _, token := range []string{customer, seller, agent} {
			if response := request(router, route.method, route.path, token, route.body); response.Code != http.StatusForbidden {
				t.Errorf("non-admin %s %s status = %d, want 403", route.method, route.path, response.Code)
			}
		}
	}
	if response := request(router, http.MethodGet, "/api/admin/orders", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin request status = %d, want 401", response.Code)
	}

	if response := request(router, http.MethodPatch, "/api/admin/sellers/seller-1/status", admin,
		`{"action":"reject"}`); response.Code != http.StatusOK {
		t.Fatalf("reject seller status = %d body %s", response.Code, response.Body.String())
	}
	if user := repo.users["seller-1"]; user.SellerStatus != "rejected" || user.Verified || user.IsActive() {
		t.Fatalf("seller reject was not persisted: %+v", user)
	}
	if repo.products["product-1"].Available {
		t.Fatal("rejecting a seller did not deactivate their product listing")
	}
	if response := request(router, http.MethodPatch, "/api/admin/sellers/seller-1/status", admin,
		`{"action":"verify"}`); response.Code != http.StatusOK {
		t.Fatalf("verify seller status = %d body %s", response.Code, response.Body.String())
	}
	if user := repo.users["seller-1"]; user.SellerStatus != "approved" || !user.Verified || !user.IsActive() {
		t.Fatalf("seller verification was not persisted: %+v", user)
	}
	if response := request(router, http.MethodPatch, "/api/admin/agents/agent-1/active", admin,
		`{"active":false}`); response.Code != http.StatusOK {
		t.Fatalf("deactivate agent status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/auth/me", agent, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("deactivated agent session status = %d, want 401", response.Code)
	}

	villageResponse := request(router, http.MethodPost, "/api/admin/villages", admin,
		`{"name":"Hilltop","district":"Udupi","taluk":"Karkala","state":"Karnataka"}`)
	if villageResponse.Code != http.StatusCreated {
		t.Fatalf("create village status = %d body %s", villageResponse.Code, villageResponse.Body.String())
	}
	var createdVillage struct {
		Village models.Village `json:"village"`
	}
	if err := json.Unmarshal(villageResponse.Body.Bytes(), &createdVillage); err != nil {
		t.Fatal(err)
	}
	if response := request(router, http.MethodPut, "/api/admin/villages/"+createdVillage.Village.ID, admin,
		`{"name":"Hilltop North","district":"Udupi","taluk":"Karkala","state":"Karnataka"}`); response.Code != http.StatusOK {
		t.Fatalf("edit village status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodPatch, "/api/admin/villages/"+createdVillage.Village.ID+"/active", admin,
		`{"active":false}`); response.Code != http.StatusOK {
		t.Fatalf("deactivate village status = %d body %s", response.Code, response.Body.String())
	}

	categoryResponse := request(router, http.MethodPost, "/api/admin/categories", admin, `{"name":"Spices"}`)
	if categoryResponse.Code != http.StatusCreated {
		t.Fatalf("create category status = %d body %s", categoryResponse.Code, categoryResponse.Body.String())
	}
	var createdCategory struct {
		Category models.Category `json:"category"`
	}
	if err := json.Unmarshal(categoryResponse.Body.Bytes(), &createdCategory); err != nil {
		t.Fatal(err)
	}
	if response := request(router, http.MethodPut, "/api/admin/categories/"+createdCategory.Category.ID, admin,
		`{"name":"Herbs"}`); response.Code != http.StatusOK {
		t.Fatalf("edit category status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodPatch, "/api/admin/categories/"+createdCategory.Category.ID+"/active", admin,
		`{"active":false}`); response.Code != http.StatusOK {
		t.Fatalf("deactivate category status = %d body %s", response.Code, response.Body.String())
	}

	if response := request(router, http.MethodPatch, "/api/admin/products/product-1/availability", admin,
		`{"available":false}`); response.Code != http.StatusOK || repo.products["product-1"].Available {
		t.Fatalf("product moderation status=%d available=%t body=%s", response.Code, repo.products["product-1"].Available, response.Body.String())
	}
	for _, path := range []string{"/api/admin/users", "/api/admin/sellers", "/api/admin/agents", "/api/admin/villages", "/api/admin/categories", "/api/admin/products", "/api/admin/orders", "/api/admin/analytics"} {
		if response := request(router, http.MethodGet, path, admin, ""); response.Code != http.StatusOK {
			t.Errorf("admin GET %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	if response := request(router, http.MethodPatch, "/api/admin/users/customer-1/active", admin,
		`{"active":false}`); response.Code != http.StatusOK {
		t.Fatalf("deactivate user status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/auth/me", customer, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("deactivated customer session status = %d, want 401", response.Code)
	}
}

func TestNotificationsAreRoleEventsAndUserScoped(t *testing.T) {
	router, repo := newTestRouter(t)
	admin := tokenFor(t, repo, repo.users["admin-1"])
	customer := tokenFor(t, repo, repo.users["customer-1"])
	unverifiedSeller := repo.users["seller-2"]
	unverifiedSeller.Verified = false
	unverifiedSeller.SellerStatus = "rejected"
	repo.users[unverifiedSeller.ID] = unverifiedSeller
	seller := tokenFor(t, repo, unverifiedSeller)

	if response := request(router, http.MethodPost, "/api/seller/verification-request", seller, ""); response.Code != http.StatusOK {
		t.Fatalf("verification request status = %d body %s", response.Code, response.Body.String())
	}
	if user := repo.users["seller-2"]; user.SellerStatus != "pending" || user.Verified {
		t.Fatalf("seller verification request state = %+v", user)
	}
	if response := request(router, http.MethodGet, "/api/notifications/unread-count", admin, ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"unread":1`) {
		t.Fatalf("admin unread count status=%d body=%s", response.Code, response.Body.String())
	}
	adminNotifications := request(router, http.MethodGet, "/api/notifications", admin, "")
	if adminNotifications.Code != http.StatusOK || !strings.Contains(adminNotifications.Body.String(), "seller_verification") {
		t.Fatalf("admin notification list status=%d body=%s", adminNotifications.Code, adminNotifications.Body.String())
	}
	notificationID := repo.notifications[0].ID
	if response := request(router, http.MethodPatch, "/api/notifications/"+notificationID+"/read", customer, ""); response.Code != http.StatusNotFound {
		t.Fatalf("cross-user mark-read status = %d, want 404", response.Code)
	}
	if response := request(router, http.MethodPatch, "/api/notifications/"+notificationID+"/read", admin, ""); response.Code != http.StatusOK {
		t.Fatalf("mark-read status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/notifications/unread-count", admin, ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"unread":0`) {
		t.Fatalf("read notification count status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/notifications", customer, ""); response.Code != http.StatusOK ||
		strings.Contains(response.Body.String(), notificationID) {
		t.Fatalf("customer saw another role's notification status=%d body=%s", response.Code, response.Body.String())
	}

	order := repo.orders["order-1"]
	order.Status = models.OrderReadyForPickup
	order.FulfillmentType = "delivery"
	repo.orders[order.ID] = order
	deliveryResponse := request(router, http.MethodPost, "/api/deliveries", admin,
		`{"orderId":"order-1","agentId":"agent-1"}`)
	if deliveryResponse.Code != http.StatusCreated {
		t.Fatalf("create delivery status = %d body %s", deliveryResponse.Code, deliveryResponse.Body.String())
	}
	var createdDelivery struct {
		Delivery models.DeliveryAssignment `json:"delivery"`
	}
	if err := json.Unmarshal(deliveryResponse.Body.Bytes(), &createdDelivery); err != nil {
		t.Fatal(err)
	}
	if response := request(router, http.MethodGet, "/api/notifications", tokenFor(t, repo, repo.users["agent-1"]), ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "delivery_assignment") {
		t.Fatalf("agent assignment notification status=%d body=%s", response.Code, response.Body.String())
	}
	agentToken := tokenFor(t, repo, repo.users["agent-1"])
	if response := request(router, http.MethodPatch, "/api/deliveries/"+createdDelivery.Delivery.ID+"/status", agentToken,
		`{"status":"accepted"}`); response.Code != http.StatusOK {
		t.Fatalf("agent accept delivery status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/notifications", customer, ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "delivery_update") {
		t.Fatalf("customer delivery update notification status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestFourRoleMarketplaceDeliveryAndReviewWorkflow(t *testing.T) {
	router, repo := newTestRouter(t)
	customer := tokenFor(t, repo, repo.users["customer-1"])
	seller := tokenFor(t, repo, repo.users["seller-1"])
	agent := tokenFor(t, repo, repo.users["agent-1"])
	admin := tokenFor(t, repo, repo.users["admin-1"])

	if response := request(router, http.MethodPost, "/api/cart/items", customer,
		`{"productId":"product-1","villageId":"v1","quantity":1}`); response.Code != http.StatusOK {
		t.Fatalf("customer add-to-cart status=%d body=%s", response.Code, response.Body.String())
	}
	checkout := request(router, http.MethodPost, "/api/cart/checkout", customer,
		`{"villageId":"v1","fulfillmentType":"delivery","address":"Kudlu Road","paymentMethod":"simulated"}`)
	if checkout.Code != http.StatusCreated {
		t.Fatalf("customer checkout status=%d body=%s", checkout.Code, checkout.Body.String())
	}
	var checkoutResult struct {
		Order models.Order `json:"order"`
	}
	if err := json.Unmarshal(checkout.Body.Bytes(), &checkoutResult); err != nil {
		t.Fatal(err)
	}
	orderID := checkoutResult.Order.ID
	for _, status := range []string{"confirmed", "preparing", "packed", "ready_for_pickup"} {
		response := request(router, http.MethodPatch, "/api/seller/orders/"+orderID+"/status", seller,
			`{"status":"`+status+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("seller transition %s status=%d body=%s", status, response.Code, response.Body.String())
		}
	}
	deliveryResponse := request(router, http.MethodPost, "/api/deliveries", admin,
		`{"orderId":"`+orderID+`","agentId":"agent-1"}`)
	if deliveryResponse.Code != http.StatusCreated {
		t.Fatalf("admin delivery assignment status=%d body=%s", deliveryResponse.Code, deliveryResponse.Body.String())
	}
	var assignmentResult struct {
		Delivery models.DeliveryAssignment `json:"delivery"`
	}
	if err := json.Unmarshal(deliveryResponse.Body.Bytes(), &assignmentResult); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"accepted", "picked_up", "in_transit", "delivered"} {
		response := request(router, http.MethodPatch, "/api/deliveries/"+assignmentResult.Delivery.ID+"/status", agent,
			`{"status":"`+status+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("agent transition %s status=%d body=%s", status, response.Code, response.Body.String())
		}
	}
	if repo.orders[orderID].Status != models.OrderDelivered {
		t.Fatalf("order status after agent delivery = %q, want delivered", repo.orders[orderID].Status)
	}
	if response := request(router, http.MethodGet, "/api/orders/"+orderID+"/tracking", customer, ""); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), "delivery") {
		t.Fatalf("customer tracking status=%d body=%s", response.Code, response.Body.String())
	}
	review := request(router, http.MethodPost, "/api/reviews", customer,
		`{"orderId":"`+orderID+`","productId":"product-1","productRating":5,"sellerRating":5,"comment":"Fresh"}`)
	if review.Code != http.StatusCreated {
		t.Fatalf("customer eligible review status=%d body=%s", review.Code, review.Body.String())
	}
}

func TestRoleRestrictionsAndOwnership(t *testing.T) {
	router, repo := newTestRouter(t)
	customer := tokenFor(t, repo, repo.users["customer-1"])
	seller := tokenFor(t, repo, repo.users["seller-1"])
	otherSeller := tokenFor(t, repo, repo.users["seller-2"])
	agent := tokenFor(t, repo, repo.users["agent-1"])
	otherAgent := tokenFor(t, repo, repo.users["agent-2"])
	admin := tokenFor(t, repo, repo.users["admin-1"])

	for _, test := range []struct {
		name, method, path, body, token string
		want                            int
	}{
		{"customer denied admin", http.MethodGet, "/api/admin/users", "", customer, http.StatusForbidden},
		{"seller denied admin", http.MethodGet, "/api/admin/users", "", seller, http.StatusForbidden},
		{"agent denied admin", http.MethodGet, "/api/admin/users", "", agent, http.StatusForbidden},
		{"admin allowed admin", http.MethodGet, "/api/admin/users", "", admin, http.StatusOK},
		{"only admin lists all deliveries", http.MethodGet, "/api/deliveries", "", agent, http.StatusForbidden},
		{"admin lists all deliveries", http.MethodGet, "/api/deliveries", "", admin, http.StatusOK},
		{"customer cannot create product", http.MethodPost, "/api/products", `{"name":"New","category":"Vegetables","price":10,"rating":4,"unit":"kg","village":"Kudlu","available":true,"stock":2,"description":"Fresh"}`, customer, http.StatusForbidden},
		{"seller creates own product", http.MethodPost, "/api/products", `{"name":"New","category":"Vegetables","price":10,"rating":4,"unit":"kg","village":"Kudlu","sellerId":"seller-2","seller":"Attacker","available":true,"stock":2,"description":"Fresh"}`, seller, http.StatusCreated},
		{"customer sees own order", http.MethodGet, "/api/orders/order-1", "", customer, http.StatusOK},
		{"customer cannot see another order", http.MethodGet, "/api/orders/order-2", "", customer, http.StatusForbidden},
		{"seller sees own seller order", http.MethodGet, "/api/orders/order-1", "", seller, http.StatusOK},
		{"seller cannot see another seller order", http.MethodGet, "/api/orders/order-3", "", seller, http.StatusForbidden},
		{"seller can update own order", http.MethodPatch, "/api/orders/order-1/status", `{"status":"confirmed"}`, seller, http.StatusOK},
		{"seller cannot update another seller order", http.MethodPatch, "/api/orders/order-3/status", `{"status":"confirmed"}`, seller, http.StatusForbidden},
		{"customer cannot cancel another order", http.MethodPatch, "/api/orders/order-2/status", `{"status":"cancelled"}`, customer, http.StatusForbidden},
		{"customer can cancel own order", http.MethodPatch, "/api/orders/order-1/status", `{"status":"cancelled"}`, customer, http.StatusOK},
		{"agent sees assigned delivery", http.MethodGet, "/api/deliveries/delivery-1", "", agent, http.StatusOK},
		{"agent cannot see other delivery", http.MethodGet, "/api/deliveries/delivery-2", "", agent, http.StatusForbidden},
		{"agent cannot query another agent", http.MethodGet, "/api/agents/agent-2/deliveries", "", agent, http.StatusForbidden},
		{"agent can update assigned delivery", http.MethodPatch, "/api/deliveries/delivery-1/status", `{"status":"accepted"}`, agent, http.StatusOK},
		{"admin can access any delivery", http.MethodGet, "/api/deliveries/delivery-2", "", admin, http.StatusOK},
		{"seller cannot query other seller products", http.MethodGet, "/api/sellers/seller-2/products", "", seller, http.StatusForbidden},
		{"seller can query own products", http.MethodGet, "/api/sellers/seller-1/products", "", seller, http.StatusOK},
		{"admin can query seller products", http.MethodGet, "/api/sellers/seller-2/products", "", admin, http.StatusOK},
		{"seller cannot edit another listing", http.MethodPut, "/api/products/product-2", `{"name":"Hijack","category":"Fruits","price":1,"rating":4,"unit":"kg","village":"Kudlu","available":true,"stock":1,"description":"Hijacked"}`, seller, http.StatusForbidden},
		{"seller edits own listing", http.MethodPut, "/api/products/product-1", `{"name":"Updated","category":"Vegetables","price":39,"rating":4,"unit":"kg","village":"Kudlu","sellerId":"seller-2","seller":"Seller Two","available":true,"stock":9,"description":"Fresh"}`, seller, http.StatusOK},
		{"agent denied orders", http.MethodGet, "/api/orders", "", agent, http.StatusForbidden},
		{"seller denied cart", http.MethodPost, "/api/cart/items", `{"productId":"product-1","villageId":"v1","quantity":1}`, seller, http.StatusForbidden},
		{"seller denied agent deliveries", http.MethodGet, "/api/agents/agent-1/deliveries", "", seller, http.StatusForbidden},
		{"seller reads own orders", http.MethodGet, "/api/seller/orders", "", seller, http.StatusOK},
		{"customer reads own tracking", http.MethodGet, "/api/orders/order-1/tracking", "", customer, http.StatusOK},
		{"customer cannot read another tracking", http.MethodGet, "/api/orders/order-2/tracking", "", customer, http.StatusForbidden},
		{"seller cannot read another seller order", http.MethodGet, "/api/seller/orders/order-3", "", seller, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(router, test.method, test.path, test.token, test.body)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d, body %s", response.Code, test.want, response.Body.String())
			}
		})
	}
	if product := repo.products["product-1"]; product.SellerID != "seller-1" || product.Seller != "Seller One" {
		t.Fatalf("seller modified product ownership: %+v", product)
	}
	if product := repo.products["product-1"]; product.Change == nil || product.Change.ActorID != "seller-1" ||
		product.ExpectedStock == nil || *product.ExpectedStock != 10 {
		t.Fatalf("seller product update did not use authenticated identity and prior stock: %+v", product)
	}
	foundCreatedProduct := false
	for _, product := range repo.products {
		if product.Name == "New" {
			foundCreatedProduct = true
			if product.SellerID != "seller-1" || product.Seller != "Seller One" {
				t.Fatalf("seller forged new product ownership: %+v", product)
			}
		}
	}
	if !foundCreatedProduct {
		t.Fatal("seller product creation did not persist")
	}

	added := request(router, http.MethodPost, "/api/cart/items", customer,
		`{"productId":"product-1","villageId":"v1","quantity":1}`)
	if added.Code != http.StatusOK {
		t.Fatalf("customer cart add status = %d body %s", added.Code, added.Body.String())
	}
	created := request(router, http.MethodPost, "/api/cart/checkout", customer,
		`{"customerId":"customer-2","sellerId":"seller-2","villageId":"v1","fulfillmentType":"delivery","address":"Kudlu","paymentMethod":"simulated","total":0.01}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("customer order creation status = %d body %s", created.Code, created.Body.String())
	}
	var createdResponse struct {
		Order models.Order `json:"order"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdResponse); err != nil {
		t.Fatal(err)
	}
	if createdResponse.Order.CustomerID != "customer-1" || createdResponse.Order.SellerID != "seller-1" ||
		createdResponse.Order.Items[0].Price != 39 || createdResponse.Order.Subtotal != 39 ||
		createdResponse.Order.DeliveryFee != 25 || createdResponse.Order.Total != 64 ||
		createdResponse.Order.Payment.Status != "paid" || createdResponse.Order.PaymentID == "" ||
		createdResponse.Order.Items[0].SellerID != "seller-1" || createdResponse.Order.Items[0].LineTotal != 39 {
		t.Fatalf("order trusted client identity/seller/price: %+v", createdResponse.Order)
	}
	if len(repo.payments) != 1 || repo.payments[0].OrderID != createdResponse.Order.ID || repo.payments[0].Amount != 64 {
		t.Fatalf("order payment record was not persisted: %+v", repo.payments)
	}

	status := request(router, http.MethodPatch, "/api/deliveries/delivery-2/status", agent, `{"status":"accepted"}`)
	if status.Code != http.StatusForbidden {
		t.Fatalf("agent updated another agent delivery with status %d", status.Code)
	}
	_ = otherSeller
	_ = otherAgent
}

func TestDeliveryProgressUpdatesOrderLifecycle(t *testing.T) {
	router, repo := newTestRouter(t)
	order := repo.orders["order-1"]
	order.Status = models.OrderReadyForPickup
	order.FulfillmentType = "delivery"
	repo.orders[order.ID] = order
	delivery := repo.deliveries["delivery-1"]
	delivery.Status = "picked_up"
	repo.deliveries[delivery.ID] = delivery
	agent := tokenFor(t, repo, repo.users["agent-1"])

	for _, step := range []struct {
		deliveryStatus string
		orderStatus    models.OrderStatus
	}{
		{"in_transit", models.OrderOutForDelivery},
		{"delivered", models.OrderDelivered},
	} {
		response := request(router, http.MethodPatch, "/api/deliveries/delivery-1/status", agent,
			`{"status":"`+step.deliveryStatus+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("delivery status %s returned %d: %s", step.deliveryStatus, response.Code, response.Body.String())
		}
		if got := repo.orders["order-1"].Status; got != step.orderStatus {
			t.Errorf("order status = %q after delivery status %q, want %q", got, step.deliveryStatus, step.orderStatus)
		}
	}
}

func TestSellerAndCustomerOrderLifecycleRoutes(t *testing.T) {
	router, repo := newTestRouter(t)
	seller := tokenFor(t, repo, repo.users["seller-1"])
	otherSeller := tokenFor(t, repo, repo.users["seller-2"])
	customer := tokenFor(t, repo, repo.users["customer-1"])
	agent := tokenFor(t, repo, repo.users["agent-1"])
	otherAgent := tokenFor(t, repo, repo.users["agent-2"])

	if response := request(router, http.MethodGet, "/api/seller/orders", seller, ""); response.Code != http.StatusOK {
		t.Fatalf("seller orders status = %d body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodPatch, "/api/seller/orders/order-2/status", seller,
		`{"status":"REJECTED"}`); response.Code != http.StatusOK {
		t.Fatalf("seller rejection status = %d body %s", response.Code, response.Body.String())
	}
	if repo.orders["order-2"].Status != models.OrderRejected {
		t.Fatalf("seller rejection left order status at %q", repo.orders["order-2"].Status)
	}
	if response := request(router, http.MethodPatch, "/api/seller/orders/order-1/status", otherSeller,
		`{"status":"CONFIRMED"}`); response.Code != http.StatusForbidden {
		t.Fatalf("other seller transition status = %d, want 403", response.Code)
	}
	for _, status := range []string{"CONFIRMED", "PREPARING", "PACKED", "READY_FOR_PICKUP"} {
		response := request(router, http.MethodPatch, "/api/seller/orders/order-1/status", seller, `{"status":"`+status+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("seller transition to %s status=%d body=%s", status, response.Code, response.Body.String())
		}
	}
	if response := request(router, http.MethodPatch, "/api/seller/orders/order-1/status", seller,
		`{"status":"DELIVERED"}`); response.Code != http.StatusConflict {
		t.Fatalf("invalid seller skip transition status = %d, want 409", response.Code)
	}
	if response := request(router, http.MethodPatch, "/api/orders/order-1/status", otherAgent,
		`{"status":"OUT_FOR_DELIVERY"}`); response.Code != http.StatusForbidden {
		t.Fatalf("unassigned agent transition status = %d, want 403", response.Code)
	}
	for _, status := range []string{"OUT_FOR_DELIVERY", "DELIVERED"} {
		response := request(router, http.MethodPatch, "/api/orders/order-1/status", agent, `{"status":"`+status+`"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("assigned agent transition to %s status=%d body=%s", status, response.Code, response.Body.String())
		}
	}
	if response := request(router, http.MethodPatch, "/api/orders/order-2/status", customer,
		`{"status":"CANCELLED"}`); response.Code != http.StatusForbidden {
		t.Fatalf("customer cancellation of another order status = %d, want 403", response.Code)
	}
	if response := request(router, http.MethodGet, "/api/orders/order-1/tracking", customer, ""); response.Code != http.StatusOK {
		t.Fatalf("customer tracking status = %d body %s", response.Code, response.Body.String())
	}
}

func TestCustomerReviewsRequireDeliveredPurchasedOrderItem(t *testing.T) {
	router, repo := newTestRouter(t)
	customer := tokenFor(t, repo, repo.users["customer-1"])
	secondCustomer := tokenFor(t, repo, repo.users["customer-2"])

	firstOrder := repo.orders["order-1"]
	firstOrder.Items = []models.OrderItem{{ProductID: "product-1", Name: "Tomato", SellerID: "seller-1", Quantity: 1}}
	repo.orders[firstOrder.ID] = firstOrder
	reviewBody := `{"orderId":"order-1","productId":"product-1","productRating":5,"sellerRating":4,"comment":"Fresh and well packed"}`
	if response := request(router, http.MethodPost, "/api/reviews", customer, reviewBody); response.Code != http.StatusConflict {
		t.Fatalf("review for undelivered order status = %d, want 409: %s", response.Code, response.Body.String())
	}

	firstOrder.Status = models.OrderDelivered
	repo.orders[firstOrder.ID] = firstOrder
	notPurchased := strings.Replace(reviewBody, `"product-1"`, `"product-2"`, 1)
	if response := request(router, http.MethodPost, "/api/reviews", customer, notPurchased); response.Code != http.StatusForbidden {
		t.Fatalf("review for unpurchased product status = %d, want 403: %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodPost, "/api/reviews", customer, reviewBody); response.Code != http.StatusCreated {
		t.Fatalf("eligible review status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodPost, "/api/reviews", customer, reviewBody); response.Code != http.StatusConflict {
		t.Fatalf("duplicate item review status = %d, want 409: %s", response.Code, response.Body.String())
	}

	secondOrder := repo.orders["order-2"]
	secondOrder.Items = []models.OrderItem{{ProductID: "product-1", Name: "Tomato", SellerID: "seller-1", Quantity: 2}}
	secondOrder.Status = models.OrderDelivered
	repo.orders[secondOrder.ID] = secondOrder
	secondBody := `{"orderId":"order-2","productId":"product-1","productRating":3,"sellerRating":2}`
	if response := request(router, http.MethodPost, "/api/reviews", secondCustomer, secondBody); response.Code != http.StatusCreated {
		t.Fatalf("second eligible review status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if got := repo.products["product-1"]; got.Rating != 4 || got.RatingCount != 2 {
		t.Fatalf("product aggregate rating = %.2f (%d reviews), want 4.00 (2)", got.Rating, got.RatingCount)
	}
	if got := repo.users["seller-1"]; got.Rating != 3 || got.RatingCount != 2 {
		t.Fatalf("seller aggregate rating = %.2f (%d reviews), want 3.00 (2)", got.Rating, got.RatingCount)
	}
	if response := request(router, http.MethodGet, "/api/orders/order-1/reviews", customer, ""); response.Code != http.StatusOK {
		t.Fatalf("customer's order review lookup status = %d, body %s", response.Code, response.Body.String())
	}
	if response := request(router, http.MethodGet, "/api/orders/order-1/reviews", secondCustomer, ""); response.Code != http.StatusForbidden {
		t.Fatalf("other customer's order review lookup status = %d, want 403", response.Code)
	}
}

func TestCartQuantityRemovalAndClearEndpoints(t *testing.T) {
	router, repo := newTestRouter(t)
	customer := tokenFor(t, repo, repo.users["customer-1"])
	added := request(router, http.MethodPost, "/api/cart/items", customer,
		`{"customerId":"customer-2","sellerId":"seller-2","productId":"product-1","villageId":"v1","quantity":2,"price":0.01}`)
	if added.Code != http.StatusOK {
		t.Fatalf("add item status = %d, body %s", added.Code, added.Body.String())
	}
	var result struct {
		Cart models.CartView `json:"cart"`
	}
	if err := json.Unmarshal(added.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Cart.Subtotal != 76 || result.Cart.Total != 101 || repo.carts["customer-1"].Items[0].Quantity != 2 {
		t.Fatalf("cart did not persist authoritative item/price data: response=%+v stored=%+v", result.Cart, repo.carts["customer-1"])
	}
	if _, exists := repo.carts["customer-2"]; exists {
		t.Fatal("request-supplied customer identity changed another customer's cart")
	}
	otherCustomer := tokenFor(t, repo, repo.users["customer-2"])
	otherCart := request(router, http.MethodGet, "/api/cart", otherCustomer, "")
	if otherCart.Code != http.StatusOK || strings.Contains(otherCart.Body.String(), `"productId":"product-1"`) {
		t.Fatalf("customer read another customer's cart: status=%d body=%s", otherCart.Code, otherCart.Body.String())
	}
	updated := request(router, http.MethodPut, "/api/cart/items/product-1", customer, `{"quantity":3}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update quantity status = %d, body %s", updated.Code, updated.Body.String())
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Cart.Subtotal != 114 || repo.carts["customer-1"].Items[0].Quantity != 3 {
		t.Fatalf("cart quantity update failed: %+v", result.Cart)
	}
	removed := request(router, http.MethodDelete, "/api/cart/items/product-1", customer, "")
	if removed.Code != http.StatusOK || len(repo.carts["customer-1"].Items) != 0 {
		t.Fatalf("remove item status=%d stored cart=%+v", removed.Code, repo.carts["customer-1"])
	}
	_ = request(router, http.MethodPost, "/api/cart/items", customer,
		`{"productId":"product-1","villageId":"v1","quantity":1}`)
	cleared := request(router, http.MethodDelete, "/api/cart", customer, "")
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear cart status = %d, body %s", cleared.Code, cleared.Body.String())
	}
	if _, exists := repo.carts["customer-1"]; exists {
		t.Fatal("clear cart did not remove the persisted customer cart")
	}
}

func TestProductDeletionChecksSellerOwnershipAndRemovesMongoRecord(t *testing.T) {
	router, repo := newTestRouter(t)
	seller := tokenFor(t, repo, repo.users["seller-1"])
	otherSeller := tokenFor(t, repo, repo.users["seller-2"])

	denied := request(router, http.MethodDelete, "/api/products/product-1", otherSeller, "")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("other seller delete status = %d, want 403", denied.Code)
	}
	deleted := request(router, http.MethodDelete, "/api/products/product-1", seller, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("owner delete status = %d, body %s", deleted.Code, deleted.Body.String())
	}
	if _, exists := repo.products["product-1"]; exists {
		t.Fatal("seller product deletion did not remove the MongoDB source record")
	}
}

func TestTokenRoleIsReloadedFromUserRecord(t *testing.T) {
	router, repo := newTestRouter(t)
	token := tokenFor(t, repo, repo.users["customer-1"])
	user := repo.users["customer-1"]
	user.Role = models.RoleAdmin
	repo.users[user.ID] = user
	response := request(router, http.MethodGet, "/api/admin/users", token, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("stale token role status = %d, want 401", response.Code)
	}
}

func TestUnverifiedSellerCannotUseSellerRoutes(t *testing.T) {
	router, repo := newTestRouter(t)
	user := repo.users["seller-1"]
	user.Verified = false
	repo.users[user.ID] = user
	token := tokenFor(t, repo, user)
	response := request(router, http.MethodGet, "/api/sellers/seller-1/products", token, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("unverified seller status = %d, want 403", response.Code)
	}
}

func TestExpiredAndRevokedSessionsAreRejected(t *testing.T) {
	router, repo := newTestRouter(t)
	user := repo.users["customer-1"]
	token, claims, err := auth.GenerateSessionToken(testJWTSecret, user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().UTC().Add(-time.Minute)
	repo.sessions[claims.ID] = models.AuthSession{ID: claims.ID, UserID: user.ID, ExpiresAt: expired}
	response := request(router, http.MethodGet, "/api/auth/me", token, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status = %d, want 401", response.Code)
	}

	_, missingErr := repo.GetSession(context.Background(), "missing")
	if !errors.Is(missingErr, repository.ErrNotFound) {
		t.Fatalf("missing session error = %v", missingErr)
	}
}
