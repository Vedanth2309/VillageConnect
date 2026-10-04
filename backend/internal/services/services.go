package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/mail"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"villageconnect/internal/auth"
	"villageconnect/internal/models"
	"villageconnect/internal/repository"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrConflict     = errors.New("resource conflict")
	ErrUnauthorized = errors.New("invalid credentials")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("resource not found")
)

const SessionLifetime = 24 * time.Hour
const DeliveryFee = 25

var (
	dummyPasswordHashOnce sync.Once
	dummyPasswordHash     []byte
	dummyPasswordHashErr  error
)

type UserService struct{ repo repository.Repository }
type CatalogService struct {
	repo    repository.Repository
	indexer ProductIndexer
}
type OrderService struct{ repo repository.Repository }
type DeliveryService struct {
	repo   repository.Repository
	orders *OrderService
}
type ReviewService struct {
	repo    repository.Repository
	indexer ProductIndexer
}
type NotificationService struct{ repo repository.Repository }
type AdminService struct{ repo repository.Repository }

type ProductIndexer interface {
	IndexProduct(context.Context, models.Product) error
	DeleteProduct(context.Context, string) error
}

func NewUserService(repo repository.Repository) *UserService { return &UserService{repo: repo} }
func NewCatalogService(repo repository.Repository, indexers ...ProductIndexer) *CatalogService {
	service := &CatalogService{repo: repo}
	if len(indexers) > 0 {
		service.indexer = indexers[0]
	}
	return service
}
func NewOrderService(repo repository.Repository) *OrderService {
	return &OrderService{repo: repo}
}
func NewDeliveryService(repo repository.Repository, orderServices ...*OrderService) *DeliveryService {
	orders := NewOrderService(repo)
	if len(orderServices) > 0 && orderServices[0] != nil {
		orders = orderServices[0]
	}
	return &DeliveryService{repo: repo, orders: orders}
}
func NewAdminService(repo repository.Repository) *AdminService {
	return &AdminService{repo: repo}
}
func NewReviewService(repo repository.Repository, indexers ...ProductIndexer) *ReviewService {
	service := &ReviewService{repo: repo}
	if len(indexers) > 0 {
		service.indexer = indexers[0]
	}
	return service
}
func NewNotificationService(repo repository.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) List(ctx context.Context, userID string, limit int) ([]models.Notification, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidInput
	}
	return s.repo.ListNotifications(ctx, userID, limit)
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountUnreadNotifications(ctx, userID)
}

func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID string) error {
	return translateRepositoryError(s.repo.MarkNotificationRead(ctx, userID, notificationID))
}

func createNotification(userID, notificationType, title, message, entityID string) models.Notification {
	return models.Notification{
		ID: fmt.Sprintf("n%d", time.Now().UnixNano()), UserID: userID,
		Type: notificationType, Title: title, Message: message, EntityID: entityID,
		CreatedAt: time.Now().UTC(),
	}
}

func sendNotification(ctx context.Context, repo repository.Repository, notification models.Notification) {
	if notification.UserID == "" {
		return
	}
	if err := repo.CreateNotification(ctx, notification); err != nil {
		log.Printf("notification %s could not be persisted: %v", notification.ID, err)
	}
}

func (s *ReviewService) ForOrder(ctx context.Context, orderID string) ([]models.Review, error) {
	return s.repo.ListReviewsByOrder(ctx, orderID)
}

func (s *ReviewService) ForProduct(ctx context.Context, productID string) ([]models.Review, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListReviewsByProduct(ctx, productID)
}

func (s *ReviewService) ForSeller(ctx context.Context, sellerID string) ([]models.Review, error) {
	if strings.TrimSpace(sellerID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListReviewsBySeller(ctx, sellerID)
}

func (s *ReviewService) Create(ctx context.Context, customerID, orderID, productID string,
	productRating, sellerRating int, comment string) (*models.Review, error) {
	orderID = strings.TrimSpace(orderID)
	productID = strings.TrimSpace(productID)
	comment = strings.TrimSpace(comment)
	if orderID == "" || productID == "" || productRating < 1 || productRating > 5 ||
		sellerRating < 1 || sellerRating > 5 || len(comment) > 1000 {
		return nil, ErrInvalidInput
	}
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, translateRepositoryError(err)
	}
	if order.CustomerID != customerID {
		return nil, ErrForbidden
	}
	if order.Status != models.OrderDelivered {
		return nil, ErrConflict
	}
	sellerID := ""
	purchased := false
	for _, item := range order.Items {
		if item.ProductID == productID {
			purchased = true
			sellerID = item.SellerID
			break
		}
	}
	if !purchased {
		return nil, ErrForbidden
	}
	if sellerID == "" {
		sellerID = order.SellerID
	}
	review := models.Review{
		ID: fmt.Sprintf("r%d", time.Now().UnixNano()), OrderID: order.ID,
		ProductID: productID, SellerID: sellerID, CustomerID: customerID,
		ProductRating: productRating, SellerRating: sellerRating,
		Comment: comment, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := translateRepositoryError(s.repo.CreateReview(ctx, review)); err != nil {
		return nil, err
	}
	if s.indexer != nil {
		product, productErr := s.repo.GetProductByID(ctx, productID)
		if productErr == nil {
			if indexErr := s.indexer.IndexProduct(ctx, *product); indexErr != nil {
				log.Printf("review %s saved but product rating indexing failed: %v", review.ID, indexErr)
			}
		}
	}
	return &review, nil
}

func (s *UserService) Register(ctx context.Context, name, email, phone, password, villageID, rawRole string) (*models.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || len(password) < 8 || len(password) > 72 {
		return nil, ErrInvalidInput
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return nil, ErrInvalidInput
	}

	role := models.RoleCustomer
	switch strings.ToLower(strings.TrimSpace(rawRole)) {
	case "", "customer":
	default:
		// Elevated roles are assigned through trusted administrative
		// provisioning, never from a public registration payload.
		return nil, ErrInvalidInput
	}

	if _, err := s.repo.GetUserByEmailForAuth(ctx, email); err == nil {
		return nil, ErrConflict
	} else if !isMissingUser(err) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}
	user := models.User{
		ID:           fmt.Sprintf("u%d", time.Now().UnixNano()),
		Name:         name,
		Email:        email,
		Phone:        strings.TrimSpace(phone),
		VillageID:    strings.TrimSpace(villageID),
		Role:         role,
		Verified:     true,
		PasswordHash: string(hash),
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, translateRepositoryError(err)
	}
	return &user, nil
}

func (s *UserService) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" || len(password) > 72 {
		return nil, ErrInvalidInput
	}
	user, err := s.repo.GetUserByEmailForAuth(ctx, email)
	if err != nil {
		if isMissingUser(err) {
			if err := compareDummyPassword(password); err != nil {
				return nil, err
			}
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrUnauthorized
	}
	if !user.IsActive() {
		return nil, ErrUnauthorized
	}
	return user, nil
}

func compareDummyPassword(password string) error {
	dummyPasswordHashOnce.Do(func() {
		dummyPasswordHash, dummyPasswordHashErr = bcrypt.GenerateFromPassword(
			[]byte("VillageConnect-unmatched-account-password"), 12)
	})
	if dummyPasswordHashErr != nil {
		return dummyPasswordHashErr
	}
	_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
	return nil
}

func (s *UserService) CreateSession(ctx context.Context, claims *auth.Claims) error {
	if claims == nil || claims.ID == "" || claims.UserID == "" || claims.ExpiresAt == nil {
		return ErrInvalidInput
	}
	now := time.Now().UTC()
	if !claims.ExpiresAt.Time.After(now) || claims.ExpiresAt.Time.After(now.Add(SessionLifetime)) {
		return ErrInvalidInput
	}
	session := models.AuthSession{
		ID:        claims.ID,
		UserID:    claims.UserID,
		ExpiresAt: claims.ExpiresAt.Time,
		CreatedAt: now,
	}
	return translateRepositoryError(s.repo.CreateSession(ctx, session))
}

func (s *UserService) ValidateSession(ctx context.Context, claims *auth.Claims) (models.User, error) {
	if claims == nil || claims.ID == "" || claims.UserID == "" {
		return models.User{}, ErrUnauthorized
	}
	session, err := s.repo.GetSession(ctx, claims.ID)
	if err != nil {
		if isNotFound(err) {
			return models.User{}, ErrUnauthorized
		}
		return models.User{}, err
	}
	now := time.Now().UTC()
	if session.UserID != claims.UserID || session.RevokedAt != nil ||
		!session.ExpiresAt.After(now) || claims.ExpiresAt == nil ||
		!session.ExpiresAt.Equal(claims.ExpiresAt.Time) {
		return models.User{}, ErrUnauthorized
	}
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if isNotFound(err) {
			return models.User{}, ErrUnauthorized
		}
		return models.User{}, err
	}
	if string(user.Role) != claims.Role {
		return models.User{}, ErrUnauthorized
	}
	if !user.IsActive() {
		return models.User{}, ErrUnauthorized
	}
	return *user, nil
}

func (s *UserService) RevokeSession(ctx context.Context, claims *auth.Claims) error {
	if claims == nil || claims.ID == "" || claims.UserID == "" {
		return ErrUnauthorized
	}
	session, err := s.repo.GetSession(ctx, claims.ID)
	if err != nil {
		if isNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	if session.UserID != claims.UserID {
		return ErrUnauthorized
	}
	return s.repo.RevokeSession(ctx, claims.ID, time.Now().UTC())
}

func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if isMissingUser(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *UserService) RequestSellerVerification(ctx context.Context, sellerID string) error {
	user, err := s.repo.GetUserByID(ctx, sellerID)
	if err != nil {
		return translateRepositoryError(err)
	}
	if user.Role != models.RoleSeller {
		return ErrForbidden
	}
	if user.Verified && (user.SellerStatus == "" || user.SellerStatus == "approved") {
		return ErrConflict
	}
	if err := translateRepositoryError(s.repo.SetSellerState(ctx, sellerID, "pending", false, true)); err != nil {
		return err
	}
	users, err := s.repo.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, admin := range users {
		if admin.Role == models.RoleAdmin && admin.IsActive() {
			sendNotification(ctx, s.repo, createNotification(admin.ID, "seller_verification", "Seller verification requested",
				fmt.Sprintf("%s requested seller verification.", user.Name), sellerID))
		}
	}
	return nil
}

func (s *CatalogService) Villages(ctx context.Context) ([]models.Village, error) {
	villages, err := s.repo.ListVillages(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]models.Village, 0, len(villages))
	for _, village := range villages {
		if village.IsActive() {
			active = append(active, village)
		}
	}
	return active, nil
}
func (s *CatalogService) Categories(ctx context.Context) ([]models.Category, error) {
	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]models.Category, 0, len(categories))
	for _, category := range categories {
		if category.IsActive() {
			active = append(active, category)
		}
	}
	return active, nil
}
func (s *CatalogService) Products(ctx context.Context, query models.ProductQuery) (models.ProductPage, error) {
	query.Query = strings.TrimSpace(query.Query)
	query.Category = strings.TrimSpace(query.Category)
	if len(query.Query) > 200 || len(query.Category) > 100 ||
		query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 ||
		(query.MinPrice != nil && (math.IsNaN(*query.MinPrice) || math.IsInf(*query.MinPrice, 0) || *query.MinPrice < 0)) ||
		(query.MaxPrice != nil && (math.IsNaN(*query.MaxPrice) || math.IsInf(*query.MaxPrice, 0) || *query.MaxPrice < 0)) ||
		(query.MinPrice != nil && query.MaxPrice != nil && *query.MinPrice > *query.MaxPrice) {
		return models.ProductPage{}, ErrInvalidInput
	}
	switch query.Sort {
	case "", "name_asc", "name_desc", "price_asc", "price_desc", "rating_desc":
	default:
		return models.ProductPage{}, ErrInvalidInput
	}
	if query.VillageID != "" {
		villages, err := s.repo.ListVillages(ctx)
		if err != nil {
			return models.ProductPage{}, err
		}
		found := false
		villageName := ""
		for _, village := range villages {
			if village.ID == query.VillageID && village.IsActive() {
				villageName = village.Name
				found = true
			}
		}
		matchingVillageNames := 0
		for _, village := range villages {
			if strings.EqualFold(strings.TrimSpace(village.Name), strings.TrimSpace(villageName)) {
				matchingVillageNames++
			}
		}
		if !found {
			return models.ProductPage{}, ErrInvalidInput
		}
		query.VillageName = villageName
		if matchingVillageNames != 1 {
			query.VillageName = ""
		}
	}
	if query.Category != "" {
		categories, err := s.repo.ListCategories(ctx)
		if err != nil {
			return models.ProductPage{}, err
		}
		found := false
		for _, category := range categories {
			if category.IsActive() && strings.EqualFold(category.Name, query.Category) {
				found = true
				break
			}
		}
		if !found {
			return models.ProductPage{}, ErrInvalidInput
		}
	}
	if query.Sort == "" {
		query.Sort = "name_asc"
	}
	return s.repo.ListMarketplaceProducts(ctx, query)
}
func (s *CatalogService) Product(ctx context.Context, id string) (*models.Product, error) {
	product, err := s.repo.GetProductByID(ctx, id)
	if err != nil && isNotFound(err) {
		return nil, ErrNotFound
	}
	return product, err
}
func (s *CatalogService) Search(ctx context.Context, query string) ([]models.Product, error) {
	if len(query) > 200 {
		return nil, ErrInvalidInput
	}
	return s.repo.SearchProducts(ctx, strings.TrimSpace(query))
}
func (s *CatalogService) SellerProducts(ctx context.Context, sellerID string) ([]models.Product, error) {
	if strings.TrimSpace(sellerID) == "" {
		return nil, ErrInvalidInput
	}
	return s.repo.ListSellerProducts(ctx, sellerID)
}

func (s *CatalogService) CreateProduct(ctx context.Context, product models.Product) error {
	if err := s.assignProductVillage(ctx, &product); err != nil {
		return err
	}
	if err := validateProduct(product); err != nil {
		return err
	}
	if err := translateRepositoryError(s.repo.CreateProduct(ctx, product)); err != nil {
		return err
	}
	if s.indexer != nil {
		if err := s.indexer.IndexProduct(ctx, product); err != nil {
			log.Printf("product %s saved to MongoDB but search indexing failed: %v", product.ID, err)
		}
	}
	return nil
}
func (s *CatalogService) UpdateProduct(ctx context.Context, product models.Product) error {
	if err := s.assignProductVillage(ctx, &product); err != nil {
		return err
	}
	if err := validateProduct(product); err != nil {
		return err
	}
	if err := translateRepositoryError(s.repo.UpdateProduct(ctx, product)); err != nil {
		return err
	}
	if product.Change != nil && product.Change.NewStock != nil && *product.Change.NewStock <= 5 {
		sendNotification(ctx, s.repo, createNotification(product.SellerID, "low_stock", "Low stock alert",
			fmt.Sprintf("%s now has %d units in stock.", product.Name, *product.Change.NewStock), product.ID))
	}
	if s.indexer != nil {
		if err := s.indexer.IndexProduct(ctx, product); err != nil {
			log.Printf("product %s updated in MongoDB but search indexing failed: %v", product.ID, err)
		}
	}
	return nil
}

func (s *CatalogService) SetProductAvailability(ctx context.Context, id string, available bool) error {
	product, err := s.Product(ctx, id)
	if err != nil {
		return err
	}
	if available && product.Stock == 0 {
		return ErrConflict
	}
	if err := translateRepositoryError(s.repo.SetProductAvailability(ctx, id, available)); err != nil {
		return err
	}
	product.Available = available
	if s.indexer != nil {
		if err := s.indexer.IndexProduct(ctx, *product); err != nil {
			log.Printf("product %s availability saved but search indexing failed: %v", id, err)
		}
	}
	return nil
}

func (s *CatalogService) DeleteProduct(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidInput
	}
	if _, err := s.repo.GetProductByID(ctx, id); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if err := translateRepositoryError(s.repo.DeleteProduct(ctx, id)); err != nil {
		return err
	}
	if s.indexer != nil {
		if err := s.indexer.DeleteProduct(ctx, id); err != nil {
			log.Printf("product %s deleted from MongoDB but search index deletion failed: %v", id, err)
		}
	}
	return nil
}

func (s *CatalogService) assignProductVillage(ctx context.Context, product *models.Product) error {
	villages, err := s.repo.ListVillages(ctx)
	if err != nil {
		return err
	}
	villageFound := false
	for _, village := range villages {
		if !village.IsActive() {
			continue
		}
		if product.VillageID != "" && product.VillageID == village.ID {
			if strings.TrimSpace(product.Village) != "" && !strings.EqualFold(strings.TrimSpace(product.Village), village.Name) {
				return ErrInvalidInput
			}
			product.VillageID = village.ID
			product.Village = village.Name
			villageFound = true
			break
		}
		if product.VillageID == "" && strings.EqualFold(strings.TrimSpace(product.Village), village.Name) {
			product.VillageID = village.ID
			product.Village = village.Name
			villageFound = true
			break
		}
	}
	if !villageFound {
		return ErrInvalidInput
	}
	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return err
	}
	categoryFound := false
	for _, category := range categories {
		if category.IsActive() && strings.EqualFold(strings.TrimSpace(category.Name), strings.TrimSpace(product.Category)) {
			categoryFound = true
			break
		}
	}
	if !categoryFound {
		return ErrInvalidInput
	}
	return nil
}

func validateProduct(product models.Product) error {
	if strings.TrimSpace(product.Name) == "" ||
		len(strings.TrimSpace(product.Name)) > 120 ||
		strings.TrimSpace(product.Category) == "" ||
		len(strings.TrimSpace(product.Category)) > 100 ||
		strings.TrimSpace(product.Unit) == "" ||
		len(strings.TrimSpace(product.Unit)) > 32 ||
		strings.TrimSpace(product.Village) == "" || strings.TrimSpace(product.VillageID) == "" ||
		strings.TrimSpace(product.SellerID) == "" ||
		strings.TrimSpace(product.Seller) == "" ||
		strings.TrimSpace(product.Description) == "" ||
		len(strings.TrimSpace(product.Description)) > 2000 ||
		math.IsNaN(product.Price) || math.IsInf(product.Price, 0) || product.Price <= 0 ||
		product.Stock < 0 || math.IsNaN(product.Rating) || math.IsInf(product.Rating, 0) ||
		product.Rating < 0 || product.Rating > 5 || (product.Available && product.Stock == 0) {
		return ErrInvalidInput
	}
	return nil
}

func (s *OrderService) List(ctx context.Context) ([]models.Order, error) {
	return s.repo.ListOrders(ctx)
}
func (s *OrderService) ListForCustomer(ctx context.Context, customerID string) ([]models.Order, error) {
	orders, err := s.repo.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]models.Order, 0)
	for _, order := range orders {
		if order.CustomerID == customerID {
			restored, restoreErr := s.restorePendingInventory(ctx, order)
			if restoreErr != nil {
				return nil, restoreErr
			}
			order = restored
			result = append(result, order)
		}
	}
	return result, nil
}
func (s *OrderService) ListForSeller(ctx context.Context, sellerID string) ([]models.Order, error) {
	orders, err := s.repo.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]models.Order, 0)
	for _, order := range orders {
		if order.SellerID == sellerID {
			restored, restoreErr := s.restorePendingInventory(ctx, order)
			if restoreErr != nil {
				return nil, restoreErr
			}
			order = restored
			result = append(result, order)
		}
	}
	return result, nil
}

func (s *OrderService) restorePendingInventory(ctx context.Context, order models.Order) (models.Order, error) {
	if order.InventoryReserved && !order.InventoryRestored &&
		(order.Status == models.OrderCancelled || order.Status == models.OrderRejected) {
		restored, err := s.repo.RestoreOrderInventory(ctx, order.ID)
		if err != nil {
			return order, translateRepositoryError(err)
		}
		return *restored, nil
	}
	return order, nil
}
func (s *OrderService) Get(ctx context.Context, id string) (*models.Order, error) {
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil && isNotFound(err) {
		return nil, ErrNotFound
	}
	return order, err
}
func (s *OrderService) Tracking(ctx context.Context, id, customerID string) (*models.Order, error) {
	order, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.CustomerID != customerID {
		return nil, ErrForbidden
	}
	return order, nil
}
func (s *OrderService) GetCart(ctx context.Context, customerID, fulfillmentType string) (models.CartView, error) {
	fulfillmentType = strings.TrimSpace(fulfillmentType)
	if fulfillmentType == "" {
		fulfillmentType = "delivery"
	}
	if fulfillmentType != "delivery" && fulfillmentType != "pickup" {
		return models.CartView{}, ErrInvalidInput
	}
	cart, err := s.repo.GetCart(ctx, customerID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return models.CartView{}, err
	}
	view := models.CartView{Items: []models.CartLine{}, FulfillmentType: fulfillmentType}
	if cart == nil {
		return view, nil
	}
	view.VillageID = cart.VillageID
	for _, item := range cart.Items {
		product, err := s.repo.GetProductByID(ctx, item.ProductID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				view.Items = append(view.Items, models.CartLine{
					ProductID: item.ProductID, Quantity: item.Quantity, Issue: "Product no longer exists",
				})
				continue
			}
			return models.CartView{}, err
		}
		sellerID, sellerErr := s.productSeller(ctx, *product)
		serviceabilityErr := s.validateVillageProduct(ctx, cart.VillageID, *product)
		if serviceabilityErr != nil && !errors.Is(serviceabilityErr, ErrInvalidInput) &&
			!errors.Is(serviceabilityErr, ErrConflict) {
			return models.CartView{}, serviceabilityErr
		}
		if sellerErr != nil && !errors.Is(sellerErr, repository.ErrNotFound) &&
			!errors.Is(sellerErr, ErrConflict) {
			return models.CartView{}, sellerErr
		}
		line := models.CartLine{
			ProductID: product.ID, Name: product.Name, Price: product.Price, Unit: product.Unit,
			Quantity: item.Quantity, Stock: product.Stock, Available: product.Available && product.Stock > 0,
			SellerID: sellerID, VillageID: product.VillageID,
		}
		switch {
		case !product.Available || product.Stock < item.Quantity:
			line.Issue = "Product is unavailable or has insufficient stock"
		case errors.Is(serviceabilityErr, ErrInvalidInput) || errors.Is(serviceabilityErr, ErrConflict):
			line.Issue = "Product is not serviceable in this village"
		case sellerErr != nil || sellerID == "":
			line.Issue = "Seller is unavailable or unverified"
		case cart.SellerID != "" && sellerID != cart.SellerID:
			line.Issue = "Cart contains products from different sellers"
		default:
			line.Valid = true
			view.Subtotal += product.Price * float64(item.Quantity)
		}
		view.Items = append(view.Items, line)
	}
	if len(view.Items) > 0 && fulfillmentType == "delivery" {
		view.DeliveryFee = DeliveryFee
	}
	view.Total = view.Subtotal + view.DeliveryFee
	return view, nil
}

func (s *OrderService) AddCartItem(ctx context.Context, customerID, productID, villageID string, quantity int) error {
	if strings.TrimSpace(customerID) == "" || strings.TrimSpace(productID) == "" ||
		strings.TrimSpace(villageID) == "" || quantity < 1 || quantity > 1000 {
		return ErrInvalidInput
	}
	product, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		return translateRepositoryError(err)
	}
	if !product.Available || product.Stock < quantity {
		return ErrConflict
	}
	sellerID, err := s.productSeller(ctx, *product)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, ErrConflict) {
			return ErrConflict
		}
		return err
	}
	if sellerID == "" {
		return ErrConflict
	}
	if err := s.validateVillageProduct(ctx, villageID, *product); err != nil {
		return err
	}

	cart, err := s.getOrNewCart(ctx, customerID)
	if err != nil {
		return err
	}
	if cart.VillageID != "" && cart.VillageID != villageID {
		return ErrConflict
	}
	if cart.SellerID != "" && cart.SellerID != sellerID {
		return ErrConflict
	}
	cart.VillageID, cart.SellerID = villageID, sellerID
	found := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			if cart.Items[i].Quantity+quantity > product.Stock {
				return ErrConflict
			}
			cart.Items[i].Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, models.CartItem{ProductID: productID, Quantity: quantity})
	}
	cart.UpdatedAt = time.Now().UTC()
	return translateRepositoryError(s.repo.SaveCart(ctx, *cart, cart.Revision))
}

func (s *OrderService) SetCartItemQuantity(ctx context.Context, customerID, productID string, quantity int) error {
	if quantity < 1 || quantity > 1000 {
		return ErrInvalidInput
	}
	cart, err := s.getOrNewCart(ctx, customerID)
	if err != nil {
		return err
	}
	for i := range cart.Items {
		item := &cart.Items[i]
		if item.ProductID != productID {
			continue
		}
		product, err := s.repo.GetProductByID(ctx, productID)
		if err != nil {
			return translateRepositoryError(err)
		}
		if quantity > item.Quantity && (!product.Available || quantity > product.Stock) {
			return ErrConflict
		}
		item.Quantity = quantity
		cart.UpdatedAt = time.Now().UTC()
		return translateRepositoryError(s.repo.SaveCart(ctx, *cart, cart.Revision))
	}
	return ErrNotFound
}

func (s *OrderService) RemoveCartItem(ctx context.Context, customerID, productID string) error {
	cart, err := s.getOrNewCart(ctx, customerID)
	if err != nil {
		return err
	}
	for i := range cart.Items {
		if cart.Items[i].ProductID == productID {
			cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			if len(cart.Items) == 0 {
				cart.VillageID, cart.SellerID = "", ""
			}
			cart.UpdatedAt = time.Now().UTC()
			return translateRepositoryError(s.repo.SaveCart(ctx, *cart, cart.Revision))
		}
	}
	return ErrNotFound
}

func (s *OrderService) ClearCart(ctx context.Context, customerID string) error {
	return s.repo.ClearCart(ctx, customerID)
}

func (s *OrderService) Checkout(ctx context.Context, customerID, villageID, fulfillmentType, address, paymentMethod string) (models.Order, error) {
	if fulfillmentType != "delivery" && fulfillmentType != "pickup" {
		return models.Order{}, ErrInvalidInput
	}
	address = strings.TrimSpace(address)
	if fulfillmentType == "delivery" && (address == "" || len(address) > 500) ||
		fulfillmentType == "pickup" && address != "" {
		return models.Order{}, ErrInvalidInput
	}
	if (fulfillmentType == "delivery" && paymentMethod != "simulated" && paymentMethod != "cash_on_delivery") ||
		(fulfillmentType == "pickup" && paymentMethod != "simulated" && paymentMethod != "cash_on_pickup") {
		return models.Order{}, ErrInvalidInput
	}
	customer, err := s.repo.GetUserByID(ctx, customerID)
	if err != nil || customer.Role != models.RoleCustomer {
		return models.Order{}, ErrUnauthorized
	}
	cart, err := s.repo.GetCart(ctx, customerID)
	if err != nil {
		return models.Order{}, translateRepositoryError(err)
	}
	if len(cart.Items) == 0 || len(cart.Items) > 100 || cart.VillageID != villageID {
		return models.Order{}, ErrConflict
	}
	village, err := s.getVillage(ctx, villageID)
	if err != nil {
		return models.Order{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	order := models.Order{
		ID: fmt.Sprintf("o%d", time.Now().UnixNano()), CustomerID: customerID,
		VillageID: village.ID, Items: make([]models.OrderItem, 0, len(cart.Items)),
		FulfillmentType: fulfillmentType, Address: address, Status: models.OrderPlaced,
		CreatedAt: now, UpdatedAt: now,
		InventoryReserved: true,
		StatusHistory: []models.OrderStatusEvent{{Status: models.OrderPending, ActorID: customerID,
			ActorRole: models.RoleCustomer, CreatedAt: now, Note: "Order placed"}},
	}
	seenProducts := make(map[string]struct{}, len(cart.Items))
	for _, cartItem := range cart.Items {
		if cartItem.Quantity < 1 || cartItem.Quantity > 1000 {
			return models.Order{}, ErrInvalidInput
		}
		if _, duplicate := seenProducts[cartItem.ProductID]; duplicate {
			return models.Order{}, ErrConflict
		}
		seenProducts[cartItem.ProductID] = struct{}{}
		product, err := s.repo.GetProductByID(ctx, cartItem.ProductID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return models.Order{}, ErrConflict
			}
			return models.Order{}, err
		}
		if !product.Available || product.Stock < cartItem.Quantity {
			return models.Order{}, ErrConflict
		}
		if err := s.validateVillageProduct(ctx, villageID, *product); err != nil {
			return models.Order{}, ErrConflict
		}
		sellerID, err := s.productSeller(ctx, *product)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) || errors.Is(err, ErrConflict) {
				return models.Order{}, ErrConflict
			}
			return models.Order{}, err
		}
		if sellerID == "" || cart.SellerID != sellerID ||
			order.SellerID != "" && order.SellerID != sellerID {
			return models.Order{}, ErrConflict
		}
		order.SellerID = sellerID
		order.Items = append(order.Items, models.OrderItem{
			ProductID: product.ID, Name: product.Name, Unit: product.Unit, SellerID: sellerID,
			Quantity: cartItem.Quantity, Price: product.Price, LineTotal: product.Price * float64(cartItem.Quantity),
		})
		order.Subtotal += product.Price * float64(cartItem.Quantity)
	}
	if math.IsNaN(order.Subtotal) || math.IsInf(order.Subtotal, 0) {
		return models.Order{}, ErrInvalidInput
	}
	if fulfillmentType == "delivery" {
		order.DeliveryFee = DeliveryFee
	}
	order.Total = order.Subtotal + order.DeliveryFee
	paymentStatus := "pending"
	if paymentMethod == "simulated" {
		paymentStatus = "paid"
	}
	order.Payment = models.OrderPayment{Method: paymentMethod, Status: paymentStatus, Amount: order.Total}
	order.PaymentID = fmt.Sprintf("pay%s", order.ID[1:])
	payment := models.Payment{
		ID: order.PaymentID, OrderID: order.ID, CustomerID: customerID,
		Method: paymentMethod, Status: paymentStatus, Amount: order.Total,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := translateRepositoryError(s.repo.CheckoutOrder(ctx, order, payment, cart.Revision)); err != nil {
		return models.Order{}, err
	}
	sendNotification(ctx, s.repo, createNotification(order.SellerID, "new_order", "New order received",
		fmt.Sprintf("Order %s has been placed.", order.ID), order.ID))
	for _, item := range order.Items {
		product, productErr := s.repo.GetProductByID(ctx, item.ProductID)
		if productErr == nil && product.Stock <= 5 {
			sendNotification(ctx, s.repo, createNotification(order.SellerID, "low_stock", "Low stock alert",
				fmt.Sprintf("%s now has %d units in stock.", product.Name, product.Stock), product.ID))
		}
	}
	return order, nil
}

func (s *OrderService) getOrNewCart(ctx context.Context, customerID string) (*models.Cart, error) {
	cart, err := s.repo.GetCart(ctx, customerID)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	return &models.Cart{CustomerID: customerID, Items: []models.CartItem{}}, nil
}

func (s *OrderService) productSeller(ctx context.Context, product models.Product) (string, error) {
	var seller *models.User
	if product.SellerID != "" {
		user, err := s.repo.GetUserByID(ctx, product.SellerID)
		if err != nil {
			return "", err
		}
		seller = user
	} else {
		users, err := s.repo.ListUsers(ctx)
		if err != nil {
			return "", err
		}
		for i := range users {
			if users[i].Role == models.RoleSeller &&
				strings.EqualFold(strings.TrimSpace(users[i].Name), strings.TrimSpace(product.Seller)) {
				if seller != nil {
					return "", ErrConflict
				}
				seller = &users[i]
			}
		}
	}
	if seller == nil || seller.Role != models.RoleSeller || !seller.Verified || !seller.IsActive() {
		return "", ErrConflict
	}
	return seller.ID, nil
}

func (s *OrderService) getVillage(ctx context.Context, villageID string) (*models.Village, error) {
	if strings.TrimSpace(villageID) == "" {
		return nil, ErrInvalidInput
	}
	villages, err := s.repo.ListVillages(ctx)
	if err != nil {
		return nil, err
	}
	for i := range villages {
		if villages[i].ID == villageID {
			if !villages[i].IsActive() {
				return nil, ErrInvalidInput
			}
			return &villages[i], nil
		}
	}
	return nil, ErrInvalidInput
}

func (s *OrderService) validateVillageProduct(ctx context.Context, villageID string, product models.Product) error {
	village, err := s.getVillage(ctx, villageID)
	if err != nil {
		return err
	}
	if product.VillageID != "" {
		if product.VillageID != villageID {
			return ErrConflict
		}
	} else {
		if !strings.EqualFold(strings.TrimSpace(product.Village), strings.TrimSpace(village.Name)) {
			return ErrConflict
		}
		villages, err := s.repo.ListVillages(ctx)
		if err != nil {
			return err
		}
		nameMatches := 0
		for _, candidate := range villages {
			if strings.EqualFold(strings.TrimSpace(candidate.Name), strings.TrimSpace(village.Name)) {
				nameMatches++
			}
		}
		if nameMatches != 1 {
			return ErrConflict
		}
	}
	return nil
}
func (s *OrderService) UpdateStatus(ctx context.Context, id string, status models.OrderStatus, actorID, actorRole string) (*models.Order, error) {
	status = normalizeOrderStatus(status)
	if !validOrderStatus(status) {
		return nil, ErrInvalidInput
	}
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if order.Status == models.OrderReadyForPickup && status == models.OrderDelivered &&
		order.FulfillmentType != "pickup" {
		return nil, ErrConflict
	}
	switch actorRole {
	case string(models.RoleCustomer):
		if order.CustomerID != actorID || status != models.OrderCancelled {
			return nil, ErrForbidden
		}
	case string(models.RoleSeller):
		if order.SellerID != actorID {
			return nil, ErrForbidden
		}
		if !validOrderTransition(order.Status, status) {
			return nil, ErrConflict
		}
		if !validSellerTransition(order.Status, status) {
			return nil, ErrForbidden
		}
	case string(models.RoleDeliveryAgent):
		if !validOrderTransition(order.Status, status) {
			return nil, ErrConflict
		}
		if !validOrderDeliveryTransition(order.Status, status) {
			return nil, ErrForbidden
		}
		assignments, listErr := s.repo.ListAgentDeliveries(ctx, actorID)
		if listErr != nil {
			return nil, listErr
		}
		assigned := false
		for _, assignment := range assignments {
			if assignment.OrderID == order.ID {
				assigned = true
				break
			}
		}
		if !assigned {
			return nil, ErrForbidden
		}
	case string(models.RoleAdmin):
	default:
		return nil, ErrForbidden
	}
	if !validOrderTransition(order.Status, status) {
		return nil, ErrConflict
	}
	event := models.OrderStatusEvent{Status: status, ActorID: actorID, ActorRole: models.UserRole(actorRole)}
	updated, err := s.repo.TransitionOrder(ctx, id, order.Status, status, actorID, event)
	if err != nil {
		return nil, translateRepositoryError(err)
	}
	if actorRole != string(models.RoleCustomer) {
		sendNotification(ctx, s.repo, createNotification(order.CustomerID, "order_status", "Order status updated",
			fmt.Sprintf("Order %s is now %s.", order.ID, strings.ReplaceAll(string(status), "_", " ")), order.ID))
	}
	if actorRole != string(models.RoleSeller) && actorRole != string(models.RoleAdmin) {
		sendNotification(ctx, s.repo, createNotification(order.SellerID, "order_status", "Order status updated",
			fmt.Sprintf("Order %s is now %s.", order.ID, strings.ReplaceAll(string(status), "_", " ")), order.ID))
	}
	return updated, nil
}

func normalizeOrderStatus(status models.OrderStatus) models.OrderStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case "placed", "pending":
		return models.OrderPending
	case "confirmed":
		return models.OrderConfirmed
	case "preparing":
		return models.OrderPreparing
	case "packed":
		return models.OrderPacked
	case "ready_for_pickup", "out_for_pickup":
		return models.OrderReadyForPickup
	case "out_for_delivery", "in_transit":
		return models.OrderOutForDelivery
	case "delivered":
		return models.OrderDelivered
	case "cancelled":
		return models.OrderCancelled
	case "rejected":
		return models.OrderRejected
	default:
		return ""
	}
}

func validOrderStatus(status models.OrderStatus) bool {
	switch status {
	case models.OrderPending, models.OrderConfirmed, models.OrderPreparing, models.OrderPacked,
		models.OrderReadyForPickup, models.OrderOutForDelivery, models.OrderDelivered,
		models.OrderCancelled, models.OrderRejected:
		return true
	default:
		return false
	}
}
func validOrderTransition(from, to models.OrderStatus) bool {
	if from == to {
		return false
	}
	switch from {
	case models.OrderPending:
		return to == models.OrderConfirmed || to == models.OrderCancelled || to == models.OrderRejected
	case models.OrderConfirmed:
		return to == models.OrderPreparing || to == models.OrderCancelled
	case models.OrderPreparing:
		return to == models.OrderPacked
	case models.OrderPacked:
		return to == models.OrderReadyForPickup
	case models.OrderReadyForPickup:
		return to == models.OrderOutForDelivery || to == models.OrderDelivered
	case models.OrderOutForDelivery:
		return to == models.OrderDelivered
	default:
		return false
	}
}

func validSellerTransition(from, to models.OrderStatus) bool {
	switch from {
	case models.OrderPending:
		return to == models.OrderConfirmed || to == models.OrderRejected
	case models.OrderConfirmed:
		return to == models.OrderPreparing
	case models.OrderPreparing:
		return to == models.OrderPacked
	case models.OrderPacked:
		return to == models.OrderReadyForPickup
	default:
		return false
	}
}

func validOrderDeliveryTransition(from, to models.OrderStatus) bool {
	return from == models.OrderReadyForPickup && to == models.OrderOutForDelivery ||
		from == models.OrderOutForDelivery && to == models.OrderDelivered
}

func (s *DeliveryService) List(ctx context.Context) ([]models.DeliveryAssignment, error) {
	return s.repo.ListDeliveries(ctx)
}
func (s *DeliveryService) Get(ctx context.Context, id string) (*models.DeliveryAssignment, error) {
	delivery, err := s.repo.GetDeliveryByID(ctx, id)
	if err != nil && isNotFound(err) {
		return nil, ErrNotFound
	}
	return delivery, err
}
func (s *DeliveryService) ForAgent(ctx context.Context, agentID string) ([]models.DeliveryAssignment, error) {
	return s.repo.ListAgentDeliveries(ctx, agentID)
}
func (s *DeliveryService) ForOrder(ctx context.Context, orderID string) (*models.DeliveryAssignment, error) {
	deliveries, err := s.repo.ListDeliveries(ctx)
	if err != nil {
		return nil, err
	}
	var latest *models.DeliveryAssignment
	for i := range deliveries {
		if deliveries[i].OrderID == orderID && (latest == nil || deliveries[i].CreatedAt > latest.CreatedAt) {
			assignment := deliveries[i]
			latest = &assignment
		}
	}
	return latest, nil
}

func (s *DeliveryService) Create(ctx context.Context, delivery models.DeliveryAssignment) error {
	if delivery.OrderID == "" || delivery.AgentID == "" || !validDeliveryStatus(delivery.Status) {
		return ErrInvalidInput
	}
	order, err := s.repo.GetOrderByID(ctx, delivery.OrderID)
	if err != nil {
		return ErrInvalidInput
	}
	if order.FulfillmentType != "delivery" || delivery.Status != "assigned" {
		return ErrInvalidInput
	}
	agent, err := s.repo.GetUserByID(ctx, delivery.AgentID)
	if err != nil {
		if isNotFound(err) {
			return ErrInvalidInput
		}
		return err
	}
	if agent.Role != models.RoleDeliveryAgent || !agent.Verified {
		return ErrInvalidInput
	}
	if err := translateRepositoryError(s.repo.CreateDelivery(ctx, delivery)); err != nil {
		return err
	}
	sendNotification(ctx, s.repo, createNotification(delivery.AgentID, "delivery_assignment", "New delivery assignment",
		fmt.Sprintf("Delivery %s is assigned to order %s.", delivery.ID, delivery.OrderID), delivery.ID))
	return nil
}
func (s *DeliveryService) UpdateStatus(ctx context.Context, id, status, actorID, actorRole string) (*models.DeliveryAssignment, error) {
	if !validDeliveryStatus(status) {
		return nil, ErrInvalidInput
	}
	delivery, err := s.repo.GetDeliveryByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !validDeliveryTransition(delivery.Status, status) {
		return nil, ErrConflict
	}
	if status == "in_transit" || status == "delivered" {
		order, err := s.repo.GetOrderByID(ctx, delivery.OrderID)
		if err != nil {
			return nil, translateRepositoryError(err)
		}
		if order.FulfillmentType != "delivery" {
			return nil, ErrConflict
		}
		if status == "in_transit" {
			if order.Status != models.OrderOutForDelivery {
				if _, err := s.orders.UpdateStatus(ctx, order.ID, models.OrderOutForDelivery, actorID, actorRole); err != nil {
					return nil, err
				}
			}
		} else {
			if order.Status == models.OrderReadyForPickup {
				if _, err := s.orders.UpdateStatus(ctx, order.ID, models.OrderOutForDelivery, actorID, actorRole); err != nil {
					return nil, err
				}
				order.Status = models.OrderOutForDelivery
			}
			if order.Status != models.OrderDelivered {
				if _, err := s.orders.UpdateStatus(ctx, order.ID, models.OrderDelivered, actorID, actorRole); err != nil {
					return nil, err
				}
			}
		}
	}
	updated, err := s.repo.UpdateDeliveryStatus(ctx, id, delivery.Status, status)
	if err != nil {
		return nil, translateRepositoryError(err)
	}
	if order, orderErr := s.repo.GetOrderByID(ctx, delivery.OrderID); orderErr == nil {
		sendNotification(ctx, s.repo, createNotification(order.CustomerID, "delivery_update", "Delivery updated",
			fmt.Sprintf("Delivery for order %s is now %s.", order.ID, strings.ReplaceAll(status, "_", " ")), order.ID))
	}
	return updated, nil
}
func validDeliveryStatus(status string) bool {
	switch status {
	case "assigned", "accepted", "picked_up", "in_transit", "delivered", "failed", "cancelled":
		return true
	default:
		return false
	}
}
func validDeliveryTransition(from, to string) bool {
	switch from {
	case "assigned":
		return to == "accepted" || to == "in_transit" || to == "cancelled"
	case "accepted":
		return to == "picked_up" || to == "cancelled"
	case "picked_up":
		return to == "in_transit" || to == "delivered" || to == "failed"
	case "in_transit":
		return to == "delivered" || to == "failed"
	default:
		return false
	}
}

func (s *AdminService) Analytics(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetAnalytics(ctx)
}

func (s *AdminService) Villages(ctx context.Context) ([]models.Village, error) {
	return s.repo.ListVillages(ctx)
}

func (s *AdminService) Categories(ctx context.Context) ([]models.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *AdminService) Products(ctx context.Context) ([]models.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *AdminService) Orders(ctx context.Context) ([]models.Order, error) {
	return s.repo.ListOrders(ctx)
}

func (s *AdminService) SetUserActive(ctx context.Context, id string, active bool) error {
	if _, err := s.repo.GetUserByID(ctx, strings.TrimSpace(id)); err != nil {
		return translateRepositoryError(err)
	}
	return translateRepositoryError(s.repo.SetUserActive(ctx, id, active))
}

func (s *AdminService) SetSellerState(ctx context.Context, id, action string) error {
	user, err := s.repo.GetUserByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return translateRepositoryError(err)
	}
	if user.Role != models.RoleSeller {
		return ErrInvalidInput
	}
	status, verified, active := "", user.Verified, user.IsActive()
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve", "verify":
		status, verified, active = "approved", true, true
	case "reject":
		status, verified, active = "rejected", false, false
	case "suspend":
		status, verified, active = "suspended", true, false
	default:
		return ErrInvalidInput
	}
	if err := translateRepositoryError(s.repo.SetSellerState(ctx, id, status, verified, active)); err != nil {
		return err
	}
	if !active {
		products, err := s.repo.ListSellerProducts(ctx, id)
		if err != nil {
			log.Printf("seller %s was suspended but products could not be listed for deactivation: %v", id, err)
			return nil
		}
		for _, product := range products {
			if err := s.repo.SetProductAvailability(ctx, product.ID, false); err != nil {
				log.Printf("seller %s was suspended but product %s could not be deactivated: %v", id, product.ID, err)
			}
		}
	}
	return nil
}

func (s *AdminService) CreateVillage(ctx context.Context, village models.Village) error {
	village.Name = strings.TrimSpace(village.Name)
	village.District = strings.TrimSpace(village.District)
	village.Taluk = strings.TrimSpace(village.Taluk)
	village.State = strings.TrimSpace(village.State)
	if err := validateVillage(village); err != nil {
		return err
	}
	all, err := s.repo.ListVillages(ctx)
	if err != nil {
		return err
	}
	for _, existing := range all {
		if strings.EqualFold(existing.Name, village.Name) && strings.EqualFold(existing.District, village.District) &&
			strings.EqualFold(existing.State, village.State) {
			return ErrConflict
		}
	}
	active := true
	village.Active = &active
	return translateRepositoryError(s.repo.CreateVillage(ctx, village))
}

func (s *AdminService) UpdateVillage(ctx context.Context, village models.Village) error {
	village.Name = strings.TrimSpace(village.Name)
	village.District = strings.TrimSpace(village.District)
	village.Taluk = strings.TrimSpace(village.Taluk)
	village.State = strings.TrimSpace(village.State)
	if err := validateVillage(village); err != nil {
		return err
	}
	all, err := s.repo.ListVillages(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, existing := range all {
		if existing.ID == village.ID {
			found = true
			if village.Active == nil {
				village.Active = existing.Active
			}
		} else if strings.EqualFold(existing.Name, village.Name) && strings.EqualFold(existing.District, village.District) &&
			strings.EqualFold(existing.State, village.State) {
			return ErrConflict
		}
	}
	if !found {
		return ErrNotFound
	}
	return translateRepositoryError(s.repo.UpdateVillage(ctx, village))
}

func (s *AdminService) SetVillageActive(ctx context.Context, id string, active bool) error {
	return translateRepositoryError(s.repo.SetVillageActive(ctx, id, active))
}

func validateVillage(village models.Village) error {
	if village.ID == "" || village.Name == "" || village.District == "" || village.State == "" ||
		len(village.Name) > 120 || len(village.District) > 120 || len(village.Taluk) > 120 || len(village.State) > 120 {
		return ErrInvalidInput
	}
	return nil
}

func (s *AdminService) CreateCategory(ctx context.Context, category models.Category) error {
	category.Name = strings.TrimSpace(category.Name)
	if category.ID == "" || category.Name == "" || len(category.Name) > 100 {
		return ErrInvalidInput
	}
	all, err := s.repo.ListCategories(ctx)
	if err != nil {
		return err
	}
	for _, existing := range all {
		if strings.EqualFold(existing.Name, category.Name) {
			return ErrConflict
		}
	}
	active := true
	category.Active = &active
	return translateRepositoryError(s.repo.CreateCategory(ctx, category))
}

func (s *AdminService) UpdateCategory(ctx context.Context, category models.Category) error {
	category.Name = strings.TrimSpace(category.Name)
	if category.ID == "" || category.Name == "" || len(category.Name) > 100 {
		return ErrInvalidInput
	}
	all, err := s.repo.ListCategories(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, existing := range all {
		if existing.ID == category.ID {
			found = true
			if category.Active == nil {
				category.Active = existing.Active
			}
		} else if strings.EqualFold(existing.Name, category.Name) {
			return ErrConflict
		}
	}
	if !found {
		return ErrNotFound
	}
	return translateRepositoryError(s.repo.UpdateCategory(ctx, category))
}

func (s *AdminService) SetCategoryActive(ctx context.Context, id string, active bool) error {
	return translateRepositoryError(s.repo.SetCategoryActive(ctx, id, active))
}

func isMissingUser(err error) bool {
	if errors.Is(err, repository.ErrNotFound) {
		return true
	}
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "user not found") ||
		strings.Contains(strings.ToLower(err.Error()), "no documents"))
}
func isNotFound(err error) bool {
	if errors.Is(err, repository.ErrNotFound) {
		return true
	}
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "not found") ||
		strings.Contains(strings.ToLower(err.Error()), "no documents"))
}

func translateRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, repository.ErrConflict):
		return ErrConflict
	default:
		return err
	}
}
