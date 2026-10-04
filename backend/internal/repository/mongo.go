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
	villages      []models.Village
	categories    []models.Category
	products      []models.Product
	orders        []models.Order
	reviews       []models.Review
	notifications []models.Notification
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
		villages:   append([]models.Village{}, models.DefaultVillages...),
		categories: append([]models.Category{}, models.DefaultCategories...),
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
	if err != nil {
		return err
	}
	_, err = s.DB.Collection("notifications").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "read", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("notifications_user_unread_created"),
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
		{collection: "users", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "role", Value: 1}},
			Options: options.Index().SetName("users_role"),
		}},
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "customerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("orders_customer_created"),
		}},
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "sellerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("orders_seller_created"),
		}},
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("orders_status_created"),
		}},
		{collection: "orders", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "items.productId", Value: 1}},
			Options: options.Index().SetName("orders_status_item_product"),
		}},
		{collection: "products", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "stock", Value: 1}, {Key: "available", Value: 1}},
			Options: options.Index().SetName("products_stock_availability"),
		}},
		{collection: "products", model: mongo.IndexModel{
			Keys:    bson.D{{Key: "sellerId", Value: 1}, {Key: "name", Value: 1}},
			Options: options.Index().SetName("products_seller_name"),
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
	if _, err := s.DB.Collection("reviews").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "orderId", Value: 1}, {Key: "productId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("reviews_order_product_unique"),
	}); err != nil {
		return err
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
		return append([]models.Village(nil), s.villages...), nil
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
		return append([]models.Category(nil), s.categories...), nil
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

func (s *Store) CreateVillage(ctx context.Context, village models.Village) error {
	if s.DB == nil || s.Fallback {
		for _, existing := range s.villages {
			if existing.ID == village.ID || (strings.EqualFold(existing.Name, village.Name) &&
				strings.EqualFold(existing.District, village.District) && strings.EqualFold(existing.State, village.State)) {
				return ErrConflict
			}
		}
		s.villages = append(s.villages, village)
		return nil
	}
	_, err := s.DB.Collection("villages").InsertOne(ctx, village)
	return mongoError(err)
}

func (s *Store) UpdateVillage(ctx context.Context, village models.Village) error {
	if s.DB == nil || s.Fallback {
		for i, existing := range s.villages {
			if existing.ID == village.ID {
				s.villages[i] = village
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("villages").ReplaceOne(ctx, bson.M{"_id": village.ID}, village)
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetVillageActive(ctx context.Context, id string, active bool) error {
	if s.DB == nil || s.Fallback {
		for i := range s.villages {
			if s.villages[i].ID == id {
				s.villages[i].Active = &active
				return nil
			}
		}
		return ErrNotFound
	}
	return updateActiveField(ctx, s.DB.Collection("villages"), id, active)
}

func (s *Store) CreateCategory(ctx context.Context, category models.Category) error {
	if s.DB == nil || s.Fallback {
		for _, existing := range s.categories {
			if existing.ID == category.ID || strings.EqualFold(existing.Name, category.Name) {
				return ErrConflict
			}
		}
		s.categories = append(s.categories, category)
		return nil
	}
	_, err := s.DB.Collection("categories").InsertOne(ctx, category)
	return mongoError(err)
}

func (s *Store) UpdateCategory(ctx context.Context, category models.Category) error {
	if s.DB == nil || s.Fallback {
		for i, existing := range s.categories {
			if existing.ID == category.ID {
				s.categories[i] = category
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("categories").ReplaceOne(ctx, bson.M{"_id": category.ID}, category)
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetCategoryActive(ctx context.Context, id string, active bool) error {
	if s.DB == nil || s.Fallback {
		for i := range s.categories {
			if s.categories[i].ID == id {
				s.categories[i].Active = &active
				return nil
			}
		}
		return ErrNotFound
	}
	return updateActiveField(ctx, s.DB.Collection("categories"), id, active)
}

func updateActiveField(ctx context.Context, collection *mongo.Collection, id string, active bool) error {
	result, err := collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"active": active}})
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
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

func (s *Store) SetUserActive(ctx context.Context, id string, active bool) error {
	if s.DB == nil || s.Fallback {
		for i := range s.users {
			if s.users[i].ID == id {
				s.users[i].Active = &active
				return nil
			}
		}
		return ErrNotFound
	}
	return updateActiveField(ctx, s.DB.Collection("users"), id, active)
}

func (s *Store) SetSellerState(ctx context.Context, id, status string, verified, active bool) error {
	if s.DB == nil || s.Fallback {
		for i := range s.users {
			if s.users[i].ID == id && s.users[i].Role == models.RoleSeller {
				s.users[i].SellerStatus = status
				s.users[i].Verified = verified
				s.users[i].Active = &active
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("users").UpdateOne(ctx,
		bson.M{"_id": id, "role": models.RoleSeller},
		bson.M{"$set": bson.M{"sellerStatus": status, "verified": verified, "active": active}})
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
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

func (s *Store) ListReviewsByOrder(ctx context.Context, orderID string) ([]models.Review, error) {
	return s.listReviews(ctx, bson.M{"orderId": orderID}, func() []models.Review {
		result := make([]models.Review, 0)
		for _, review := range s.reviews {
			if review.OrderID == orderID {
				result = append(result, review)
			}
		}
		return result
	})
}

func (s *Store) ListReviewsByProduct(ctx context.Context, productID string) ([]models.Review, error) {
	return s.listReviews(ctx, bson.M{"productId": productID}, func() []models.Review {
		result := make([]models.Review, 0)
		for _, review := range s.reviews {
			if review.ProductID == productID {
				result = append(result, review)
			}
		}
		return result
	})
}

func (s *Store) ListReviewsBySeller(ctx context.Context, sellerID string) ([]models.Review, error) {
	return s.listReviews(ctx, bson.M{"sellerId": sellerID}, func() []models.Review {
		result := make([]models.Review, 0)
		for _, review := range s.reviews {
			if review.SellerID == sellerID {
				result = append(result, review)
			}
		}
		return result
	})
}

func (s *Store) listReviews(ctx context.Context, filter bson.M, fallback func() []models.Review) ([]models.Review, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		return fallback(), nil
	}
	cursor, err := s.DB.Collection("reviews").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	reviews := make([]models.Review, 0)
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}

func (s *Store) CreateReview(ctx context.Context, review models.Review) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		for _, existing := range s.reviews {
			if existing.OrderID == review.OrderID && existing.ProductID == review.ProductID {
				return ErrConflict
			}
		}
		s.reviews = append(s.reviews, review)
		s.refreshFallbackRatings(review.ProductID, review.SellerID)
		return nil
	}
	if _, err := s.DB.Collection("reviews").InsertOne(ctx, review); err != nil {
		return mongoError(err)
	}
	productRating, productCount, err := s.averageReviewRating(ctx, "productId", review.ProductID, "productRating")
	if err != nil {
		return err
	}
	if _, err := s.DB.Collection("products").UpdateOne(ctx, bson.M{"_id": review.ProductID},
		bson.M{"$set": bson.M{"rating": productRating, "ratingCount": productCount}}); err != nil {
		return err
	}
	sellerRating, sellerCount, err := s.averageReviewRating(ctx, "sellerId", review.SellerID, "sellerRating")
	if err != nil {
		return err
	}
	if _, err := s.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": review.SellerID},
		bson.M{"$set": bson.M{"rating": sellerRating, "ratingCount": sellerCount}}); err != nil {
		return err
	}
	return nil
}

func (s *Store) averageReviewRating(ctx context.Context, key, id, ratingField string) (float64, int, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{key: id}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "average": bson.M{"$avg": "$" + ratingField}, "count": bson.M{"$sum": 1}}}},
	}
	cursor, err := s.DB.Collection("reviews").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)
	var result []struct {
		Average float64 `bson:"average"`
		Count   int     `bson:"count"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		return 0, 0, err
	}
	if len(result) == 0 {
		return 0, 0, nil
	}
	return result[0].Average, result[0].Count, nil
}

func (s *Store) refreshFallbackRatings(productID, sellerID string) {
	productTotal, productCount := 0, 0
	sellerTotal, sellerCount := 0, 0
	for _, review := range s.reviews {
		if review.ProductID == productID {
			productTotal += review.ProductRating
			productCount++
		}
		if review.SellerID == sellerID {
			sellerTotal += review.SellerRating
			sellerCount++
		}
	}
	for i := range s.products {
		if s.products[i].ID == productID && productCount > 0 {
			s.products[i].Rating = float64(productTotal) / float64(productCount)
			s.products[i].RatingCount = productCount
		}
	}
	for i := range s.users {
		if s.users[i].ID == sellerID && sellerCount > 0 {
			s.users[i].Rating = float64(sellerTotal) / float64(sellerCount)
			s.users[i].RatingCount = sellerCount
		}
	}
}

func (s *Store) CreateNotification(ctx context.Context, notification models.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		s.notifications = append(s.notifications, notification)
		return nil
	}
	_, err := s.DB.Collection("notifications").InsertOne(ctx, notification)
	return mongoError(err)
}

func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]models.Notification, error) {
	if userID == "" || limit < 1 || limit > 100 {
		return nil, ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		result := make([]models.Notification, 0)
		for i := len(s.notifications) - 1; i >= 0 && len(result) < limit; i-- {
			if s.notifications[i].UserID == userID {
				result = append(result, s.notifications[i])
			}
		}
		return result, nil
	}
	cursor, err := s.DB.Collection("notifications").Find(ctx, bson.M{"userId": userID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := make([]models.Notification, 0)
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) CountUnreadNotifications(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		var count int64
		for _, notification := range s.notifications {
			if notification.UserID == userID && !notification.Read {
				count++
			}
		}
		return count, nil
	}
	return s.DB.Collection("notifications").CountDocuments(ctx, bson.M{"userId": userID, "read": false})
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID string) error {
	if userID == "" || notificationID == "" {
		return ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		for i := range s.notifications {
			if s.notifications[i].ID == notificationID && s.notifications[i].UserID == userID {
				s.notifications[i].Read = true
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("notifications").UpdateOne(ctx,
		bson.M{"_id": notificationID, "userId": userID}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
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
							if !containsOrderID(s.products[productIndex].RestockedOrderIDs, order.ID) {
								s.products[productIndex].Stock += item.Quantity
								s.products[productIndex].RestockedOrderIDs = append(s.products[productIndex].RestockedOrderIDs, order.ID)
							}
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
		return s.RestoreOrderInventory(ctx, order.ID)
	}
	return &order, nil
}

func (s *Store) RestoreOrderInventory(ctx context.Context, id string) (*models.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.DB == nil || s.Fallback {
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		for orderIndex := range s.orders {
			order := &s.orders[orderIndex]
			if order.ID != id {
				continue
			}
			if order.InventoryRestored {
				clearFallbackRestockMarkers(s.products, order)
				clone := cloneOrder(*order)
				return &clone, nil
			}
			if !order.InventoryReserved || (order.Status != models.OrderCancelled && order.Status != models.OrderRejected) {
				return nil, ErrConflict
			}
			for _, item := range order.Items {
				for productIndex := range s.products {
					product := &s.products[productIndex]
					if product.ID == item.ProductID && !containsOrderID(product.RestockedOrderIDs, order.ID) {
						product.Stock += item.Quantity
						product.RestockedOrderIDs = append(product.RestockedOrderIDs, order.ID)
						break
					}
				}
			}
			order.InventoryRestored = true
			updateOrderPaymentState(order, &s.payments)
			clearFallbackRestockMarkers(s.products, order)
			clone := cloneOrder(*order)
			return &clone, nil
		}
		return nil, ErrNotFound
	}

	order, err := s.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.InventoryRestored {
		return order, nil
	}
	if !order.InventoryReserved || (order.Status != models.OrderCancelled && order.Status != models.OrderRejected) {
		return nil, ErrConflict
	}
	for _, item := range order.Items {
		result, updateErr := s.DB.Collection("products").UpdateOne(ctx,
			bson.M{"_id": item.ProductID, "restockedOrderIds": bson.M{"$ne": order.ID}},
			bson.M{"$inc": bson.M{"stock": item.Quantity}, "$addToSet": bson.M{"restockedOrderIds": order.ID}})
		if updateErr != nil {
			return nil, fmt.Errorf("restore stock for order %s product %s: %w", order.ID, item.ProductID, updateErr)
		}
		if result.MatchedCount == 0 {
			if _, lookupErr := s.GetProductByID(ctx, item.ProductID); lookupErr != nil && !errors.Is(lookupErr, ErrNotFound) {
				return nil, lookupErr
			}
		}
	}

	updateOrderPaymentState(order, nil)
	if _, err := s.DB.Collection("payments").UpdateOne(ctx, bson.M{"_id": order.PaymentID},
		bson.M{"$set": bson.M{"status": order.Payment.Status, "updatedAt": time.Now().UTC().Format(time.RFC3339)}}); err != nil {
		return nil, fmt.Errorf("update payment record for order %s: %w", order.ID, err)
	}
	result, err := s.DB.Collection("orders").UpdateOne(ctx,
		bson.M{"_id": order.ID, "status": order.Status, "inventoryRestored": bson.M{"$ne": true}},
		bson.M{"$set": bson.M{"inventoryRestored": true, "payment.status": order.Payment.Status}})
	if err != nil {
		return nil, fmt.Errorf("mark inventory restored for order %s: %w", order.ID, err)
	}
	if result.MatchedCount == 0 {
		current, lookupErr := s.GetOrderByID(ctx, id)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if current.InventoryRestored {
			if cleanupErr := s.clearMongoRestockMarkers(ctx, *current); cleanupErr != nil {
				log.Printf("stock restoration completed but marker cleanup failed: %v", cleanupErr)
			}
			return current, nil
		}
		return nil, ErrConflict
	}
	order.InventoryRestored = true
	if err := s.clearMongoRestockMarkers(ctx, *order); err != nil {
		log.Printf("stock restoration completed but marker cleanup failed: %v", err)
	}
	return order, nil
}

func (s *Store) clearMongoRestockMarkers(ctx context.Context, order models.Order) error {
	for _, item := range order.Items {
		if _, err := s.DB.Collection("products").UpdateOne(ctx, bson.M{"_id": item.ProductID},
			bson.M{"$pull": bson.M{"restockedOrderIds": order.ID}}); err != nil {
			return fmt.Errorf("clear stock restoration marker for order %s product %s: %w", order.ID, item.ProductID, err)
		}
	}
	return nil
}

func clearFallbackRestockMarkers(products []models.Product, order *models.Order) {
	for productIndex := range products {
		if !containsOrderID(products[productIndex].RestockedOrderIDs, order.ID) {
			continue
		}
		orderIDs := products[productIndex].RestockedOrderIDs[:0]
		for _, orderID := range products[productIndex].RestockedOrderIDs {
			if orderID != order.ID {
				orderIDs = append(orderIDs, orderID)
			}
		}
		products[productIndex].RestockedOrderIDs = orderIDs
	}
}

func containsOrderID(orderIDs []string, id string) bool {
	for _, orderID := range orderIDs {
		if orderID == id {
			return true
		}
	}
	return false
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
		s.cartMu.Lock()
		defer s.cartMu.Unlock()
		for i, existing := range s.products {
			if existing.ID == product.ID {
				if product.ExpectedStock != nil && existing.Stock != *product.ExpectedStock {
					return ErrConflict
				}
				product.History = append([]models.ProductChange(nil), existing.History...)
				if product.Change != nil {
					product.History = append(product.History, *product.Change)
					if len(product.History) > 100 {
						product.History = product.History[len(product.History)-100:]
					}
				}
				product.ExpectedStock = nil
				product.Change = nil
				s.products[i] = product
				return nil
			}
		}
		return errors.New("product not found")
	}
	filter := bson.M{"_id": product.ID}
	if product.ExpectedStock != nil {
		filter["stock"] = *product.ExpectedStock
	}
	set := bson.M{
		"name": product.Name, "category": product.Category, "price": product.Price,
		"rating": product.Rating, "unit": product.Unit, "village": product.Village,
		"villageId": product.VillageID, "sellerId": product.SellerID, "seller": product.Seller,
		"available": product.Available, "stock": product.Stock, "description": product.Description,
	}
	update := bson.M{"$set": set}
	if product.Change != nil {
		update["$push"] = bson.M{"history": bson.M{"$each": []models.ProductChange{*product.Change}, "$slice": -100}}
	}
	result, err := s.DB.Collection("products").UpdateOne(ctx, filter, update)
	if err != nil {
		return mongoError(err)
	}
	if result.MatchedCount == 0 {
		if _, lookupErr := s.GetProductByID(ctx, product.ID); lookupErr != nil {
			return lookupErr
		}
		return ErrConflict
	}
	return nil
}

func (s *Store) SetProductAvailability(ctx context.Context, id string, available bool) error {
	if s.DB == nil || s.Fallback {
		for i := range s.products {
			if s.products[i].ID == id {
				s.products[i].Available = available && s.products[i].Stock > 0
				return nil
			}
		}
		return ErrNotFound
	}
	result, err := s.DB.Collection("products").UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"available": available}})
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
	if s.DB == nil || s.Fallback {
		return s.fallbackAnalytics(ctx)
	}
	users, err := s.DB.Collection("users").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	sellers, err := s.DB.Collection("users").CountDocuments(ctx, bson.M{"role": models.RoleSeller})
	if err != nil {
		return nil, err
	}
	agents, err := s.DB.Collection("users").CountDocuments(ctx, bson.M{"role": models.RoleDeliveryAgent})
	if err != nil {
		return nil, err
	}
	products, err := s.DB.Collection("products").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	orders, err := s.DB.Collection("orders").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	// Aggregate delivered seller subtotal rather than item totals including delivery fees.
	revenueRows, err := s.aggregateRows(ctx, "orders", mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.OrderDelivered}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "revenue": bson.M{"$sum": "$subtotal"}, "deliveredOrders": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	var revenue float64
	var deliveredOrders int64
	if len(revenueRows) > 0 {
		revenue = analyticsFloat(revenueRows[0]["revenue"])
		deliveredOrders = analyticsInt64(revenueRows[0]["deliveredOrders"])
	}

	topProducts, err := s.aggregateRows(ctx, "orders", mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.OrderDelivered}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$group", Value: bson.M{"_id": "$items.productId", "name": bson.M{"$first": "$items.name"}, "unitsSold": bson.M{"$sum": "$items.quantity"}, "revenue": bson.M{"$sum": "$items.lineTotal"}}}},
		{{Key: "$sort", Value: bson.D{{Key: "unitsSold", Value: -1}}}},
		{{Key: "$limit", Value: 5}},
		{{Key: "$project", Value: bson.M{"_id": 0, "productId": "$_id", "name": 1, "unitsSold": 1, "revenue": 1}}},
	})
	if err != nil {
		return nil, err
	}

	salesByVillage, err := s.aggregateRows(ctx, "orders", mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.OrderDelivered}}},
		{{Key: "$group", Value: bson.M{"_id": "$villageId", "revenue": bson.M{"$sum": "$subtotal"}, "orders": bson.M{"$sum": 1}}}},
		{{Key: "$lookup", Value: bson.M{"from": "villages", "localField": "_id", "foreignField": "_id", "as": "village"}}},
		{{Key: "$unwind", Value: bson.M{"path": "$village", "preserveNullAndEmptyArrays": true}}},
		{{Key: "$project", Value: bson.M{"_id": 0, "villageId": "$_id", "name": bson.M{"$ifNull": bson.A{"$village.name", "$_id"}}, "revenue": 1, "orders": 1}}},
		{{Key: "$sort", Value: bson.D{{Key: "revenue", Value: -1}}}},
	})
	if err != nil {
		return nil, err
	}

	salesByCategory, err := s.aggregateRows(ctx, "orders", mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.OrderDelivered}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$lookup", Value: bson.M{"from": "products", "localField": "items.productId", "foreignField": "_id", "as": "product"}}},
		{{Key: "$unwind", Value: bson.M{"path": "$product", "preserveNullAndEmptyArrays": true}}},
		{{Key: "$set", Value: bson.M{"categoryName": bson.M{"$ifNull": bson.A{"$product.category", "Uncategorized"}}}}},
		{{Key: "$group", Value: bson.M{"_id": "$categoryName", "unitsSold": bson.M{"$sum": "$items.quantity"}, "revenue": bson.M{"$sum": "$items.lineTotal"}}}},
		{{Key: "$project", Value: bson.M{"_id": 0, "category": "$_id", "unitsSold": 1, "revenue": 1}}},
		{{Key: "$sort", Value: bson.D{{Key: "revenue", Value: -1}}}},
	})
	if err != nil {
		return nil, err
	}

	lowStockProducts := make([]models.Product, 0)
	cursor, err := s.DB.Collection("products").Find(ctx, bson.M{"stock": bson.M{"$lte": 5}},
		options.Find().SetSort(bson.D{{Key: "stock", Value: 1}, {Key: "name", Value: 1}}).SetLimit(10))
	if err != nil {
		return nil, err
	}
	if err := cursor.All(ctx, &lowStockProducts); err != nil {
		cursor.Close(ctx)
		return nil, err
	}
	cursor.Close(ctx)

	salesByDay, err := s.salesOverTime(ctx, 10)
	if err != nil {
		return nil, err
	}
	salesByMonth, err := s.salesOverTime(ctx, 7)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"users": users, "sellers": sellers, "deliveryAgents": agents,
		"products": products, "orders": orders, "revenue": revenue,
		"deliveredOrders": deliveredOrders, "topSellingProducts": topProducts,
		"salesByVillage": salesByVillage, "salesByCategory": salesByCategory,
		"lowStockProducts": lowStockProducts, "salesByDay": salesByDay, "salesByMonth": salesByMonth,
	}, nil
}

func (s *Store) aggregateRows(ctx context.Context, collection string, pipeline mongo.Pipeline) ([]bson.M, error) {
	cursor, err := s.DB.Collection(collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	rows := make([]bson.M, 0)
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) salesOverTime(ctx context.Context, dateLength int32) ([]bson.M, error) {
	dateExpression := bson.M{"$substrBytes": bson.A{"$createdAt", 0, dateLength}}
	return s.aggregateRows(ctx, "orders", mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.OrderDelivered, "createdAt": bson.M{"$type": "string"}}}},
		{{Key: "$group", Value: bson.M{"_id": dateExpression, "revenue": bson.M{"$sum": "$subtotal"}, "orders": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	})
}

func analyticsFloat(value interface{}) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int32:
		return float64(number)
	case int64:
		return float64(number)
	default:
		return 0
	}
}

func analyticsInt64(value interface{}) int64 {
	switch number := value.(type) {
	case int32:
		return int64(number)
	case int64:
		return number
	case float64:
		return int64(number)
	default:
		return 0
	}
}

func (s *Store) fallbackAnalytics(ctx context.Context) (map[string]interface{}, error) {
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
	villages, _ := s.ListVillages(ctx)
	villageNames := make(map[string]string, len(villages))
	for _, village := range villages {
		villageNames[village.ID] = village.Name
	}
	productByID := make(map[string]models.Product, len(products))
	for _, product := range products {
		productByID[product.ID] = product
	}
	byProduct := map[string]map[string]interface{}{}
	byVillage := map[string]map[string]interface{}{}
	byCategory := map[string]map[string]interface{}{}
	byDay := map[string]map[string]interface{}{}
	byMonth := map[string]map[string]interface{}{}
	var revenue float64
	var delivered int
	for _, order := range orders {
		if order.Status != models.OrderDelivered {
			continue
		}
		delivered++
		revenue += order.Subtotal
		villageName := villageNames[order.VillageID]
		if villageName == "" {
			villageName = order.VillageID
		}
		addAnalyticsGroup(byVillage, order.VillageID, "name", villageName, "orders", 1, "revenue", order.Subtotal)
		if len(order.CreatedAt) >= 10 {
			addAnalyticsGroup(byDay, order.CreatedAt[:10], "date", order.CreatedAt[:10], "orders", 1, "revenue", order.Subtotal)
		}
		if len(order.CreatedAt) >= 7 {
			addAnalyticsGroup(byMonth, order.CreatedAt[:7], "month", order.CreatedAt[:7], "orders", 1, "revenue", order.Subtotal)
		}
		for _, item := range order.Items {
			product := productByID[item.ProductID]
			addAnalyticsGroup(byProduct, item.ProductID, "name", item.Name, "unitsSold", item.Quantity, "revenue", item.LineTotal)
			addAnalyticsGroup(byCategory, product.Category, "category", product.Category, "unitsSold", item.Quantity, "revenue", item.LineTotal)
		}
	}
	lowStock := make([]models.Product, 0)
	for _, product := range products {
		if product.Stock <= 5 {
			lowStock = append(lowStock, product)
		}
	}
	sort.Slice(lowStock, func(i, j int) bool { return lowStock[i].Stock < lowStock[j].Stock })
	if len(lowStock) > 10 {
		lowStock = lowStock[:10]
	}
	topProducts := analyticsGroupValues(byProduct, "unitsSold")
	if len(topProducts) > 5 {
		topProducts = topProducts[:5]
	}
	stats := map[string]interface{}{
		"users": len(users), "sellers": 0, "deliveryAgents": 0,
		"products": len(products), "orders": len(orders), "revenue": revenue, "deliveredOrders": delivered,
		"topSellingProducts": topProducts, "salesByVillage": analyticsGroupValues(byVillage, "revenue"),
		"salesByCategory": analyticsGroupValues(byCategory, "revenue"), "lowStockProducts": lowStock,
		"salesByDay": analyticsGroupValues(byDay, "date"), "salesByMonth": analyticsGroupValues(byMonth, "month"),
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

func addAnalyticsGroup(groups map[string]map[string]interface{}, id, labelKey string, label interface{}, countKey string, count int, amountKey string, amount float64) {
	group, exists := groups[id]
	if !exists {
		group = map[string]interface{}{labelKey: label, countKey: 0, amountKey: 0.0}
		groups[id] = group
	}
	group[countKey] = group[countKey].(int) + count
	group[amountKey] = group[amountKey].(float64) + amount
}

func analyticsGroupValues(groups map[string]map[string]interface{}, sortKey string) []map[string]interface{} {
	values := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		values = append(values, group)
	}
	sort.Slice(values, func(i, j int) bool {
		if sortKey == "date" || sortKey == "month" {
			return values[i][sortKey].(string) < values[j][sortKey].(string)
		}
		left, right := analyticsFloat(values[i][sortKey]), analyticsFloat(values[j][sortKey])
		if left == right {
			return fmt.Sprint(values[i]["name"]) < fmt.Sprint(values[j]["name"])
		}
		return left > right
	})
	return values
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
