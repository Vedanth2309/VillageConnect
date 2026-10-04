# VillageConnect implementation audit

**Audit date:** 2026-10-04  
**Scope:** Baseline working-tree contents at the initial audit. This was a source inspection, not a runtime/security test.

> **Historical baseline:** This report predates the later roadmap implementation. Its findings describe that earlier baseline and are not a current assessment of the repository.

## Executive summary

VillageConnect is a React/Vite single-page marketplace prototype backed by one Go/Gin service. It has a useful starting catalog, MongoDB persistence with seeded demo records and an in-memory fallback, a product-search integration with Elasticsearch, password hashing, JWT issuance, and rudimentary role-oriented panels.

The main user journey is not complete end-to-end. The UI has catalog browsing, a local cart, login/registration forms, a product-listing form, lightweight admin/agent panels, and a checkout request. The backend exposes more endpoints than the UI uses, but almost all are unauthenticated. In particular, public registration accepts privileged roles; orders, products, and deliveries can be read or mutated without role or ownership checks. Checkout trusts client-supplied identities, prices, totals, and status, and does not reserve/decrement stock.

The repository is small outside dependencies and generated output. The tracked application code is concentrated in one large React component and one large Go HTTP entrypoint. No application test files or migrations are present in the repository inventory.

> The worktree already contained modified and untracked project files when this audit began. Findings describe the current contents, not a clean committed baseline. No existing source or configuration was edited for this audit.

## A. What is already implemented

### Frontend and screens

- React 19 + Vite app with a single rendered `App` component; `main.jsx` mounts it under `StrictMode`. There are no separate route-backed pages or feature component modules.
- Landing/marketplace view includes a village selector, categories, a product grid, text filtering, cart summary, and explanatory/trust sections.
- Cart quantity changes and totals are kept in React component state. Checkout prompts unauthenticated users to sign in and, for signed-in users, sends an order request.
- Login/registration modal stores the token and user payload in `localStorage` and calls `/api/auth/me` to refresh the user on startup.
- Role-conditional seller, admin, and delivery-agent panels render inside the same page. Seller panel can submit a listing; admin panel displays summary counts and a user list; delivery panel can attempt status updates.
- Most catalog data is initially available from a duplicated frontend fallback list if the API is unavailable.
- Relevant source: `frontend/src/App.jsx`, `frontend/src/main.jsx`, `frontend/src/App.css`, `frontend/src/index.css`.

### Backend and API

- Go 1.23 service uses Gin, MongoDB Go driver, JWT v5, bcrypt, and Elasticsearch Go client.
- Routes currently registered in `backend/cmd/server/main.go`:

  | Method | Path | Current behavior |
  |---|---|---|
  | GET | `/api/health` | Returns a basic API message and storage mode |
  | GET | `/api/villages` | Lists villages |
  | GET | `/api/categories` | Lists categories |
  | GET | `/api/products` | Lists products |
  | GET | `/api/products/:id` | Fetches one product |
  | GET | `/api/search/products?q=...` | Searches products |
  | GET | `/api/orders` | Lists all orders |
  | GET | `/api/orders/:id` | Fetches one order |
  | POST | `/api/orders` | Creates an order from the request body |
  | PATCH | `/api/orders/:id/status` | Changes an order status |
  | GET | `/api/deliveries` | Lists all delivery assignments |
  | GET | `/api/deliveries/:id` | Fetches one delivery |
  | POST | `/api/deliveries` | Creates an assignment |
  | PATCH | `/api/deliveries/:id/status` | Changes a delivery status |
  | GET | `/api/agents/:agentId/deliveries` | Filters deliveries by supplied agent ID |
  | GET | `/api/sellers` | Lists seller-role users |
  | GET | `/api/sellers/:sellerId/products` | Lists products matching a seller |
  | POST | `/api/products` | Creates a product |
  | PUT | `/api/products/:id` | Replaces product fields |
  | GET | `/api/admin/users` | Lists users |
  | GET | `/api/admin/analytics` | Returns simple aggregate counts and revenue |
  | POST | `/api/auth/register` | Creates a user and returns a JWT |
  | POST | `/api/auth/login` | Verifies a password hash and returns a JWT |
  | GET | `/api/auth/me` | Validates a JWT and returns its current user |

- Passwords are hashed with bcrypt. JWTs are signed with HMAC-SHA256 and carry user ID, role, email, issue time, and a 24-hour expiry (`backend/internal/auth/auth.go`).
- CORS is configured for a list of local origins plus optional `CORS_ALLOWED_ORIGINS`.
- Relevant source: `backend/cmd/server/main.go`, `backend/internal/auth/auth.go`, `backend/internal/config/config.go`.

### Data, search, and deployment

- MongoDB collections used by the store: `villages`, `categories`, `products`, `users`, `orders`, and `deliveries`.
- Mongo documents use string IDs as `_id`; models include users/roles, products, orders/order items, delivery assignments, villages, and categories (`backend/internal/models/models.go`).
- Empty collections are populated from built-in defaults. If MongoDB cannot connect or seed, the service continues with in-memory copies of the demo users, products, orders, and deliveries (`backend/internal/storage/storage.go`).
- Elasticsearch can create a product index and search across product name, description, category, village, and seller. Mongo regex search or in-memory substring search is used as a fallback.
- `docker-compose.yml` provides MongoDB and Elasticsearch for local use. `docker-compose.prod.yml` additionally builds the API and static frontend images. Nginx serves the SPA and falls back to `index.html` for client routes.
- Environment names present in config/docs include `PORT`, `GIN_MODE`, `MONGO_URI`, `MONGO_DATABASE`, `ELASTICSEARCH_URL`, `ELASTICSEARCH_INDEX`, `FRONTEND_URL`, `JWT_SECRET`, `CORS_ALLOWED_ORIGINS`, and build-time `VITE_API_URL`. A local ignored `.env` file exists; its values were not inspected or reproduced here.

## B. What is partially implemented

| Area | Implemented | Incomplete behavior |
|---|---|---|
| Authentication | Registration, login, bcrypt hashes, signed JWTs, `/api/auth/me`, local token storage | No authorization middleware on feature routes; public role selection includes `admin`; no verification, password reset/change, refresh/revocation, or session invalidation |
| Roles | `customer`, `seller`, `delivery_agent`, `admin` constants and role-conditioned panels | Roles are labels/UI branching, not effective server-side access policy |
| Products | Read/list/detail/search, create, update, seller filter, stock/availability fields | No delete; update/create are not seller-scoped; no field validation or ownership enforcement; stock is not tied to purchasing |
| Villages/categories | Read endpoints and seeded records; village selection filters catalog in the browser | No create/update/delete APIs or database relationship validation; records are not managed in UI |
| Cart | Add/remove-by-quantity and client-side subtotal/delivery fee | No persistence, server-side cart, inventory check, seller grouping, or authoritative price calculation |
| Orders | Create/list/detail/status persistence and a status enum | No customer/seller-scoped queries, state transition rules, stock reservation, payment, cancellation flow, order history/tracking UI, or trusted total calculation |
| Seller | Listing form, seller-filtered product route, basic count/inventory panel | No order management, listing edit/delete, verified seller workflow, or enforceable seller/product association |
| Delivery agent | Seed assignment, list-by-agent route, basic status button | No assignment workflow, proof/pickup validation, transition validation, or linkage that synchronizes assignment status with order status |
| Admin | Analytics and all-user read endpoints plus a summary panel | No protected admin access or user/product/order/village management operations |
| Elasticsearch | Index setup, product indexing on product writes, and fuzzy multi-field search | Not authoritative or reliably synchronized with the current Mongo catalog; UI does not call the search endpoint |
| Persistence fallback | Read/write operations have in-memory fallback branches | Fallback state is process-local and disappears on restart; the UI may report generic fallback data instead of making that limitation clear |

## C. What is completely missing from the current implementation

- Protected API groups and server-side role/ownership authorization.
- Separate frontend routes/pages for customer orders, seller order handling, delivery workflows, and administration.
- Customer order-history/detail UI and APIs that scope orders to the authenticated customer.
- Product deletion, village/category management, seller onboarding/verification, and admin mutation APIs.
- Payment processing, checkout address/contact capture, refunds, and payment/order reconciliation.
- Inventory reservation/decrement/restock rules, concurrency-safe stock updates, and out-of-stock enforcement.
- A complete order-to-delivery lifecycle: creating assignments from accepted orders, assigning agents, and keeping order/delivery status consistent.
- Persistent server-side cart or a guest-cart merge flow.
- MongoDB schema migrations/index setup for unique email constraints, relationship query indexes, or data normalization.
- Elasticsearch backfill/reindex, delete synchronization, retry/repair workflow, and monitoring.
- Automated backend/frontend tests, API contract tests, and end-to-end tests in the inspected project files.
- Production-grade operational configuration such as database authentication/TLS, Elasticsearch security, secret injection/validation, and deployment-domain configuration.

## D. Existing bugs and correctness/security gaps

1. **Critical — role escalation during public registration.** The registration payload accepts `role`; `normalizeRole` accepts `admin`, and the frontend exposes an Admin role option. Any caller can register an administrator account. See `backend/cmd/server/main.go:22-31,410-458` and `frontend/src/App.jsx:948-962`.
2. **Critical — protected data and mutations have no enforced authorization.** The only handler that validates a token is `/api/auth/me`; route registration does not attach authentication/role middleware. Order/delivery lists, admin endpoints, product writes, and status changes are therefore publicly callable. The frontend sending a bearer token on some calls does not protect the API. See `backend/cmd/server/main.go:76-508`.
3. **High — order creation trusts client-controlled identity, price, amount, and status.** The handler accepts `customerId`, `sellerId`, item names/prices/quantities, total, and arbitrary status rather than deriving/validating them from a verified user and current products. A caller can forge order ownership, understate totals, use invalid statuses or quantities, and bypass inventory. See `backend/cmd/server/main.go:148-187`.
4. **High — seller identity in checkout is hard-coded.** The UI submits `sellerId: 'u2'` regardless of which products are in the cart. The cart can contain mixed sellers, while the seeded seller user is not the seller name associated with the seeded catalog. This makes order attribution unreliable. See `frontend/src/App.jsx:316-343` and `backend/internal/models/models.go:104-153`.
5. **High — seller product mutations are not scoped to the seller.** `/api/products` and `/api/products/:id` accept caller-controlled seller values and do not verify the caller; the UI only has a client-side role check for product creation. See `backend/cmd/server/main.go:311-390` and `frontend/src/App.jsx:408-438`.
6. **High — production JWT secret is a checked-in placeholder override.** `docker-compose.prod.yml` explicitly sets `JWT_SECRET: change-me-to-a-secure-secret`, overriding the `.env` value supplied through `env_file`. Replacing `.env` therefore does not replace this service value. The Go config also falls back to a known development secret when `JWT_SECRET` is absent. See `docker-compose.prod.yml:41-53` and `backend/internal/config/config.go:18-24`.
7. **High — production Compose exposes insecure data services.** MongoDB and Elasticsearch have no configured authentication; Elasticsearch security is explicitly disabled, and ports `27017` and `9200` are published to the host. This is not a safe production posture. See `docker-compose.prod.yml:3-40`.
8. **High — product availability and stock are not enforced in cart or order flows.** The frontend lets users add unavailable items and increase quantity without limit; the backend accepts any quantity and never adjusts stock. See `frontend/src/App.jsx:270-314,640-653` and `backend/cmd/server/main.go:148-187`.
9. **High — demo state is exposed as if it were real marketplace data.** Both frontend and backend include hard-coded village/product samples; backend defaults include demo users of every role and seeded orders/deliveries. On a fresh DB these are inserted. The login form is prefilled with a demo customer email and a fixed password value. This is unsuitable for a real deployment unless explicitly gated or replaced. See `backend/internal/models/models.go:96-153`, `frontend/src/App.jsx:19-92,115-145`, and `frontend/src/App.jsx:922-946`.
10. **Medium — development frontend/backend defaults disagree.** The Vite app defaults to API port `9090`, but the Go config defaults to `8080`; the README's direct Go run uses the default server port and does not set `VITE_API_URL`. The client will fail its API calls in that documented local setup and fall back to demo data. See `frontend/src/App.jsx:95`, `backend/internal/config/config.go:18`, and `README.md:13-34`.
11. **Medium — production frontend API URL is baked in as localhost.** Compose passes `http://localhost:9090` to the Vite build. This works only when the user's browser can reach the API at its own `localhost`; it will fail when the frontend is deployed on another host/domain. See `docker-compose.prod.yml:60-67`.
12. **Medium — search input bypasses Elasticsearch API.** The browser filters the already loaded products locally by name and description, so the backend search endpoint, fuzzy matching, and backend search fields are not used by the actual search box. See `frontend/src/App.jsx:260-270` and `backend/cmd/server/main.go:120-128`.
13. **Medium — search regex uses unescaped user input.** Mongo search embeds the raw query in `.*<query>.*`; regular-expression syntax is interpreted rather than searched literally and can lead to unexpectedly broad or expensive matches. See `backend/internal/storage/storage.go:193-211`.
14. **Medium — database email behavior differs between fallback and MongoDB.** Fallback lookup compares email case-insensitively, while Mongo lookup is exact; registration stores a trimmed email but checks for duplicates using the untrimmed input. There is no unique email index, so case variants or concurrent registration can create duplicates. See `backend/internal/storage/storage.go:268-284`, `backend/cmd/server/main.go:421-455`, and `backend/internal/storage/storage.go:687-718`.
15. **Medium — status endpoints accept invalid or arbitrary transitions.** Order status only checks for a non-empty string and delivery status has no status validation at all. Any caller can move an order directly to any string; delivery state changes do not update the related order. See `backend/cmd/server/main.go:189-207,262-276` and `backend/internal/storage/storage.go:457-475,541-559`.
16. **Medium — Elasticsearch synchronization can fail silently or partially.** Startup marks Elasticsearch enabled before confirming index creation; startup indexing errors are discarded, and startup indexes only the default products rather than backfilling current Mongo records. Product writes persist to Mongo before indexing, so an indexing failure can return an API error after the Mongo write succeeded. See `backend/internal/storage/storage.go:72-91,363-395,571-684`.
17. **Medium — analytics revenue counts every order total.** Canceled orders are included in `revenue`, and the figures are calculated by loading every user, product, and order into memory. The current number is not necessarily paid/realized revenue and will not scale well. See `backend/internal/storage/storage.go:419-455`.
18. **Low — visible UI content contains malformed characters and inactive controls.** Several labels render question marks in place of currency/emoji/check/close symbols; hero/footer calls to action have no behavior. See `frontend/src/App.jsx:560-590,636-713,904-912,968-989`.
19. **Low — seeded order and product relationships do not consistently agree.** The seeded mango product names a different seller than the seller ID on the seeded order. Its total is higher than the item subtotal, but the order model has no delivery-fee field to explain the difference. Products store seller/village display strings while orders use IDs, leaving no canonical relationship to validate. See `backend/internal/models/models.go:104-153`.

## E. Architecture problems

- **Oversized feature modules:** UI, state, API calls, and all role dashboards live in `App.jsx`; route setup and all business logic are inline in `main.go`. This makes testing and changing individual flows difficult.
- **Client-authoritative business rules:** Pricing, identity, status, seller assignment, and availability are sent from the browser and trusted by the API. Business invariants need to live at the service/data boundary.
- **No authorization boundary:** JWT validation is an isolated `/me` operation instead of reusable middleware. UI conditional rendering is not an access-control mechanism.
- **Weak relationships:** Product seller and village are free-text values; categories are also strings. There are no referential checks when creating products/orders/deliveries, and seller-product matching relies on names or IDs with fallback behavior.
- **One-size-fits-all data access:** List endpoints return complete collections and no filtering/pagination/sorting limits are provided. Analytics also loads whole collections into process memory.
- **Duplicated and drifting demo fixtures:** Frontend and backend contain separate product/village lists and descriptions; the frontend default cart is populated even before a customer chooses an item.
- **Non-durable error mode:** Backend fallback allows operation without MongoDB but writes disappear on restart; it can make a deployment appear functional while not persisting transactions.
- **No clear cross-store consistency contract:** MongoDB is primary and Elasticsearch is secondary, but startup synchronization, retries, repair, and partial-write handling are not defined.
- **Configuration does not separate local and production guarantees:** Development secrets have code defaults, the production Compose file overrides secret configuration, and browser API origin is a build-time localhost constant.
- **Documentation and inventory do not yet describe a finished product:** Root README documents basic catalog endpoints and fallback behavior, but not the auth/order/seller/admin/delivery routes or their prototype limitations. The frontend README remains the generated Vite template.

## F. Recommended development order

1. **Close the access-control and deployment exposure gaps first.** Remove public admin-role assignment; define role/ownership policy; enforce JWT and role checks in backend middleware; protect all customer, seller, agent, and admin data/mutations; replace development secret fallbacks with required deployment secrets; stop exposing unauthenticated database/search services in production.
2. **Define canonical domain relationships and validation.** Agree on entity IDs and relationships (product-to-seller/village/category; order-to-customer/seller/items; delivery-to-order/agent). Add request validation, unique email constraints, and the Mongo indexes needed for those queries.
3. **Implement authoritative checkout and inventory behavior.** Derive the customer from JWT; fetch current products/prices; validate quantity, availability, seller grouping, village, and totals; apply stock reservation/decrement atomically or with a transaction/compensation strategy; return a persisted order.
4. **Complete order and fulfillment lifecycles.** Add scoped order queries and allowed transitions; seller confirmation/packing operations; assignment creation and agent ownership; delivery updates that synchronize the order; customer history/detail/status views.
5. **Separate frontend routes and role areas.** Extract shared components, API client/auth handling, marketplace, cart/checkout, and role dashboards. Remove dead controls, make search invoke the API, and expose meaningful loading/empty/error states.
6. **Make MongoDB and Elasticsearch behavior deterministic.** Add a versioned seed/migration strategy, explicit unique/query indexes, Elasticsearch backfill/reindex and delete/update synchronization, retries/repair visibility, and a clear policy for when fallback mode is permitted.
7. **Fix configuration and operations.** Use environment-specific configuration without localhost assumptions; ensure the production `.env`/secret mechanism actually controls the API; scope service ports/network access; add readiness checks and useful health details for both data services.
8. **Add automated coverage before expanding features.** Cover auth/role access, ownership boundaries, input/status validation, order totals and inventory races, persistence/fallback behavior, Elasticsearch indexing/search fallback, frontend checkout and dashboards, and local/production configuration. Then update README/setup instructions to the validated behavior.
