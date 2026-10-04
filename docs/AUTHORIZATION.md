# VillageConnect API access policy

This document classifies every route registered by `internal/routes`.
`PUBLIC` routes do not require a token. All other routes validate the JWT
signature and expiry, check the persisted session has not expired or been
revoked, reload the user from MongoDB, and use the current database role.

## Access classes

- **PUBLIC** — no login required.
- **AUTHENTICATED** — any active account.
- **CUSTOMER** — active customer role; resource ownership is also checked.
- **SELLER** — verified seller role; seller ownership is checked.
- **DELIVERY_AGENT** — verified delivery-agent role; assignment ownership is checked.
- **ADMIN** — verified admin role.

Where multiple roles are listed, the request must still pass the handler's
resource ownership check.

## Route classification

| Method | Route | Classification | Ownership / notes |
|---|---|---|---|
| GET | `/api/health` | PUBLIC | |
| POST | `/api/auth/register` | PUBLIC | Creates customer accounts only; requested elevated roles are rejected |
| POST | `/api/auth/login` | PUBLIC | Issues a 24-hour JWT and persisted session |
| GET | `/api/auth/me` | AUTHENTICATED | Loads the current account from the database |
| POST | `/api/auth/logout` | AUTHENTICATED | Revokes the current persisted session |
| GET | `/api/users/me` | AUTHENTICATED | Alias of `/api/auth/me` |
| GET | `/api/villages` | PUBLIC | Read-only; includes state, district, taluk, and village fields for location selection |
| GET | `/api/categories` | PUBLIC | Read-only |
| GET | `/api/products` | PUBLIC | Read-only; supports villageId, q, category, minPrice, maxPrice, available, page, pageSize, and sort filters |
| GET | `/api/products/:id` | PUBLIC | Read-only |
| POST | `/api/products` | SELLER, ADMIN | Seller ID is taken from authenticated account; admins must name a verified seller; MongoDB write is primary and indexing is synchronized best-effort |
| PUT | `/api/products/:id` | SELLER, ADMIN | Seller must own `sellerId`; MongoDB write is primary and indexing is synchronized best-effort |
| DELETE | `/api/products/:id` | SELLER, ADMIN | Seller must own `sellerId`; MongoDB delete is primary and the indexed document is deleted |
| GET | `/api/search/products` | PUBLIC | Elasticsearch full-text/fuzzy/autocomplete results with category, village, price, availability, pagination, and relevance filters; falls back to MongoDB |
| GET | `/api/search/sellers` | PUBLIC | Verified seller search from its Elasticsearch representation; falls back to MongoDB |
| GET | `/api/sellers` | PUBLIC | Returns verified seller profiles without private account fields |
| GET | `/api/sellers/:sellerId` | PUBLIC | Verified seller profile; private contact and authentication fields are omitted |
| GET | `/api/sellers/:sellerId/products` | SELLER, ADMIN | Sellers may request only their own ID |
| GET | `/api/orders` | CUSTOMER, SELLER, ADMIN | Results filtered to the caller's customer/seller orders; admin sees all |
| GET | `/api/orders/:id` | CUSTOMER, SELLER, ADMIN | Order customer/seller ownership or admin access required |
| GET | `/api/orders/:id/tracking` | CUSTOMER | Customer must own the order; returns status history |
| GET | `/api/cart` | CUSTOMER | Returns the caller's MongoDB cart with current catalog values and calculated totals |
| POST | `/api/cart/items` | CUSTOMER | Adds a product after village, stock, availability, and verified-seller checks |
| PUT | `/api/cart/items/:productId` | CUSTOMER | Updates only the caller's cart quantity; stock is revalidated |
| DELETE | `/api/cart/items/:productId` | CUSTOMER | Removes an item from the caller's cart |
| DELETE | `/api/cart` | CUSTOMER | Clears the caller's cart |
| POST | `/api/cart/checkout` | CUSTOMER | Revalidates saved cart, calculates totals, reserves stock, and creates the order |
| POST | `/api/orders` | CUSTOMER | Checkout alias; client identity, line-item details, prices, and totals are ignored |
| GET | `/api/seller/orders` | SELLER, ADMIN | Seller results are restricted to authenticated seller ID |
| GET | `/api/seller/orders/:id` | SELLER, ADMIN | Seller must own the order; admin may access any |
| PATCH | `/api/seller/orders/:id/status` | SELLER, ADMIN | Seller ownership and seller-specific state transition are required |
| PATCH | `/api/orders/:id/status` | CUSTOMER, SELLER, DELIVERY_AGENT, ADMIN | Customer may cancel own pending/confirmed orders; seller ownership and assigned delivery-agent ownership are validated |
| GET | `/api/deliveries` | ADMIN | Full assignment list |
| GET | `/api/deliveries/:id` | DELIVERY_AGENT, ADMIN | Agent must be assigned to the delivery |
| POST | `/api/deliveries` | ADMIN | |
| PATCH | `/api/deliveries/:id/status` | DELIVERY_AGENT, ADMIN | Agent must be assigned; status transition is validated |
| GET | `/api/agents/:agentId/deliveries` | DELIVERY_AGENT, ADMIN | Agent may query only own ID |
| GET | `/api/admin/users` | ADMIN | Password hashes are removed from responses |
| GET | `/api/admin/analytics` | ADMIN | |
| POST | `/api/admin/search/reindex` | ADMIN | Rebuilds product and seller search indexes from MongoDB |
| GET | `/api/users` | ADMIN | |
| POST, GET | `/api/payments`, `/api/payments/:id` | CUSTOMER | Currently returns `501 Not Implemented` after authorization |
| GET | `/api/reviews`, `/api/reviews/products/:productId` | PUBLIC | Currently returns `501 Not Implemented` |
| POST | `/api/reviews` | CUSTOMER | Currently returns `501 Not Implemented` after authorization |
| Any unmatched path | `*` | PUBLIC | Returns `404` JSON |
| Unsupported method | Registered route | Same as route | Returns `405` JSON |

## Session and role behavior

- Tokens use HS256, an explicit issuer and audience, `sub`, `jti`, `iat`, `nbf`,
  and a required expiry. Tokens are capped at 24 hours.
- `jti` is stored in `auth_sessions`; logout records revocation. MongoDB expires
  old records using a TTL index. In local fallback mode, sessions are
  process-local and disappear on restart; production startup must require
  MongoDB.
- Every protected request reloads the user and session. Role changes invalidate
  outstanding tokens whose role claim no longer matches the user record.
- Public registration cannot select seller, delivery-agent, or admin role.
  Elevated accounts must be provisioned through a trusted administrative
  process. Seller and agent routes also require `verified=true`.
- Products with legacy records lacking `sellerId` are not treated as owned by a
  seller for listing or update operations. Backfill these records before
  expecting seller dashboard access.
- Cart records are keyed by customer and persist in MongoDB. Checkout is limited
  to one seller and one village to match the existing order model. Products must
  remain available, have sufficient stock, and belong to the selected village;
  sellers must be verified. Stock decrements use conditional MongoDB updates, so
  concurrent checkouts cannot drive stock below zero; failed checkouts compensate
  earlier item reservations. A unique payment record is persisted with each
  checkout, in addition to the payment snapshot on the order. No external gateway
  is called.
- Orders follow `pending -> confirmed -> preparing -> packed ->
  ready_for_pickup -> out_for_delivery -> delivered`; pending orders may instead
  be rejected or cancelled, and customers may cancel their own order through
  confirmed. Seller actions are restricted to their own orders and transitions;
  delivery agents may advance only orders assigned to them. State changes are
  compare-and-set against the prior status and append an audit event. Cancellation
  and rejection release reserved stock once and mark simulated payments
  cancelled/refunded.
- Payment query endpoints and review handlers remain protected
  `501 Not Implemented` placeholders and do not claim success.
