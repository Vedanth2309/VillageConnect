package routes

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"villageconnect/internal/handlers"
	"villageconnect/internal/middleware"
	"villageconnect/internal/models"
)

func New(api *handlers.API, databaseStatus func() string, frontendURL string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.HandleMethodNotAllowed = true
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Route not found"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"success": false, "message": "Method not allowed"})
	})

	allowedOrigins := []string{frontendURL, "http://localhost:3000", "http://127.0.0.1:5173", "http://localhost:4173", "http://localhost"}
	if extraOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); extraOrigins != "" {
		for _, origin := range strings.Split(extraOrigins, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	apiGroup := router.Group("/api")
	apiGroup.GET("/health", api.Health(databaseStatus))

	authRoutes := apiGroup.Group("/auth")
	authRoutes.POST("/register", api.Register)
	authRoutes.POST("/login", api.Login)
	requireAuth := middleware.RequireAuth(api.JWTSecret, api.Users)
	authRoutes.GET("/me", requireAuth, api.Me)
	authRoutes.POST("/logout", requireAuth, api.Logout)
	apiGroup.GET("/users/me", requireAuth, api.Me)

	apiGroup.GET("/villages", api.ListVillages)
	apiGroup.GET("/categories", api.ListCategories)

	products := apiGroup.Group("/products")
	products.GET("", api.ListProducts)
	products.GET("/:id", api.GetProduct)
	products.POST("", requireAuth, middleware.RequireRoles(string(models.RoleSeller), string(models.RoleAdmin)), api.CreateProduct)
	products.PUT("/:id", requireAuth, middleware.RequireRoles(string(models.RoleSeller), string(models.RoleAdmin)), api.UpdateProduct)
	products.DELETE("/:id", requireAuth, middleware.RequireRoles(string(models.RoleSeller), string(models.RoleAdmin)), api.DeleteProduct)

	searchRoutes := apiGroup.Group("/search")
	searchRoutes.GET("/products", api.SearchProducts)
	searchRoutes.GET("/sellers", api.SearchSellers)

	sellers := apiGroup.Group("/sellers")
	sellers.GET("", api.Sellers)
	sellers.GET("/:sellerId", api.GetSeller)
	sellers.GET("/:sellerId/products", requireAuth,
		middleware.RequireRoles(string(models.RoleSeller), string(models.RoleAdmin)), api.SellerProducts)
	sellerOrders := apiGroup.Group("/seller/orders", requireAuth, middleware.RequireRoles(string(models.RoleSeller), string(models.RoleAdmin)))
	sellerOrders.GET("", api.SellerOrders)
	sellerOrders.GET("/:id", api.SellerOrder)
	sellerOrders.PATCH("/:id/status", api.UpdateOrderStatus)

	orders := apiGroup.Group("/orders")
	orders.Use(requireAuth)
	orders.GET("", middleware.RequireRoles(string(models.RoleCustomer), string(models.RoleSeller), string(models.RoleAdmin)), api.ListOrders)
	orders.GET("/:id", middleware.RequireRoles(string(models.RoleCustomer), string(models.RoleSeller), string(models.RoleAdmin)), api.GetOrder)
	orders.GET("/:id/tracking", middleware.RequireCustomer(), api.GetOrderTracking)
	orders.POST("", middleware.RequireRoles(string(models.RoleCustomer)), api.CreateOrder)
	orders.PATCH("/:id/status", middleware.RequireRoles(string(models.RoleCustomer), string(models.RoleSeller),
		string(models.RoleDeliveryAgent), string(models.RoleAdmin)), api.UpdateOrderStatus)

	deliveries := apiGroup.Group("/deliveries")
	deliveries.Use(requireAuth)
	deliveries.GET("", middleware.RequireRoles(string(models.RoleAdmin)), api.ListDeliveries)
	deliveries.GET("/:id", middleware.RequireRoles(string(models.RoleDeliveryAgent), string(models.RoleAdmin)), api.GetDelivery)
	deliveries.POST("", middleware.RequireRoles(string(models.RoleAdmin)), api.CreateDelivery)
	deliveries.PATCH("/:id/status", middleware.RequireRoles(string(models.RoleDeliveryAgent), string(models.RoleAdmin)), api.UpdateDeliveryStatus)
	apiGroup.GET("/agents/:agentId/deliveries", requireAuth,
		middleware.RequireRoles(string(models.RoleDeliveryAgent), string(models.RoleAdmin)), api.AgentDeliveries)

	admin := apiGroup.Group("/admin", requireAuth, middleware.RequireAdmin())
	admin.GET("/users", api.ListUsers)
	admin.GET("/analytics", api.AdminAnalytics)
	admin.POST("/search/reindex", api.ReindexSearch)
	apiGroup.GET("/users", requireAuth, middleware.RequireAdmin(), api.ListUsers)

	cart := apiGroup.Group("/cart", requireAuth, middleware.RequireCustomer())
	cart.GET("", api.GetCart)
	cart.POST("/items", api.AddCartItem)
	cart.PUT("/items/:productId", api.UpdateCartItem)
	cart.DELETE("/items/:productId", api.RemoveCartItem)
	cart.DELETE("", api.ClearCart)
	cart.POST("/checkout", api.CreateOrder)

	payments := apiGroup.Group("/payments", requireAuth, middleware.RequireCustomer())
	payments.POST("", handlers.NotImplemented("Payments"))
	payments.GET("/:id", handlers.NotImplemented("Payments"))

	reviews := apiGroup.Group("/reviews")
	reviews.GET("", handlers.NotImplemented("Reviews"))
	reviews.GET("/products/:productId", handlers.NotImplemented("Reviews"))
	reviews.POST("", requireAuth, middleware.RequireCustomer(), handlers.NotImplemented("Reviews"))

	return router
}
