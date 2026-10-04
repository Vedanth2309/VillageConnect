# VillageConnect

VillageConnect is a village-first marketplace prototype built with React, Go, and a Docker-ready MongoDB/Elasticsearch setup. It follows the phased project plan for local commerce discovery, ordering, supply management, and delivery workflows.

## Stack

- Frontend: React + Vite
- Backend: Go + Gin
- Data services: MongoDB + Elasticsearch via Docker Compose

## Getting started

### Backend

```bash
cd backend
go mod tidy
go run ./cmd/server
```

The API will be available at http://localhost:8080.

### Frontend

```bash
cd frontend
npm install
npm run dev -- --host 0.0.0.0
```

The site will be available at http://localhost:5173.

### Optional local data services

```bash
docker compose up -d
```

The backend will automatically use MongoDB at `mongodb://localhost:27017` and Elasticsearch at `http://localhost:9200` when those services are running. If they are not available, the app keeps working in resilient fallback mode using the bundled seed data.

MongoDB remains the source of truth for products and sellers. The Go API synchronizes product creates, updates, and deletes to Elasticsearch; search reads re-hydrate product results from MongoDB. Elasticsearch includes fuzzy full-text, prefix autocomplete, village/category/price/availability filters, and relevance sorting, with MongoDB fallback when search is unavailable. Product and verified-seller indexes are rebuilt at startup and can be rebuilt by an admin with `POST /api/admin/search/reindex` using a valid bearer token.

### Production deployment

```bash
cp .env.example .env
docker compose -f docker-compose.prod.yml up --build -d
```

This builds and runs all services together:
- frontend on http://localhost:9091
- backend on http://localhost:9090
- MongoDB on mongodb://localhost:27017
- Elasticsearch on http://localhost:9200

If local ports 8080/80 are already occupied in the environment, the project uses the alternate production ports 9090/9091 to avoid conflicts.

## Environment variables

```bash
PORT=9090
GIN_MODE=release
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=villageconnect
ELASTICSEARCH_URL=http://localhost:9200
ELASTICSEARCH_INDEX=villageconnect-products
FRONTEND_URL=http://localhost:9091
JWT_SECRET=villageconnect-dev-secret-change-me
CORS_ALLOWED_ORIGINS=http://localhost:9091,http://127.0.0.1:9091
```

## Included in this version

- Landing page and village selector
- Product catalog with category filtering and search
- Customer-owned carts persisted in MongoDB
- Server-validated checkout with delivery/pickup, address, simulated payment, and inventory reservation
- Go API health and product endpoints
- Local fallback data when the backend is not running

## Main API endpoints

| Method | Endpoint | Access |
|---|---|---|
| GET | `/api/health` | Public |
| POST | `/api/auth/register` | Public; customer accounts only |
| POST | `/api/auth/login` | Public |
| GET | `/api/auth/me`, `/api/users/me` | Authenticated |
| POST | `/api/auth/logout` | Authenticated; revokes current session |
| GET | `/api/villages`, `/api/categories` | Public |
| GET | `/api/products`, `/api/products/:id` | Public |
| POST, PUT | `/api/products`, `/api/products/:id` | Seller or admin |
| GET | `/api/search/products?q=tomato` | Public |
| GET | `/api/sellers` | Public |
| GET | `/api/sellers/:sellerId/products` | Own seller account or admin |
| GET | `/api/cart` | Customer; current product data and server-calculated totals |
| POST | `/api/cart/items` | Customer; adds an available product in the selected village |
| PUT, DELETE | `/api/cart/items/:productId` | Customer; updates or removes own cart item |
| DELETE | `/api/cart` | Customer; clears own cart |
| POST | `/api/cart/checkout` | Customer; validates the saved cart, reserves stock, and creates the order |
| GET | `/api/orders` | Customer's own orders; seller's own orders; admin all |
| GET | `/api/orders/:id`, `/api/orders/:id/tracking` | Customer/seller ownership checked; tracking includes status history |
| GET | `/api/seller/orders`, `/api/seller/orders/:id` | Seller's own orders |
| PATCH | `/api/seller/orders/:id/status` | Seller accepts/rejects and advances own orders |
| PATCH | `/api/orders/:id/status` | Customer cancellation, assigned-agent order progress, or admin action |
| POST, GET | `/api/payments`, `/api/payments/:id` | Customer; external payment endpoints return `501` (simulated payments are created at checkout) |
| GET, POST, PATCH | `/api/deliveries`, `/api/deliveries/:id`, `/api/deliveries/:id/status` | Admin or assigned delivery agent; in-transit/delivered updates also advance the order lifecycle |
| GET | `/api/agents/:agentId/deliveries` | Own agent account or admin |
| GET | `/api/admin/users`, `/api/admin/sellers`, `/api/admin/agents`, `/api/admin/villages`, `/api/admin/categories`, `/api/admin/products`, `/api/admin/orders`, `/api/admin/analytics` | Admin |
| PATCH | `/api/admin/users/:id/active`, `/api/admin/agents/:id/active`, `/api/admin/sellers/:id/status`, `/api/admin/products/:id/availability` | Admin |
| POST, PUT, PATCH | `/api/admin/villages`, `/api/admin/categories` and their `/:id` management paths | Admin |
| GET, PATCH | `/api/notifications`, `/api/notifications/unread-count`, `/api/notifications/:id/read` | Authenticated; only own notifications |
| POST | `/api/seller/verification-request` | Authenticated seller |
| GET | `/api/users` | Admin |
| GET | `/api/reviews?productId=:id`, `/api/reviews/products/:productId`, `/api/reviews/sellers/:sellerId` | Public product and seller reviews |
| GET | `/api/orders/:id/reviews` | Customer's own order review state |
| POST | `/api/reviews` | Customer; only delivered products from an owned order; one review per order item |

## Backend structure

The Go API is organized by responsibility:

- `internal/routes`: route groups and middleware attachment
- `internal/middleware`: bearer-token authentication and role checks
- `internal/handlers`: HTTP binding, status codes, and response envelopes
- `internal/services`: input validation and business rules
- `internal/repository`: MongoDB access and the existing local fallback
- `internal/search`: product-search service boundary
- `internal/auth`, `internal/models`, and `internal/config`: session tokens, data models, and runtime configuration

The current route groups are `/api/auth`, `/api/users`, `/api/villages`, `/api/categories`, `/api/products`, `/api/search`, `/api/sellers`, `/api/seller/orders`, `/api/seller/verification-request`, `/api/cart`, `/api/orders`, `/api/deliveries`, `/api/agents`, `/api/notifications`, `/api/reviews`, and `/api/admin`. Payments are simulated at checkout and persisted both as a separate payment record and an order snapshot; external payment query endpoints remain explicitly protected `501 Not Implemented` routes.

Cart lines persist in MongoDB by customer. Checkout ignores client-supplied item names, prices, and totals, re-reads product/seller/village state, recalculates subtotal and fees, conditionally reserves stock without allowing negative inventory, and creates an order plus its payment record. If order/payment/cart persistence fails, prior stock reservations are compensated. Delivery currently uses a fixed INR 25 fee; pickup has no delivery fee. Checkout supports one seller and one serviceable village per cart to match the existing order model. Sellers must be verified and active; missing active fields on legacy records mean active.

Orders follow `pending → confirmed → preparing → packed → ready_for_pickup → out_for_delivery → delivered`, with terminal `cancelled` and `rejected` outcomes. Seller actions are restricted to their own orders, delivery agents must be assigned, customer cancellation is allowed through confirmation, and every transition is validated and recorded in the order's status history. Conditional MongoDB stock updates ensure competing orders cannot oversell a product; a checkout contention test covers one customer requesting 4 from stock 5 while another requests 3. Admin analytics use MongoDB aggregations; the index/query and inventory consistency examples are documented in [docs/DBMS_DEMONSTRATION.md](./docs/DBMS_DEMONSTRATION.md).

For compatibility with databases seeded by earlier versions, startup only backfills `sellerId` on the exact bundled sample products when their original sample seller name still matches, and inserts the corresponding sample seller account only when absent. Other product and user records are not modified by this transition.

Private routes require `Authorization: Bearer <token>`. Admin routes require an admin role; sellers can manage their own listings and orders; customers can access their own orders; and delivery agents can access only their assigned deliveries. Notifications are stored in MongoDB and are readable only by their recipient. Public registration creates customer accounts only.

The precise method-by-method access classification and ownership rules are in [docs/AUTHORIZATION.md](./docs/AUTHORIZATION.md). Elevated roles must be provisioned through a trusted administrative process; self-service registration cannot assign them.

When running with `GIN_MODE=release`, configure a unique `JWT_SECRET` of at least 32 characters. Copy `.env.example` to `.env` and replace its placeholder before starting the production Compose stack.

This backend refactor deliberately retains the legacy string-ID MongoDB documents and existing seeded data. Migrating those collections to the approved ObjectId-based schema requires the separately planned, versioned data migration; the new schema is not silently applied at startup.
