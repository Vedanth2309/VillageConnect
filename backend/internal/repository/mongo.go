package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"villageconnect/internal/config"
	"villageconnect/internal/models"
)

// Store wraps the MongoDB client plus a light fallback strategy for local development.
type Store struct {
	DB            *mongo.Database
	Fallback      bool
	databaseName  string
	mongoURI      string
	collectionMap map[string]string
	users         []models.User
	products      []models.Product
	orders        []models.Order
	payments      []models.Payment
	deliveries    []models.DeliveryAssignment
	carts         map[string]models.Cart
	cartMu        sync.Mutex
	sessionMu     sync.RWMutex
	sessions      map[string]models.AuthSession
}

func NewStore(ctx context.Context, cfg config.Config) (*Store, error) {
	store := &Store{
		databaseName: cfg.MongoDatabase,
		mongoURI:     cfg.MongoURI,
		collectionMap: map[string]string{
			"villages":   "villages",
			"categories": "categories",
			"products":   "products",
			"users":      "users",
			"orders":     "orders",
			"payments":   "payments",
			"deliveries": "deliveries",
			"carts":      "carts",
		},
		users:      append([]models.User{}, models.DefaultUsers...),
		products:   append([]models.Product{}, models.DefaultProducts...),
		orders:     append([]models.Order{}, models.DefaultOrders...),
		deliveries: append([]models.DeliveryAssignment{}, models.DefaultDeliveries...),
		carts:      make(map[string]models.Cart),
		sessions:   make(map[string]models.AuthSession),
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		store.Fallback = true
		return store, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		store.Fallback = true
		return store, err
	}

	store.DB = client.Database(cfg.MongoDatabase)
	if err := store.ensureSeedData(ctx); err != nil {
		_ = client.Disconnect(ctx)
		store.DB = nil
		store.Fallback = true
		return store, err
	}
	if err := store.ensureAuthIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		store.DB = nil
		store.Fallback = true
		return store, err
	}
	if err := store.ensureMarketplaceIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		store.DB = nil
		store.Fallback = true
		return store, err
	}
	if err := store.migrateLegacyOrderStatuses(ctx); err != nil {
		_ = client.Disconnect(ctx)
		store.DB = nil
		store.Fallback = true
		return store, err
	}

	return store, nil
}

func (s *Store) ensureAuthIndexes(ctx context.Context) error {
	_, err := s.DB.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
		Options: options.Index().
			SetUnique(true).
			SetName("users_email_unique").
			SetCollation(&options.Collation{Locale: "en", Strength: 2}),
	})
	if err != nil {
		return err
	}
	_, err = s.DB.Collection("auth_sessions").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expiresAt", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0).SetName("auth_sessions_expiry"),
	})
	return err
}

func (s *Store) ensureMarketplaceIndexes(ctx context.Context) error {
	_, err := s.DB.Collection("products").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "villageId", Value: 1},
			{Key: "category", Value: 1},
			{Key: "available", Value: 1},
			{Key: "price", Value: 1},
		},
		Options: options.Index().SetName("products_marketplace_filters"),
	})
	if err != nil {
		return err
	}
	_, err = s.DB.Collection("villages").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "state", Value: 1},
			{Key: "district", Value: 1},
			{Key: "taluk", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetName("villages_location_hierarchy"),
	})
	if err != nil {
		return err
	}
	_, err = s.DB.Collection("carts").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "customerId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("carts_customer_unique"),
	})
	if err != nil {
		return err
	}
	for _, index := range []struct {
		collection string
		model      mongo.IndexModel
	}{
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("orders_customer_created"),
		}},
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "sellerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("orders_seller_created"),
		}},
		{collection: "payments", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "orderId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("payments_order_unique"),
		}},
	} {
		if _, err := s.DB.Collection(index.collection).Indexes().CreateOne(ctx, index.model); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrateLegacyOrderStatuses(ctx context.Context) error {
	for oldStatus, newStatus := range map[string]models.OrderStatus{
		"placed":         models.OrderPending,
		"out_for_pickup": models.OrderReadyForPickup,
		"in_transit":     models.OrderOutForDelivery,
	} {
		if _, err := s.DB.Collection("orders").UpdateMany(ctx, bson.M{"status": oldStatus},
			bson.M{"$set": bson.M{"status": newStatus}}); err != nil {
			return fmt.Errorf("migrate order status %q: %w", oldStatus, err)
		}
	}
	return nil
}

func (s *Store) ListVillages(ctx context.Context) ([]models.Village, error) {
	if s.DB == nil || s.Fallback {
		return models.DefaultVillages, nil
	}

	cursor, err := s.DB.Collection("villages").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	villages := make([]models.Village, 0)
	if err := cursor.All(ctx, &villages); err != nil {
		return nil, err
	}
	return villages, nil
}

func (s *Store) ListCategories(ctx context.Context) ([]models.Category, error) {
	if s.DB == nil || s.Fallback {
		return models.DefaultCategories, nil
	}

	cursor, err := s.DB.Collection("categories").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	categories := make([]models.Category, 0)
	if err := cursor.All(ctx, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func (s *Store) ListProducts(ctx context.Context) ([]models.Product, error) {
	if s.DB == nil || s.Fallback {
		products := make([]models.Product, len(s.products))
		copy(products, s.products)
		return products, nil
	}

	cursor, err := s.DB.Collection("products").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	products := make([]models.Product, 0)
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (s *Store) ListMarketplaceProducts(ctx context.Context, query models.ProductQuery) (models.ProductPage, error) {
	if s.DB == nil || s.Fallback {
		matches := make([]models.Product, 0)
		for _, product := range s.products {
			if query.VillageID != "" && product.VillageID != query.VillageID &&
				!(query.VillageName != "" && product.VillageID == "" &&
					strings.EqualFold(product.Village, query.VillageName)) {
				continue
			}
			if query.Category != "" && !strings.EqualFold(product.Category, query.Category) {
				continue
			}
			if query.MinPrice != nil && product.Price < *query.MinPrice {
				continue
			}
			if query.MaxPrice != nil && product.Price > *query.MaxPrice {
				continue
			}
			if query.Available != nil {
				isAvailable := product.Available && product.Stock > 0
				if *query.Available != isAvailable {
					continue
				}
			}
			needle := strings.ToLower(strings.TrimSpace(query.Query))
			if needle != "" && !strings.Contains(strings.ToLower(product.Name+" "+product.Description+" "+product.Category+" "+product.Seller), needle) {
				continue
			}
			matches = append(matches, product)
		}
		sortProducts(matches, query.Sort)
		return paginateProducts(matches, query)
	}

	conditions := bson.A{}
	if query.VillageID != "" {
		villageMatches := bson.A{bson.M{"villageId": query.VillageID}}
		if query.VillageName != "" {
			villageMatches = append(villageMatches,
				bson.M{"village": query.VillageName, "villageId": bson.M{"$exists": false}},
				bson.M{"village": query.VillageName, "villageId": ""})
		}
		conditions = append(conditions, bson.M{"$or": villageMatches})
	}
	if query.Category != "" {
		conditions = append(conditions, bson.M{"category": query.Category})
	}
	if query.MinPrice != nil || query.MaxPrice != nil {
		price := bson.M{}
		if query.MinPrice != nil {
			price["$gte"] = *query.MinPrice
		}
		if query.MaxPrice != nil {
			price["$lte"] = *query.MaxPrice
		}
		conditions = append(conditions, bson.M{"price": price})
	}
	if query.Available != nil {
		if *query.Available {
			conditions = append(conditions, bson.M{"available": true, "stock": bson.M{"$gt": 0}})
		} else {
			conditions = append(conditions, bson.M{"$or": bson.A{
				bson.M{"available": false},
				bson.M{"stock": bson.M{"$lte": 0}},
			}})
		}
	}
	if strings.TrimSpace(query.Query) != "" {
		pattern := regexp.QuoteMeta(strings.TrimSpace(query.Query))
		conditions = append(conditions, bson.M{"$or": bson.A{
			bson.M{"name": bson.M{"$regex": pattern, "$options": "i"}},
			bson.M{"description": bson.M{"$regex": pattern, "$options": "i"}},
			bson.M{"category": bson.M{"$regex": pattern, "$options": "i"}},
			bson.M{"seller": bson.M{"$regex": pattern, "$options": "i"}},
		}})
	}
	filter := bson.M{}
	if len(conditions) == 1 {
		filter = conditions[0].(bson.M)
	} else if len(conditions) > 1 {
		filter["$and"] = conditions
	}

	total, err := s.DB.Collection("products").CountDocuments(ctx, filter)
	if err != nil {
		return models.ProductPage{}, err
	}
	sortField := bson.D{{Key: "name", Value: 1}, {Key: "_id", Value: 1}}
	switch query.Sort {
	case "price_asc":
		sortField = bson.D{{Key: "price", Value: 1}, {Key: "_id", Value: 1}}
	case "price_desc":
		sortField = bson.D{{Key: "price", Value: -1}, {Key: "_id", Value: 1}}
	case "rating_desc":
		sortField = bson.D{{Key: "rating", Value: -1}, {Key: "name", Value: 1}}
	case "name_desc":
		sortField = bson.D{{Key: "name", Value: -1}, {Key: "_id", Value: 1}}
	}
	cursor, err := s.DB.Collection("products").Find(ctx, filter, options.Find().
		SetSort(sortField).
		SetSkip(int64((query.Page-1)*query.PageSize)).
		SetLimit(int64(query.PageSize)))
	if err != nil {
		return models.ProductPage{}, err
	}
	defer cursor.Close(ctx)
	products := make([]models.Product, 0)
	if err := cursor.All(ctx, &products); err != nil {
		return models.ProductPage{}, err
	}
	pages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	return models.ProductPage{Products: products, Total: total, Page: query.Page,
		PageSize: query.PageSize, TotalPages: pages}, nil
}

func sortProducts(products []models.Product, mode string) {
	sort.SliceStable(products, func(i, j int) bool {
		left, right := products[i], products[j]
		switch mode {
		case "price_asc":
			if left.Price != right.Price {
				return left.Price < right.Price
			}
		case "price_desc":
			if left.Price != right.Price {
				return left.Price > right.Price
			}
		case "rating_desc":
			if left.Rating != right.Rating {
				return left.Rating > right.Rating
			}
		case "name_desc":
			if left.Name != right.Name {
				return left.Name > right.Name
			}
		default:
			if left.Name != right.Name {
				return left.Name < right.Name
			}
		}
		return left.ID < right.ID
	})
}

func paginateProducts(products []models.Product, query models.ProductQuery) (models.ProductPage, error) {
	total := int64(len(products))
	start := (query.Page - 1) * query.PageSize
	if start > len(products) {
		start = len(products)
	}
	end := start + query.PageSize
	if end > len(products) {
		end = len(products)
	}
	pages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	return models.ProductPage{Products: products[start:end], Total: total, Page: query.Page,
		PageSize: query.PageSize, TotalPages: pages}, nil
}

func (s *Store) GetProductByID(ctx context.Context, id string) (*models.Product, error) {
	if s.DB == nil || s.Fallback {
		for _, product := range s.products {
			if product.ID == id {
				result := product
				return &result, nil
			}
		}
		return nil, errors.New("product not found")
	}

	var product models.Product
	if err := s.DB.Collection("products").FindOne(ctx, bson.M{"_id": id}).Decode(&product); err != nil {
		return nil, mongoError(err)
	}
	return &product, nil
}

func (s *Store) SearchProducts(ctx context.Context, query string) ([]models.Product, error) {
	query = strings.TrimSpace(query)
	if s.DB == nil || s.Fallback {
		if query == "" {
			products := make([]models.Product, len(s.products))
			copy(products, s.products)
			return products, nil
		}

		needle := strings.ToLower(query)
		matches := make([]models.Product, 0)
		for _, product := range s.products {
			fields := []string{
				strings.ToLower(product.Name),
				strings.ToLower(product.Description),
				strings.ToLower(product.Category),
				strings.ToLower(product.Village),
				strings.ToLower(product.Seller),
			}
			for _, field := range fields {
				if strings.Contains(field, needle) {
					matches = append(matches, product)
					break
				}
			}
		}
		return matches, nil
	}

	if query == "" {
		return s.ListProducts(ctx)
	}

	pattern := fmt.Sprintf(".*%s.*", regexp.QuoteMeta(strings.ToLower(query)))
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": pattern, "$options": "i"}},
			{"description": bson.M{"$regex": pattern, "$options": "i"}},
			{"category": bson.M{"$regex": pattern, "$options": "i"}},
			{"village": bson.M{"$regex": pattern, "$options": "i"}},
			{"seller": bson.M{"$regex": pattern, "$options": "i"}},
		},
	}

	cursor, err := s.DB.Collection("products").Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	products := make([]models.Product, 0)
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]models.User, error) {
	if s.DB == nil || s.Fallback {
		users := make([]models.User, len(s.users))
		copy(users, s.users)
		for i := range users {
			users[i].PasswordHash = ""
		}
		return users, nil
	}

	cursor, err := s.DB.Collection("users").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	users := make([]models.User, 0)
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	for i := range users {
		users[i].PasswordHash = ""
	}
	return users, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.GetUserByEmailForAuth(ctx, email)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Store) GetUserByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	if s.DB == nil || s.Fallback {
		for _, user := range s.users {
			if strings.EqualFold(user.Email, email) {
				clone := user
				return &clone, nil
			}
		}
		return nil, errors.New("user not found")
	}

	var user models.User
	findOptions := options.FindOne().SetCollation(&options.Collation{Locale: "en", Strength: 2})
	if err := s.DB.Collection("users").FindOne(ctx, bson.M{"email": email}, findOptions).Decode(&user); err != nil {
		return nil, mongoError(err)
	}
	return &user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	if s.DB == nil || s.Fallback {
		for _, user := range s.users {
			if user.ID == id {
				clone := user
				clone.PasswordHash = ""
				return &clone, nil
			}
		}
		return nil, errors.New("user not found")
	}

	var user models.User
	if err := s.DB.Collection("users").FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		return nil, mongoError(err)
	}
	user.PasswordHash = ""
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, user models.User) error {
	if s.DB == nil || s.Fallback {
		s.users = append(s.users, user)
		return nil
	}
	_, err := s.DB.Collection("users").InsertOne(ctx, user)
	return mongoError(err)
}

func (s *Store) CreateSession(ctx context.Context, session models.AuthSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.sessionMu.Lock()
		defer s.sessionMu.Unlock()
		if _, exists := s.sessions[session.ID]; exists {
			return ErrConflict
		}
		s.sessions[session.ID] = session
		return nil
	}
	_, err := s.DB.Collection("auth_sessions").InsertOne(ctx, session)
	return mongoError(err)
}

func (s *Store) GetSession(ctx context.Context, id string) (*models.AuthSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DB == nil || s.Fallback {
		s.sessionMu.RLock()
		defer s.sessionMu.RUnlock()
		session, exists := s.sessions[id]
		if !exists {
			return nil, ErrNotFound
		}
		clone := session
		return &clone, nil
	}
	var session models.AuthSession
	if err := s.DB.Collection("auth_sessions").FindOne(ctx, bson.M{"_id": id}).Decode(&session); err != nil {
		return nil, mongoError(err)
	}
	return &session, nil
}

func (s *Store) RevokeSession(ctx context.Context, id string, revokedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.sessionMu.Lock()
		defer s.sessionMu.Unlock()
		session, exists := s.sessions[id]
		if !exists {
			return ErrNotFound
		}
		if session.RevokedAt == nil {
			session.RevokedAt = &revokedAt
			s.sessions[id] = session
		}
		return nil
	}
	result, err := s.DB.Collection("auth_sessions").UpdateOne(ctx,
		bson.M{"_id": id, "revokedAt": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revokedAt": revokedAt}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		session, getErr := s.GetSession(ctx, id)
		if getErr != nil {
			return getErr
		}
		if session.RevokedAt != nil {
			return nil
		}
	}
	return nil
}

func (s *Store) ListOrders(ctx context.Context) ([]models.Order, error) {
	if s.DB == nil || s.Fallback {
		orders := make([]models.Order, len(s.orders))
		for i := range s.orders {
			orders[i] = cloneOrder(s.orders[i])
		}
		return orders, nil
	}

	cursor, err := s.DB.Collection("orders").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	orders := make([]models.Order, 0)
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *Store) GetOrderByID(ctx context.Context, id string) (*models.Order, error) {
	if s.DB == nil || s.Fallback {
		for _, order := range s.orders {
			if order.ID == id {
				clone := cloneOrder(order)
				return &clone, nil
			}
		}
		return nil, errors.New("order not found")
	}

	var order models.Order
	if err := s.DB.Collection("orders").FindOne(ctx, bson.M{"_id": id}).Decode(&order); err != nil {
		return nil, mongoError(err)
	}
	return &order, nil
}

func cloneOrder(order models.Order) models.Order {
	order.Items = append([]models.OrderItem(nil), order.Items...)
	order.StatusHistory = append([]models.OrderStatusEvent(nil), order.StatusHistory...)
	return order
}

func (s *Store) ListOrderPayments(ctx context.Context) ([]models.Payment, error) {
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		return append([]models.Payment(nil), s.payments...), nil
	}
	cursor, err := s.DB.Collection("payments").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	payments := make([]models.Payment, 0)
	if err := cursor.All(ctx, &payments); err != nil {
		return nil, err
	}
	return payments, nil
}

func (s *Store) GetCart(ctx context.Context, customerID string) (*models.Cart, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		cart, exists := s.carts[customerID]
		if !exists {
			return nil, ErrNotFound
		}
		clone := cart
		clone.Items = append([]models.CartItem(nil), cart.Items...)
		return &clone, nil
	}
	var cart models.Cart
	if err := s.DB.Collection("carts").FindOne(ctx, bson.M{"customerId": customerID}).Decode(&cart); err != nil {
		return nil, mongoError(err)
	}
	return &cart, nil
}

func (s *Store) SaveCart(ctx context.Context, cart models.Cart, expectedRevision string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cart.CustomerID == "" {
		return errors.New("cart customer id is required")
	}
	cart.Revision = fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		if s.carts == nil {
			s.carts = make(map[string]models.Cart)
		}
		current, exists := s.carts[cart.CustomerID]
		if expectedRevision == "" && exists || expectedRevision != "" && (!exists || current.Revision != expectedRevision) {
			return ErrConflict
		}
		cart.Items = append([]models.CartItem(nil), cart.Items...)
		s.carts[cart.CustomerID] = cart
		return nil
	}
	cart.ID = "cart:" + cart.CustomerID
	filter := bson.M{"customerId": cart.CustomerID}
	if expectedRevision == "" {
		filter["revision"] = bson.M{"$exists": false}
	} else {
		filter["revision"] = expectedRevision
	}
	result, err := s.DB.Collection("carts").ReplaceOne(ctx, filter,
		cart, options.Replace().SetUpsert(true))
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) ClearCart(ctx context.Context, customerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		delete(s.carts, customerID)
		return nil
	}
	_, err := s.DB.Collection("carts").DeleteOne(ctx, bson.M{"customerId": customerID})
	return err
}

func (s *Store) CheckoutOrder(ctx context.Context, order models.Order, payment models.Payment, expectedCartRevision string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		cart, exists := s.carts[order.CustomerID]
		if !exists || cart.Revision != expectedCartRevision {
			return ErrConflict
		}
		for _, existingPayment := range s.payments {
			if existingPayment.OrderID == payment.OrderID {
				return ErrConflict
			}
		}
		reserved := make([]models.OrderItem, 0, len(order.Items))
		for _, item := range order.Items {
			index := -1
			for i := range s.products {
				if s.products[i].ID == item.ProductID {
					index = i
					break
				}
			}
			if index < 0 || !s.products[index].Available || s.products[index].Stock < item.Quantity {
				for _, reservation := range reserved {
					for i := range s.products {
						if s.products[i].ID == reservation.ProductID {
							s.products[i].Stock += reservation.Quantity
							break
						}
					}
				}
				return ErrConflict
			}
			s.products[index].Stock -= item.Quantity
			reserved = append(reserved, item)
		}
		s.orders = append(s.orders, cloneOrder(order))
		s.payments = append(s.payments, payment)
		delete(s.carts, order.CustomerID)
		return nil
	}

	products := s.DB.Collection("products")
	reserved := make([]models.OrderItem, 0, len(order.Items))
	rollbackCtx, cancelRollback := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRollback()
	rollbackStock := func() error {
		var rollbackErr error
		for _, item := range reserved {
			_, err := products.UpdateOne(rollbackCtx, bson.M{"_id": item.ProductID},
				bson.M{"$inc": bson.M{"stock": item.Quantity}})
			if err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore stock for %s: %w", item.ProductID, err))
			}
		}
		return rollbackErr
	}

	for _, item := range order.Items {
		result, err := products.UpdateOne(ctx,
			bson.M{"_id": item.ProductID, "available": true, "stock": bson.M{"$gte": item.Quantity}},
			bson.M{"$inc": bson.M{"stock": -item.Quantity}})
		if err != nil {
			rollbackErr := logCheckoutCompensation(rollbackStock())
			return errors.Join(fmt.Errorf("reserve stock for %s: %w", item.ProductID, err), rollbackErr)
		}
		if result.MatchedCount == 0 {
			rollbackErr := logCheckoutCompensation(rollbackStock())
			return errors.Join(ErrConflict, rollbackErr)
		}
		reserved = append(reserved, item)
	}

	if _, err := s.DB.Collection("orders").InsertOne(ctx, order); err != nil {
		rollbackErr := logCheckoutCompensation(rollbackStock())
		return errors.Join(mongoError(err), rollbackErr)
	}
	if _, err := s.DB.Collection("payments").InsertOne(ctx, payment); err != nil {
		_, deleteOrderErr := s.DB.Collection("orders").DeleteOne(rollbackCtx, bson.M{"_id": order.ID})
		rollbackErr := logCheckoutCompensation(errors.Join(deleteOrderErr, rollbackStock()))
		return errors.Join(mongoError(err), rollbackErr)
	}
	cartResult, err := s.DB.Collection("carts").DeleteOne(ctx,
		bson.M{"customerId": order.CustomerID, "revision": expectedCartRevision})
	if err != nil || cartResult.DeletedCount != 1 {
		_, deletePaymentErr := s.DB.Collection("payments").DeleteOne(rollbackCtx, bson.M{"_id": payment.ID})
		_, deleteOrderErr := s.DB.Collection("orders").DeleteOne(rollbackCtx, bson.M{"_id": order.ID})
		rollbackErr := logCheckoutCompensation(errors.Join(deletePaymentErr, deleteOrderErr, rollbackStock()))
		if err != nil {
			return errors.Join(fmt.Errorf("clear cart after order creation: %w", err), rollbackErr)
		}
		return errors.Join(ErrConflict, rollbackErr)
	}
	return nil
}

func logCheckoutCompensation(err error) error {
	if err != nil {
		log.Printf("checkout compensation failed: %v", err)
	}
	return err
}

func (s *Store) TransitionOrder(ctx context.Context, id string, from, to models.OrderStatus, actorID string, event models.OrderStatusEvent) (*models.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	event.Status = to
	event.ActorID = actorID
	event.CreatedAt = now
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		for i := range s.orders {
			order := &s.orders[i]
			if order.ID != id {
				continue
			}
			if order.Status != from {
				return nil, ErrConflict
			}
			restock := (to == models.OrderCancelled || to == models.OrderRejected) &&
				order.InventoryReserved && !order.InventoryRestored
			order.Status = to
			order.UpdatedAt = now
			order.StatusHistory = append(order.StatusHistory, event)
			if restock && !order.InventoryRestored {
				for _, item := range order.Items {
					for productIndex := range s.products {
						if s.products[productIndex].ID == item.ProductID {
							s.products[productIndex].Stock += item.Quantity
							break
						}
					}
				}
				order.InventoryRestored = true
			}
			if restock {
				updateOrderPaymentState(order, &s.payments)
			}
			clone := cloneOrder(*order)
			return &clone, nil
		}
		return nil, ErrNotFound
	}

	currentOrder, err := s.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if currentOrder.Status != from {
		return nil, ErrConflict
	}
	restock := (to == models.OrderCancelled || to == models.OrderRejected) &&
		currentOrder.InventoryReserved && !currentOrder.InventoryRestored
	filter := bson.M{"_id": id, "status": from}
	if restock {
		filter["inventoryRestored"] = bson.M{"$ne": true}
	}
	update := bson.M{
		"$set":  bson.M{"status": to, "updatedAt": now},
		"$push": bson.M{"statusHistory": event},
	}
	if restock {
		update["$set"].(bson.M)["inventoryRestored"] = true
	}
	var order models.Order
	err = s.DB.Collection("orders").FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&order)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			_, lookupErr := s.GetOrderByID(ctx, id)
			if lookupErr == nil {
				return nil, ErrConflict
			}
		}
		return nil, mongoError(err)
	}
	if restock {
		for _, item := range order.Items {
			if _, err := s.DB.Collection("products").UpdateOne(ctx, bson.M{"_id": item.ProductID},
				bson.M{"$inc": bson.M{"stock": item.Quantity}}); err != nil {
				return nil, fmt.Errorf("restore stock for cancelled order %s: %w", order.ID, err)
			}
		}
		updateOrderPaymentState(&order, nil)
		if _, err := s.DB.Collection("orders").UpdateOne(ctx, bson.M{"_id": order.ID},
			bson.M{"$set": bson.M{"payment.status": order.Payment.Status}}); err != nil {
			return nil, fmt.Errorf("update payment state for cancelled order %s: %w", order.ID, err)
		}
		if _, err := s.DB.Collection("payments").UpdateOne(ctx, bson.M{"_id": order.PaymentID},
			bson.M{"$set": bson.M{"status": order.Payment.Status, "updatedAt": now}}); err != nil {
			return nil, fmt.Errorf("update payment record for cancelled order %s: %w", order.ID, err)
		}
	}
	return &order, nil
}

func updateOrderPaymentState(order *models.Order, payments *[]models.Payment) {
	paymentStatus := "cancelled"
	if order.Payment.Status == "paid" {
		paymentStatus = "refunded"
	}
	order.Payment.Status = paymentStatus
	if payments == nil {
		return
	}
	for i := range *payments {
		if (*payments)[i].ID == order.PaymentID || (*payments)[i].OrderID == order.ID {
			(*payments)[i].Status = paymentStatus
			(*payments)[i].UpdatedAt = order.UpdatedAt
		}
	}
}

func (s *Store) CreateProduct(ctx context.Context, product models.Product) error {
	if s.DB == nil || s.Fallback {
		for i, existing := range s.products {
			if existing.ID == product.ID {
				s.products[i] = product
				return nil
			}
		}
		s.products = append(s.products, product)
		return nil
	}
	_, err := s.DB.Collection("products").InsertOne(ctx, product)
	if err != nil {
		return mongoError(err)
	}
	return nil
}

func (s *Store) UpdateProduct(ctx context.Context, product models.Product) error {
	if s.DB == nil || s.Fallback {
		for i, existing := range s.products {
			if existing.ID == product.ID {
				s.products[i] = product
				return nil
			}
		}
		return errors.New("product not found")
	}
	result, err := s.DB.Collection("products").ReplaceOne(ctx, bson.M{"_id": product.ID}, product)
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteProduct(ctx context.Context, id string) error {
	if s.DB == nil || s.Fallback {
		for i, product := range s.products {
			if product.ID == id {
				s.products = append(s.products[:i], s.products[i+1:]...)
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("products").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return mongoError(err)
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListSellerProducts(ctx context.Context, seller string) ([]models.Product, error) {
	if seller == "" {
		return nil, errors.New("seller id is required")
	}
	if s.DB == nil || s.Fallback {
		filtered := make([]models.Product, 0)
		for _, product := range s.products {
			if product.SellerID == seller {
				filtered = append(filtered, product)
			}
		}
		return filtered, nil
	}

	cursor, err := s.DB.Collection("products").Find(ctx, bson.M{"sellerId": seller})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	products := make([]models.Product, 0)
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (s *Store) GetAnalytics(ctx context.Context) (map[string]interface{}, error) {
	users, err := s.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	products, err := s.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := s.ListOrders(ctx)
	if err != nil {
		return nil, err
	}

	revenue := 0.0
	for _, order := range orders {
		revenue += order.Total
	}

	stats := map[string]interface{}{
		"users":          len(users),
		"sellers":        0,
		"deliveryAgents": 0,
		"products":       len(products),
		"orders":         len(orders),
		"revenue":        revenue,
	}
	for _, user := range users {
		switch user.Role {
		case models.RoleSeller:
			stats["sellers"] = stats["sellers"].(int) + 1
		case models.RoleDeliveryAgent:
			stats["deliveryAgents"] = stats["deliveryAgents"].(int) + 1
		}
	}
	return stats, nil
}

func (s *Store) ListDeliveries(ctx context.Context) ([]models.DeliveryAssignment, error) {
	if s.DB == nil || s.Fallback {
		deliveries := make([]models.DeliveryAssignment, len(s.deliveries))
		copy(deliveries, s.deliveries)
		return deliveries, nil
	}

	cursor, err := s.DB.Collection("deliveries").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	deliveries := make([]models.DeliveryAssignment, 0)
	if err := cursor.All(ctx, &deliveries); err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (s *Store) GetDeliveryByID(ctx context.Context, id string) (*models.DeliveryAssignment, error) {
	if s.DB == nil || s.Fallback {
		for _, delivery := range s.deliveries {
			if delivery.ID == id {
				clone := delivery
				return &clone, nil
			}
		}
		return nil, ErrNotFound
	}

	var delivery models.DeliveryAssignment
	if err := s.DB.Collection("deliveries").FindOne(ctx, bson.M{"_id": id}).Decode(&delivery); err != nil {
		return nil, mongoError(err)
	}
	return &delivery, nil
}

func (s *Store) ListAgentDeliveries(ctx context.Context, agentID string) ([]models.DeliveryAssignment, error) {
	deliveries, err := s.ListDeliveries(ctx)
	if err != nil {
		return nil, err
	}
	if agentID == "" {
		return deliveries, nil
	}
	filtered := make([]models.DeliveryAssignment, 0)
	for _, delivery := range deliveries {
		if delivery.AgentID == agentID {
			filtered = append(filtered, delivery)
		}
	}
	return filtered, nil
}

func (s *Store) CreateDelivery(ctx context.Context, assignment models.DeliveryAssignment) error {
	if s.DB == nil || s.Fallback {
		s.deliveries = append(s.deliveries, assignment)
		return nil
	}
	_, err := s.DB.Collection("deliveries").InsertOne(ctx, assignment)
	return mongoError(err)
}

func (s *Store) UpdateDeliveryStatus(ctx context.Context, id, fromStatus, status string) (*models.DeliveryAssignment, error) {
	if s.DB == nil || s.Fallback {
		for i, delivery := range s.deliveries {
			if delivery.ID == id {
				if delivery.Status != fromStatus {
					return nil, ErrConflict
				}
				s.deliveries[i].Status = status
				s.deliveries[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				clone := s.deliveries[i]
				return &clone, nil
			}
		}
		return nil, errors.New("delivery not found")
	}

	update := bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().UTC().Format(time.RFC3339)}}
	result, err := s.DB.Collection("deliveries").UpdateOne(ctx, bson.M{"_id": id, "status": fromStatus}, update)
	if err != nil {
		return nil, mongoError(err)
	}
	if result.MatchedCount == 0 {
		if _, err := s.GetDeliveryByID(ctx, id); err != nil {
			return nil, err
		}
		return nil, ErrConflict
	}
	return s.GetDeliveryByID(ctx, id)
}

func (s *Store) HealthStatus() string {
	if s.DB == nil || s.Fallback {
		return "fallback"
	}
	return "mongodb"
}

func (s *Store) ensureSeedData(ctx context.Context) error {
	if s.DB == nil {
		return nil
	}

	collections := map[string]interface{}{
		"villages":   models.DefaultVillages,
		"categories": models.DefaultCategories,
		"products":   models.DefaultProducts,
		"users":      models.DefaultUsers,
		"orders":     models.DefaultOrders,
		"deliveries": models.DefaultDeliveries,
	}

	for name, docs := range collections {
		collection := s.DB.Collection(name)
		count, err := collection.EstimatedDocumentCount(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		if _, err := collection.InsertMany(ctx, toInterfaces(docs)); err != nil {
			return err
		}
	}

	if err := s.ensureSeedSellerProfiles(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureSeedSellerProfiles(ctx context.Context) error {
	for _, product := range models.DefaultProducts {
		filter := bson.M{
			"_id": product.ID, "seller": product.Seller,
			"$or": []bson.M{
				{"sellerId": bson.M{"$exists": false}},
				{"sellerId": ""},
			},
		}
		var existing models.Product
		err := s.DB.Collection("products").FindOne(ctx, filter).Decode(&existing)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return err
		}
		var seller models.User
		for _, candidate := range models.DefaultUsers {
			if candidate.ID == product.SellerID {
				seller = candidate
				break
			}
		}
		if seller.ID == "" {
			return fmt.Errorf("default product %s has no corresponding seller seed", product.ID)
		}
		if _, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": seller.ID},
			bson.M{"$setOnInsert": seller}, options.Update().SetUpsert(true)); err != nil {
			return err
		}
		if _, err := s.DB.Collection("products").UpdateOne(ctx, filter,
			bson.M{"$set": bson.M{"sellerId": product.SellerID}}); err != nil {
			return err
		}
	}
	return nil
}

func toInterfaces(items interface{}) []interface{} {
	slice := make([]interface{}, 0)
	switch value := items.(type) {
	case []models.Village:
		for _, item := range value {
			slice = append(slice, item)
		}
	case []models.Category:
		for _, item := range value {
			slice = append(slice, item)
		}
	case []models.Product:
		for _, item := range value {
			slice = append(slice, item)
		}
	case []models.User:
		for _, item := range value {
			slice = append(slice, item)
		}
	case []models.Order:
		for _, item := range value {
			slice = append(slice, item)
		}
	case []models.DeliveryAssignment:
		for _, item := range value {
			slice = append(slice, item)
		}
	default:
		return slice
	}
	return slice
}

func ProvideSeedTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
