package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"villageconnect/internal/auth"
	"villageconnect/internal/middleware"
	"villageconnect/internal/models"
	"villageconnect/internal/search"
	"villageconnect/internal/services"
)

type API struct {
	Users      *services.UserService
	Catalog    *services.CatalogService
	Orders     *services.OrderService
	Deliveries *services.DeliveryService
	Admin      *services.AdminService
	Search     *search.Service
	JWTSecret  string
}

func NewAPI(users *services.UserService, catalog *services.CatalogService, orders *services.OrderService,
	deliveries *services.DeliveryService, admin *services.AdminService, productSearch *search.Service, secret string) *API {
	return &API{
		Users: users, Catalog: catalog, Orders: orders, Deliveries: deliveries,
		Admin: admin, Search: productSearch, JWTSecret: secret,
	}
}

func (api *API) Health(databaseStatus func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		success(c, http.StatusOK, gin.H{
			"message":  "VillageConnect API is running",
			"database": databaseStatus(),
		})
	}
}

func NotImplemented(resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"message": resource + " API is not implemented",
		})
	}
}

func (api *API) ListVillages(c *gin.Context) {
	result, err := api.Catalog.Villages(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"villages": result})
}

func (api *API) ListCategories(c *gin.Context) {
	result, err := api.Catalog.Categories(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"categories": result})
}

func (api *API) ListProducts(c *gin.Context) {
	query, err := productQueryFromRequest(c, "name_asc")
	if err != nil {
		invalid(c, "Invalid marketplace filters")
		return
	}
	result, err := api.Catalog.Products(c.Request.Context(), query)
	if err != nil {
		writeError(c, err)
		return
	}
	writeProductPage(c, result, nil)
}

func (api *API) GetProduct(c *gin.Context) {
	product, err := api.Catalog.Product(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"product": product})
}

func (api *API) SearchProducts(c *gin.Context) {
	query, err := productQueryFromRequest(c, "relevance")
	if err != nil {
		invalid(c, "Invalid search filters")
		return
	}
	result, suggestions, err := api.Search.SearchProducts(c.Request.Context(), query)
	if err != nil {
		writeError(c, err)
		return
	}
	writeProductPage(c, result, suggestions)
}

func (api *API) SearchSellers(c *gin.Context) {
	sellers, err := api.Search.SearchSellers(c.Request.Context(), c.Query("q"))
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"sellers": sellers})
}

func (api *API) ReindexSearch(c *gin.Context) {
	if err := api.Search.Reindex(c.Request.Context()); err != nil {
		log.Printf("manual search reindex failed: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false, "message": "Search reindex failed; MongoDB data is unchanged",
		})
		return
	}
	success(c, http.StatusOK, gin.H{"message": "Product and seller search indexes rebuilt"})
}

func (api *API) Register(c *gin.Context) {
	var payload struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		Password  string `json:"password"`
		VillageID string `json:"villageId"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid registration payload")
		return
	}
	user, err := api.Users.Register(c.Request.Context(), payload.Name, payload.Email, payload.Phone,
		payload.Password, payload.VillageID, payload.Role)
	if err != nil {
		writeError(c, err)
		return
	}
	token, claims, err := auth.GenerateSessionToken(api.JWTSecret, *user, services.SessionLifetime)
	if err != nil {
		serverError(c, err)
		return
	}
	if err := api.Users.CreateSession(c.Request.Context(), claims); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusCreated, gin.H{"token": token, "user": publicUser(*user)})
}

func (api *API) Login(c *gin.Context) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid login payload")
		return
	}
	user, err := api.Users.Authenticate(c.Request.Context(), payload.Email, payload.Password)
	if err != nil {
		writeError(c, err)
		return
	}
	token, claims, err := auth.GenerateSessionToken(api.JWTSecret, *user, services.SessionLifetime)
	if err != nil {
		serverError(c, err)
		return
	}
	if err := api.Users.CreateSession(c.Request.Context(), claims); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"token": token, "user": publicUser(*user)})
}

func (api *API) Logout(c *gin.Context) {
	claims, ok := middleware.AuthenticatedClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Authentication required"})
		return
	}
	if err := api.Users.RevokeSession(c.Request.Context(), claims); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"message": "Session revoked"})
}

func (api *API) Me(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	user, err := api.Users.GetByID(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"user": publicUser(*user)})
}

func (api *API) ListUsers(c *gin.Context) {
	users, err := api.Users.List(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	for i := range users {
		users[i] = publicUser(users[i])
	}
	success(c, http.StatusOK, gin.H{"users": users})
}

func (api *API) ListOrders(c *gin.Context) {
	userID, role := middleware.Identity(c)
	var orders []models.Order
	var err error
	switch role {
	case string(models.RoleCustomer):
		orders, err = api.Orders.ListForCustomer(c.Request.Context(), userID)
	case string(models.RoleSeller):
		orders, err = api.Orders.ListForSeller(c.Request.Context(), userID)
	case string(models.RoleAdmin):
		orders, err = api.Orders.List(c.Request.Context())
	default:
		forbidden(c)
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"orders": orders})
}

func (api *API) SellerOrders(c *gin.Context) {
	userID, role := middleware.Identity(c)
	if role != string(models.RoleSeller) && role != string(models.RoleAdmin) {
		forbidden(c)
		return
	}
	var orders []models.Order
	var err error
	if role == string(models.RoleAdmin) {
		orders, err = api.Orders.List(c.Request.Context())
	} else {
		orders, err = api.Orders.ListForSeller(c.Request.Context(), userID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"orders": orders})
}

func (api *API) SellerOrder(c *gin.Context) {
	userID, role := middleware.Identity(c)
	order, err := api.Orders.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if role != string(models.RoleAdmin) && order.SellerID != userID {
		forbidden(c)
		return
	}
	success(c, http.StatusOK, gin.H{"order": order})
}

func (api *API) GetOrder(c *gin.Context) {
	userID, role := middleware.Identity(c)
	order, err := api.Orders.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if !canViewOrder(role, userID, *order) {
		forbidden(c)
		return
	}
	success(c, http.StatusOK, gin.H{"order": order})
}

func (api *API) GetOrderTracking(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	order, err := api.Orders.Tracking(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"tracking": gin.H{
		"orderId": order.ID, "status": order.Status, "fulfillmentType": order.FulfillmentType,
		"statusHistory": order.StatusHistory, "updatedAt": order.UpdatedAt,
	}})
}

func (api *API) CreateOrder(c *gin.Context) {
	var payload struct {
		VillageID       string `json:"villageId"`
		FulfillmentType string `json:"fulfillmentType"`
		Address         string `json:"address"`
		PaymentMethod   string `json:"paymentMethod"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid checkout payload")
		return
	}
	userID, _ := middleware.Identity(c)
	createdOrder, err := api.Orders.Checkout(c.Request.Context(), userID,
		strings.TrimSpace(payload.VillageID), strings.TrimSpace(payload.FulfillmentType),
		payload.Address, strings.TrimSpace(payload.PaymentMethod))
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusCreated, gin.H{"order": createdOrder})
}

func (api *API) GetCart(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	cart, err := api.Orders.GetCart(c.Request.Context(), userID, c.DefaultQuery("fulfillmentType", "delivery"))
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"cart": cart})
}

func (api *API) AddCartItem(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	var payload struct {
		ProductID string `json:"productId"`
		VillageID string `json:"villageId"`
		Quantity  int    `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid cart item")
		return
	}
	if err := api.Orders.AddCartItem(c.Request.Context(), userID, payload.ProductID,
		payload.VillageID, payload.Quantity); err != nil {
		writeError(c, err)
		return
	}
	cart, err := api.Orders.GetCart(c.Request.Context(), userID, "delivery")
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"cart": cart})
}

func (api *API) UpdateCartItem(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	var payload struct {
		Quantity int `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid cart quantity")
		return
	}
	if err := api.Orders.SetCartItemQuantity(c.Request.Context(), userID, c.Param("productId"), payload.Quantity); err != nil {
		writeError(c, err)
		return
	}
	cart, err := api.Orders.GetCart(c.Request.Context(), userID, "delivery")
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"cart": cart})
}

func (api *API) RemoveCartItem(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	if err := api.Orders.RemoveCartItem(c.Request.Context(), userID, c.Param("productId")); err != nil {
		writeError(c, err)
		return
	}
	cart, err := api.Orders.GetCart(c.Request.Context(), userID, "delivery")
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"cart": cart})
}

func (api *API) ClearCart(c *gin.Context) {
	userID, _ := middleware.Identity(c)
	if err := api.Orders.ClearCart(c.Request.Context(), userID); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"cart": models.CartView{Items: []models.CartLine{}}})
}

func (api *API) UpdateOrderStatus(c *gin.Context) {
	userID, role := middleware.Identity(c)
	var payload struct {
		Status models.OrderStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid order status payload")
		return
	}
	updated, err := api.Orders.UpdateStatus(c.Request.Context(), c.Param("id"), payload.Status, userID, role)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"order": updated})
}

func (api *API) ListDeliveries(c *gin.Context) {
	deliveries, err := api.Deliveries.List(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"deliveries": deliveries})
}

func (api *API) GetDelivery(c *gin.Context) {
	userID, role := middleware.Identity(c)
	delivery, err := api.Deliveries.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if role != string(models.RoleAdmin) && delivery.AgentID != userID {
		forbidden(c)
		return
	}
	success(c, http.StatusOK, gin.H{"delivery": delivery})
}

func (api *API) CreateDelivery(c *gin.Context) {
	var payload struct {
		ID         string `json:"id"`
		OrderID    string `json:"orderId"`
		AgentID    string `json:"agentId"`
		Status     string `json:"status"`
		PickupCode string `json:"pickupCode"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid delivery payload")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if payload.Status == "" {
		payload.Status = "assigned"
	}
	delivery := models.DeliveryAssignment{
		ID: payload.ID, OrderID: strings.TrimSpace(payload.OrderID),
		AgentID: strings.TrimSpace(payload.AgentID), Status: payload.Status,
		PickupCode: payload.PickupCode, CreatedAt: now, UpdatedAt: now,
	}
	if delivery.ID == "" {
		delivery.ID = fmt.Sprintf("d%d", time.Now().UnixNano())
	}
	if err := api.Deliveries.Create(c.Request.Context(), delivery); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusCreated, gin.H{"delivery": delivery})
}

func (api *API) UpdateDeliveryStatus(c *gin.Context) {
	userID, role := middleware.Identity(c)
	var payload struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid delivery status payload")
		return
	}
	delivery, err := api.Deliveries.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if role != string(models.RoleAdmin) && (role != string(models.RoleDeliveryAgent) || delivery.AgentID != userID) {
		forbidden(c)
		return
	}
	updated, err := api.Deliveries.UpdateStatus(c.Request.Context(), c.Param("id"), payload.Status, userID, role)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"delivery": updated})
}

func (api *API) AgentDeliveries(c *gin.Context) {
	userID, role := middleware.Identity(c)
	agentID := c.Param("agentId")
	if role != string(models.RoleAdmin) && agentID != userID {
		forbidden(c)
		return
	}
	deliveries, err := api.Deliveries.ForAgent(c.Request.Context(), agentID)
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"deliveries": deliveries})
}

func (api *API) Sellers(c *gin.Context) {
	users, err := api.Users.List(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	sellers := make([]gin.H, 0)
	for _, user := range users {
		if user.Role == models.RoleSeller && user.Verified {
			sellers = append(sellers, sellerProfile(user))
		}
	}
	success(c, http.StatusOK, gin.H{"sellers": sellers})
}

func (api *API) GetSeller(c *gin.Context) {
	user, err := api.Users.GetByID(c.Request.Context(), c.Param("sellerId"))
	if err != nil {
		writeError(c, err)
		return
	}
	if user.Role != models.RoleSeller || !user.Verified {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Seller not found"})
		return
	}
	success(c, http.StatusOK, gin.H{"seller": sellerProfile(*user)})
}

func (api *API) SellerProducts(c *gin.Context) {
	userID, role := middleware.Identity(c)
	sellerID := c.Param("sellerId")
	if role != string(models.RoleAdmin) && (role != string(models.RoleSeller) || sellerID != userID) {
		forbidden(c)
		return
	}
	products, err := api.Catalog.SellerProducts(c.Request.Context(), sellerID)
	if err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"products": products})
}

func (api *API) CreateProduct(c *gin.Context) {
	userID, role := middleware.Identity(c)
	user, err := api.Users.GetByID(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		invalid(c, "Invalid product payload")
		return
	}
	if role == string(models.RoleSeller) {
		if !user.Verified {
			forbidden(c)
			return
		}
		product.SellerID = user.ID
		product.Seller = user.Name
	} else if role != string(models.RoleAdmin) {
		forbidden(c)
		return
	} else {
		seller, err := api.Users.GetByID(c.Request.Context(), product.SellerID)
		if err != nil {
			writeError(c, err)
			return
		}
		if seller.Role != models.RoleSeller || !seller.Verified {
			invalid(c, "A verified seller account is required")
			return
		}
		product.Seller = seller.Name
	}
	if product.ID != "" {
		invalid(c, "Product ID cannot be supplied when creating a product")
		return
	}
	product.ID = fmt.Sprintf("p%d", time.Now().UnixNano())
	if err := api.Catalog.CreateProduct(c.Request.Context(), product); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusCreated, gin.H{"product": product})
}

func (api *API) UpdateProduct(c *gin.Context) {
	userID, role := middleware.Identity(c)
	existing, err := api.Catalog.Product(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	var payload models.Product
	if err := c.ShouldBindJSON(&payload); err != nil {
		invalid(c, "Invalid product update payload")
		return
	}
	if role != string(models.RoleAdmin) &&
		(role != string(models.RoleSeller) || existing.SellerID != userID) {
		forbidden(c)
		return
	}
	product := *existing
	product.Name = payload.Name
	product.Category = payload.Category
	product.Price = payload.Price
	product.Rating = payload.Rating
	product.Unit = payload.Unit
	product.Village = payload.Village
	product.VillageID = payload.VillageID
	if product.VillageID == "" && product.Village == existing.Village {
		product.VillageID = existing.VillageID
	}
	product.Available = payload.Available
	product.Stock = payload.Stock
	product.Description = payload.Description
	if err := api.Catalog.UpdateProduct(c.Request.Context(), product); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"product": product})
}

func (api *API) DeleteProduct(c *gin.Context) {
	userID, role := middleware.Identity(c)
	product, err := api.Catalog.Product(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	if role != string(models.RoleAdmin) &&
		(role != string(models.RoleSeller) || product.SellerID != userID) {
		forbidden(c)
		return
	}
	if err := api.Catalog.DeleteProduct(c.Request.Context(), product.ID); err != nil {
		writeError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"message": "Product deleted"})
}

func (api *API) AdminAnalytics(c *gin.Context) {
	stats, err := api.Admin.Analytics(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	success(c, http.StatusOK, gin.H{"stats": stats})
}

func canViewOrder(role, userID string, order models.Order) bool {
	return role == string(models.RoleAdmin) ||
		(role == string(models.RoleCustomer) && order.CustomerID == userID) ||
		(role == string(models.RoleSeller) && order.SellerID == userID)
}

func publicUser(user models.User) models.User {
	user.PasswordHash = ""
	return user
}

func sellerProfile(user models.User) gin.H {
	return gin.H{
		"id": user.ID, "name": user.Name, "villageId": user.VillageID,
		"verified": user.Verified,
	}
}

func productQueryFromRequest(c *gin.Context, defaultSort string) (models.ProductQuery, error) {
	query := models.ProductQuery{
		Query: c.Query("q"), Category: c.Query("category"), VillageID: c.Query("villageId"),
		Sort: c.DefaultQuery("sort", defaultSort), Page: 1, PageSize: 8,
	}
	for name, destination := range map[string]**float64{
		"minPrice": &query.MinPrice,
		"maxPrice": &query.MaxPrice,
	} {
		if raw := c.Query(name); raw != "" {
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return models.ProductQuery{}, err
			}
			*destination = &value
		}
	}
	if raw := c.Query("available"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return models.ProductQuery{}, err
		}
		query.Available = &value
	}
	for name, destination := range map[string]*int{
		"page":     &query.Page,
		"pageSize": &query.PageSize,
	} {
		if raw := c.Query(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return models.ProductQuery{}, err
			}
			*destination = value
		}
	}
	return query, nil
}

func writeProductPage(c *gin.Context, result models.ProductPage, suggestions []string) {
	fields := gin.H{
		"products": result.Products,
		"pagination": gin.H{
			"total": result.Total, "page": result.Page,
			"pageSize": result.PageSize, "totalPages": result.TotalPages,
		},
	}
	if suggestions != nil {
		fields["suggestions"] = suggestions
	}
	success(c, http.StatusOK, fields)
}

func success(c *gin.Context, status int, fields gin.H) {
	fields["success"] = true
	c.JSON(status, fields)
}

func invalid(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
}

func forbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Insufficient permissions"})
}

func serverError(c *gin.Context, err error) {
	log.Printf("request failed: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error"})
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrForbidden):
		forbidden(c)
	case errors.Is(err, services.ErrInvalidInput):
		invalid(c, "Invalid request data")
	case errors.Is(err, search.ErrInvalidQuery):
		invalid(c, "Invalid search query or filters")
	case errors.Is(err, services.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Request conflicts with current resource state"})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid credentials"})
	case errors.Is(err, services.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Resource not found"})
	default:
		serverError(c, err)
	}
}
