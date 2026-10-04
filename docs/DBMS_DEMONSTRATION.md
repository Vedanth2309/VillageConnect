# VillageConnect DBMS Demonstration

VillageConnect uses MongoDB as the source of truth. Elasticsearch is a derived
search index and is rebuilt from MongoDB when needed. The examples below use
the existing string IDs and collections; they are suitable for a local MongoDB
shell demonstration.

## Indexes

Startup creates these indexes for correctness and common dashboard queries:

- `users.email` unique with case-insensitive collation prevents duplicate logins.
- `users.role` supports role counts and admin role filters.
- `products(villageId, category, available, price)` supports marketplace filters.
- `products(sellerId, name)` supports seller-owned inventory lists in name order.
- `products(stock, available)` supports low-stock review.
- `orders(customerId, createdAt)` and `orders(sellerId, createdAt)` support owned order history.
- `orders(status, createdAt)` and `orders(status, items.productId)` support sales/status aggregation.
- `carts.customerId` unique ensures one saved cart per customer.
- `payments.orderId` unique ensures at most one payment record per order.
- `reviews(orderId, productId)` unique prevents duplicate reviews for an order item.
- `notifications(userId, read, createdAt)` supports private unread counts and newest-first lists.
- `auth_sessions.expiresAt` TTL removes expired sessions.

## Query Examples

Products in a village:

```javascript
db.products.find({ villageId: "v1", available: true, stock: { $gt: 0 } })
```

Products owned by a seller:

```javascript
db.products.find({ sellerId: "seller-id" }).sort({ name: 1 })
```

Low-stock products:

```javascript
db.products.find({ stock: { $lte: 5 } }).sort({ stock: 1 })
```

Seller revenue from delivered orders:

```javascript
db.orders.aggregate([
  { $match: { sellerId: "seller-id", status: "delivered" } },
  { $group: { _id: "$sellerId", revenue: { $sum: "$subtotal" }, orders: { $sum: 1 } } }
])
```

Top-selling products from delivered orders:

```javascript
db.orders.aggregate([
  { $match: { status: "delivered" } },
  { $unwind: "$items" },
  { $group: {
      _id: "$items.productId",
      name: { $first: "$items.name" },
      unitsSold: { $sum: "$items.quantity" },
      revenue: { $sum: "$items.lineTotal" }
  } },
  { $sort: { unitsSold: -1 } },
  { $limit: 5 }
])
```

Village revenue:

```javascript
db.orders.aggregate([
  { $match: { status: "delivered" } },
  { $group: { _id: "$villageId", revenue: { $sum: "$subtotal" }, orders: { $sum: 1 } } },
  { $sort: { revenue: -1 } }
])
```

Daily or monthly sales use the same delivered-order match and group by the
first 10 or 7 characters of the ISO `createdAt` timestamp respectively.
Category sales unwind order items, look up product categories, and group by
category. Admin analytics use these aggregations and display village/category
bars, daily sales, top sellers, and low-stock products. Revenue means delivered
order subtotal; delivery fees are excluded.

## Consistency and Search

Checkout reserves stock with a conditional MongoDB update requiring
`available: true` and `stock >= quantity`; concurrent buyers cannot reduce stock
below zero. The order and payment records are written after reservation, and
failed persistence compensates prior reservations. Cancellation/rejection
restoration records per-order markers so a partial restore can resume without
double-incrementing inventory.

Product changes are written to MongoDB first. The Go catalog service then
updates Elasticsearch best-effort; MongoDB remains authoritative and the admin
search reindex route can rebuild derived indexes. Product availability changes
also synchronize their search documents.