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

func TestProductUpdateRecordsHistoryAndRejectsStaleStock(t *testing.T) {
	store := &Store{
		Fallback: true,
		products: []models.Product{{ID: "p1", Price: 10, Stock: 5, Available: true}},
	}
	expectedStock := 5
	previousPrice, newPrice := 10.0, 12.0
	previousStock, newStock := 5, 8
	change := models.ProductChange{
		ActorID: "seller-1", ChangedAt: "2026-10-04T10:00:00Z",
		PreviousPrice: &previousPrice, NewPrice: &newPrice,
		PreviousStock: &previousStock, NewStock: &newStock,
	}
	product := models.Product{ID: "p1", Price: newPrice, Stock: newStock, Available: true,
		ExpectedStock: &expectedStock, Change: &change}
	if err := store.UpdateProduct(context.Background(), product); err != nil {
		t.Fatalf("UpdateProduct returned error: %v", err)
	}
	if len(store.products[0].History) != 1 || store.products[0].History[0].ActorID != "seller-1" {
		t.Fatalf("product history was not recorded: %+v", store.products[0].History)
	}
	unchangedStock := store.products[0].Stock
	nameOnlyUpdate := store.products[0]
	nameOnlyUpdate.ExpectedStock = &unchangedStock
	nameOnlyUpdate.Description = "Updated description"
	if err := store.UpdateProduct(context.Background(), nameOnlyUpdate); err != nil {
		t.Fatalf("name-only UpdateProduct returned error: %v", err)
	}
	if len(store.products[0].History) != 1 {
		t.Fatalf("name-only update duplicated product history: %+v", store.products[0].History)
	}

	staleExpectedStock := 5
	product.Stock = 9
	product.ExpectedStock = &staleExpectedStock
	if err := store.UpdateProduct(context.Background(), product); err != ErrConflict {
		t.Fatalf("stale stock update error = %v, want conflict", err)
	}
	if store.products[0].Stock != 8 || len(store.products[0].History) != 1 {
		t.Fatalf("stale stock update changed product: %+v", store.products[0])
	}
}

func TestRestoreOrderInventoryResumesWithoutDoubleRestocking(t *testing.T) {
	store := &Store{
		Fallback: true,
		products: []models.Product{
			{ID: "p1", Stock: 7, RestockedOrderIDs: []string{"o-restore"}},
			{ID: "p2", Stock: 1},
		},
		orders: []models.Order{{ID: "o-restore", Status: models.OrderRejected, InventoryReserved: true,
			Items: []models.OrderItem{{ProductID: "p1", Quantity: 2}, {ProductID: "p2", Quantity: 3}}}},
	}

	if _, err := store.RestoreOrderInventory(context.Background(), "o-restore"); err != nil {
		t.Fatalf("RestoreOrderInventory returned error: %v", err)
	}
	if store.products[0].Stock != 7 || store.products[1].Stock != 4 || !store.orders[0].InventoryRestored {
		t.Fatalf("partial restoration was not resumed correctly: products=%+v order=%+v", store.products, store.orders[0])
	}
	if len(store.products[0].RestockedOrderIDs) != 0 {
		t.Fatalf("completed restoration left a product marker behind: %+v", store.products[0].RestockedOrderIDs)
	}
	if _, err := store.RestoreOrderInventory(context.Background(), "o-restore"); err != nil {
		t.Fatalf("repeated RestoreOrderInventory returned error: %v", err)
	}
	if store.products[0].Stock != 7 || store.products[1].Stock != 4 {
		t.Fatalf("repeated restoration changed inventory: %+v", store.products)
	}
}

func TestCreateReviewFallbackUpdatesProductAndSellerAveragesOnce(t *testing.T) {
	store := &Store{
		Fallback: true,
		users:    []models.User{{ID: "seller-1", Role: models.RoleSeller}},
		products: []models.Product{{ID: "product-1", SellerID: "seller-1", Rating: 4.5}},
	}
	first := models.Review{ID: "review-1", OrderID: "order-1", ProductID: "product-1", SellerID: "seller-1",
		CustomerID: "customer-1", ProductRating: 5, SellerRating: 4}
	if err := store.CreateReview(context.Background(), first); err != nil {
		t.Fatalf("CreateReview returned error: %v", err)
	}
	second := models.Review{ID: "review-2", OrderID: "order-2", ProductID: "product-1", SellerID: "seller-1",
		CustomerID: "customer-2", ProductRating: 3, SellerRating: 2}
	if err := store.CreateReview(context.Background(), second); err != nil {
		t.Fatalf("second CreateReview returned error: %v", err)
	}
	if got := store.products[0]; got.Rating != 4 || got.RatingCount != 2 {
		t.Fatalf("product rating = %.2f (%d), want 4.00 (2)", got.Rating, got.RatingCount)
	}
	if got := store.users[0]; got.Rating != 3 || got.RatingCount != 2 {
		t.Fatalf("seller rating = %.2f (%d), want 3.00 (2)", got.Rating, got.RatingCount)
	}
	if err := store.CreateReview(context.Background(), second); err != ErrConflict {
		t.Fatalf("duplicate order-item review error = %v, want conflict", err)
	}
}

func TestAnalyticsFallbackAggregatesDeliveredSalesAndLowStock(t *testing.T) {
	store := &Store{
		Fallback: true,
		users: []models.User{{ID: "seller-1", Role: models.RoleSeller},
			{ID: "agent-1", Role: models.RoleDeliveryAgent}},
		villages: []models.Village{{ID: "v1", Name: "Kudlu"}},
		products: []models.Product{{ID: "p1", Name: "Tomato", Category: "Vegetables", Stock: 3}},
		orders: []models.Order{
			{ID: "o1", Status: models.OrderDelivered, VillageID: "v1", Subtotal: 20, Total: 25,
				CreatedAt: "2026-09-01T10:00:00Z", Items: []models.OrderItem{{ProductID: "p1", Name: "Tomato", Quantity: 2, LineTotal: 20}}},
			{ID: "o2", Status: models.OrderCancelled, VillageID: "v1", Subtotal: 90, Total: 95},
		},
	}
	stats, err := store.GetAnalytics(context.Background())
	if err != nil {
		t.Fatalf("GetAnalytics returned error: %v", err)
	}
	if stats["users"] != 2 || stats["sellers"] != 1 || stats["deliveryAgents"] != 1 || stats["revenue"] != 20.0 {
		t.Fatalf("unexpected analytics totals: %+v", stats)
	}
	topProducts := stats["topSellingProducts"].([]map[string]interface{})
	if len(topProducts) != 1 || topProducts[0]["name"] != "Tomato" || topProducts[0]["unitsSold"] != 2 {
		t.Fatalf("unexpected top products: %+v", topProducts)
	}
	villageSales := stats["salesByVillage"].([]map[string]interface{})
	if len(villageSales) != 1 || villageSales[0]["name"] != "Kudlu" || villageSales[0]["revenue"] != 20.0 {
		t.Fatalf("unexpected village sales: %+v", villageSales)
	}
	categorySales := stats["salesByCategory"].([]map[string]interface{})
	if len(categorySales) != 1 || categorySales[0]["category"] != "Vegetables" {
		t.Fatalf("unexpected category sales: %+v", categorySales)
	}
	lowStock := stats["lowStockProducts"].([]models.Product)
	if len(lowStock) != 1 || lowStock[0].ID != "p1" {
		t.Fatalf("unexpected low-stock products: %+v", lowStock)
	}
}

func TestNotificationsAreScopedAndMarkReadForOwner(t *testing.T) {
	store := &Store{Fallback: true}
	first := models.Notification{ID: "n1", UserID: "customer-1", Type: "order_status"}
	second := models.Notification{ID: "n2", UserID: "seller-1", Type: "new_order"}
	if err := store.CreateNotification(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateNotification(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListNotifications(context.Background(), "customer-1", 20)
	if err != nil || len(list) != 1 || list[0].ID != "n1" {
		t.Fatalf("customer notifications=%+v err=%v", list, err)
	}
	if _, err := store.ListNotifications(context.Background(), "", 20); err != ErrConflict {
		t.Fatalf("empty-user notification list error = %v, want conflict", err)
	}
	if err := store.MarkNotificationRead(context.Background(), "seller-1", "n1"); err != ErrNotFound {
		t.Fatalf("cross-user mark-read error = %v, want not found", err)
	}
	if err := store.MarkNotificationRead(context.Background(), "customer-1", "n1"); err != nil {
		t.Fatal(err)
	}
	count, err := store.CountUnreadNotifications(context.Background(), "customer-1")
	if err != nil || count != 0 {
		t.Fatalf("customer unread count=%d err=%v, want 0", count, err)
	}
}
