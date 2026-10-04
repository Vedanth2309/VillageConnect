package repository

import (
	"context"
	"time"

	"villageconnect/internal/models"
)

// Repository defines the persistence operations used by application services.
type Repository interface {
	ListVillages(context.Context) ([]models.Village, error)
	ListCategories(context.Context) ([]models.Category, error)
	CreateVillage(context.Context, models.Village) error
	UpdateVillage(context.Context, models.Village) error
	SetVillageActive(context.Context, string, bool) error
	CreateCategory(context.Context, models.Category) error
	UpdateCategory(context.Context, models.Category) error
	SetCategoryActive(context.Context, string, bool) error
	ListProducts(context.Context) ([]models.Product, error)
	ListMarketplaceProducts(context.Context, models.ProductQuery) (models.ProductPage, error)
	GetProductByID(context.Context, string) (*models.Product, error)
	SearchProducts(context.Context, string) ([]models.Product, error)
	ListUsers(context.Context) ([]models.User, error)
	GetUserByEmailForAuth(context.Context, string) (*models.User, error)
	GetUserByID(context.Context, string) (*models.User, error)
	CreateUser(context.Context, models.User) error
	SetUserActive(context.Context, string, bool) error
	SetSellerState(context.Context, string, string, bool, bool) error
	CreateSession(context.Context, models.AuthSession) error
	GetSession(context.Context, string) (*models.AuthSession, error)
	RevokeSession(context.Context, string, time.Time) error
	ListOrders(context.Context) ([]models.Order, error)
	GetOrderByID(context.Context, string) (*models.Order, error)
	ListReviewsByOrder(context.Context, string) ([]models.Review, error)
	ListReviewsByProduct(context.Context, string) ([]models.Review, error)
	ListReviewsBySeller(context.Context, string) ([]models.Review, error)
	CreateReview(context.Context, models.Review) error
	CreateNotification(context.Context, models.Notification) error
	ListNotifications(context.Context, string, int) ([]models.Notification, error)
	CountUnreadNotifications(context.Context, string) (int64, error)
	MarkNotificationRead(context.Context, string, string) error
	ListOrderPayments(context.Context) ([]models.Payment, error)
	GetCart(context.Context, string) (*models.Cart, error)
	SaveCart(context.Context, models.Cart, string) error
	ClearCart(context.Context, string) error
	CheckoutOrder(context.Context, models.Order, models.Payment, string) error
	TransitionOrder(context.Context, string, models.OrderStatus, models.OrderStatus, string, models.OrderStatusEvent) (*models.Order, error)
	RestoreOrderInventory(context.Context, string) (*models.Order, error)
	ListDeliveries(context.Context) ([]models.DeliveryAssignment, error)
	GetDeliveryByID(context.Context, string) (*models.DeliveryAssignment, error)
	ListAgentDeliveries(context.Context, string) ([]models.DeliveryAssignment, error)
	CreateDelivery(context.Context, models.DeliveryAssignment) error
	UpdateDeliveryStatus(context.Context, string, string, string) (*models.DeliveryAssignment, error)
	CreateProduct(context.Context, models.Product) error
	UpdateProduct(context.Context, models.Product) error
	SetProductAvailability(context.Context, string, bool) error
	DeleteProduct(context.Context, string) error
	ListSellerProducts(context.Context, string) ([]models.Product, error)
	GetAnalytics(context.Context) (map[string]interface{}, error)
}
