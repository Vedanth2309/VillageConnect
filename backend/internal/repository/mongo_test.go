package repository

import (
	"context"
	"sync"
	"testing"

	"villageconnect/internal/models"
)

func TestListMarketplaceProductsFiltersAndPaginatesFallback(t *testing.T) {
	store := &Store{
		Fallback: true,
		products: []models.Product{
			{ID: "p1", Name: "Tomato", Category: "Vegetables", Village: "Kudlu", VillageID: "v1", Price: 20, Available: true, Stock: 4},
			{ID: "p2", Name: "Potato", Category: "Vegetables", Village: "Kudlu", VillageID: "v1", Price: 10, Available: true, Stock: 2},
			{ID: "p3", Name: "Mango", Category: "Fruits", Village: "Anekal", VillageID: "v2", Price: 30, Available: true, Stock: 5},
			{ID: "p4", Name: "Onion", Category: "Vegetables", Village: "Kudlu", Price: 15, Available: false, Stock: 0},
		},
	}

	available := true
	minimum := 5.0
	query := models.ProductQuery{
		VillageID: "v1", VillageName: "Kudlu", Category: "Vegetables",
		MinPrice: &minimum, Available: &available, Sort: "price_desc", Page: 1, PageSize: 1,
	}
	result, err := store.ListMarketplaceProducts(context.Background(), query)
	if err != nil {
		t.Fatalf("ListMarketplaceProducts returned error: %v", err)
	}
	if result.Total != 2 || result.TotalPages != 2 || len(result.Products) != 1 || result.Products[0].ID != "p1" {
		t.Fatalf("unexpected first marketplace page: %+v", result)
	}

	query.Page = 2
	result, err = store.ListMarketplaceProducts(context.Background(), query)
	if err != nil {
		t.Fatalf("ListMarketplaceProducts page 2 returned error: %v", err)
	}
	if len(result.Products) != 1 || result.Products[0].ID != "p2" {
		t.Fatalf("unexpected second marketplace page: %+v", result)
	}

	unavailable := false
	result, err = store.ListMarketplaceProducts(context.Background(), models.ProductQuery{
		VillageID: "v1", VillageName: "Kudlu", Available: &unavailable,
		Page: 1, PageSize: 5,
	})
	if err != nil {
		t.Fatalf("ListMarketplaceProducts for unavailable listings returned error: %v", err)
	}
	if len(result.Products) != 1 || result.Products[0].ID != "p4" {
		t.Fatalf("legacy village-name listing was not retained: %+v", result)
	}
}

func TestCartPersistenceAndCheckoutStockReservationFallback(t *testing.T) {
	store := &Store{
		Fallback: true,
		products: []models.Product{{ID: "p1", Available: true, Stock: 5}},
		carts:    make(map[string]models.Cart),
	}
	cart := models.Cart{
		CustomerID: "c1", VillageID: "v1", SellerID: "s1",
		Items: []models.CartItem{{ProductID: "p1", Quantity: 2}},
	}
	if err := store.SaveCart(context.Background(), cart, ""); err != nil {
		t.Fatalf("SaveCart returned error: %v", err)
	}
	persisted, err := store.GetCart(context.Background(), "c1")
	if err != nil || persisted.Revision == "" || persisted.Items[0].Quantity != 2 {
		t.Fatalf("persisted cart = %+v, error %v", persisted, err)
	}
	order := models.Order{ID: "o1", CustomerID: "c1", InventoryReserved: true, Status: models.OrderPending,
		PaymentID: "pay1", Payment: models.OrderPayment{Status: "paid"},
		Items: []models.OrderItem{{ProductID: "p1", Quantity: 2}}}
	payment := models.Payment{ID: "pay1", OrderID: order.ID, Status: "paid"}
	if err := store.CheckoutOrder(context.Background(), order, payment, persisted.Revision); err != nil {
		t.Fatalf("CheckoutOrder returned error: %v", err)
	}
	if store.products[0].Stock != 3 {
		t.Fatalf("product stock = %d, want 3 after checkout", store.products[0].Stock)
	}
	payments, err := store.ListOrderPayments(context.Background())
	if err != nil || len(payments) != 1 || payments[0].OrderID != order.ID {
		t.Fatalf("payment record was not persisted: payments=%+v err=%v", payments, err)
	}
	if _, err := store.GetCart(context.Background(), "c1"); err != ErrNotFound {
		t.Fatalf("cart after checkout error = %v, want not found", err)
	}

	cart.Items[0].Quantity = 4
	if err := store.SaveCart(context.Background(), cart, ""); err != nil {
		t.Fatal(err)
	}
	persisted, err = store.GetCart(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	order.ID = "o2"
	order.Items[0].Quantity = 4
	staleRevision := persisted.Revision
	if err := store.SaveCart(context.Background(), *persisted, staleRevision); err != nil {
		t.Fatal(err)
	}
	payment.ID, payment.OrderID = "pay2", order.ID
	if err := store.CheckoutOrder(context.Background(), order, payment, staleRevision); err != ErrConflict {
		t.Fatalf("checkout with a stale cart revision error = %v, want conflict", err)
	}
	persisted, err = store.GetCart(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CheckoutOrder(context.Background(), order, payment, persisted.Revision); err != ErrConflict {
		t.Fatalf("insufficient stock checkout error = %v, want conflict", err)
	}
	if store.products[0].Stock != 3 {
		t.Fatalf("failed checkout changed stock to %d, want 3", store.products[0].Stock)
	}
	if _, err := store.GetCart(context.Background(), "c1"); err != nil {
		t.Fatalf("failed checkout cleared the cart: %v", err)
	}
	cancelled, err := store.TransitionOrder(context.Background(), "o1", models.OrderPending, models.OrderCancelled,
		"c1", models.OrderStatusEvent{ActorRole: models.RoleCustomer})
	if err != nil || cancelled.Payment.Status != "refunded" || store.products[0].Stock != 5 {
		t.Fatalf("cancelled order did not release reserved stock/refund payment: order=%+v stock=%d err=%v",
			cancelled, store.products[0].Stock, err)
	}
	if _, err := store.TransitionOrder(context.Background(), "o1", models.OrderPending, models.OrderCancelled,
		"c1", models.OrderStatusEvent{}); err != ErrConflict {
		t.Fatalf("duplicate cancellation error = %v, want conflict", err)
	}
	if store.products[0].Stock != 5 {
		t.Fatalf("duplicate cancellation double-restocked inventory: %d", store.products[0].Stock)
	}

	store.products[0].Stock = 3
	rejectedOrder := models.Order{ID: "o-rejected", Status: models.OrderPending, InventoryReserved: true,
		Payment: models.OrderPayment{Status: "paid"}, Items: []models.OrderItem{{ProductID: "p1", Quantity: 2}}}
	store.orders = append(store.orders, rejectedOrder)
	rejected, err := store.TransitionOrder(context.Background(), rejectedOrder.ID, models.OrderPending,
		models.OrderRejected, "seller-1", models.OrderStatusEvent{ActorRole: models.RoleSeller})
	if err != nil || rejected.Payment.Status != "refunded" || store.products[0].Stock != 5 {
		t.Fatalf("rejected order did not release reserved stock/refund payment: order=%+v stock=%d err=%v",
			rejected, store.products[0].Stock, err)
	}
}

func TestConcurrentCheckoutsCannotOversellSingleProduct(t *testing.T) {
	store := &Store{
		Fallback: true,
		products: []models.Product{{ID: "p1", Available: true, Stock: 5}},
		carts:    make(map[string]models.Cart),
	}
	customers := []string{"customer-a", "customer-b"}
	quantities := []int{4, 3}
	revisions := make([]string, len(customers))
	for i, customerID := range customers {
		cart := models.Cart{CustomerID: customerID, Items: []models.CartItem{{ProductID: "p1", Quantity: quantities[i]}}}
		if err := store.SaveCart(context.Background(), cart, ""); err != nil {
			t.Fatal(err)
		}
		persisted, err := store.GetCart(context.Background(), customerID)
		if err != nil {
			t.Fatal(err)
		}
		revisions[i] = persisted.Revision
	}

	results := make(chan error, len(customers))
	var wait sync.WaitGroup
	for i, customerID := range customers {
		wait.Add(1)
		go func(i int, customerID string) {
			defer wait.Done()
			order := models.Order{
				ID: "order-" + customerID, CustomerID: customerID, InventoryReserved: true,
				Items: []models.OrderItem{{ProductID: "p1", Quantity: quantities[i]}},
			}
			payment := models.Payment{ID: "payment-" + customerID, OrderID: order.ID}
			results <- store.CheckoutOrder(context.Background(), order, payment, revisions[i])
		}(i, customerID)
	}
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch err {
		case nil:
			successes++
		case ErrConflict:
			conflicts++
		default:
			t.Fatalf("unexpected checkout error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 || store.products[0].Stock < 0 {
		t.Fatalf("concurrent checkout results: successes=%d conflicts=%d stock=%d", successes, conflicts, store.products[0].Stock)
	}
	if len(store.orders) != 1 || len(store.payments) != 1 {
		t.Fatalf("failed checkout persisted partial records: orders=%d payments=%d", len(store.orders), len(store.payments))
	}
}
