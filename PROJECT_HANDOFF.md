# VillageConnect — Development Handoff

## Project

VillageConnect is a hyperlocal village marketplace.

Technology stack:

- Frontend: React + Vite
- Backend: Go
- API: REST
- Database: MongoDB
- Search: Elasticsearch
- Authentication: JWT
- Password hashing: bcrypt
- Infrastructure: Docker

---

# Completed Development

The project has completed development through Prompt 8 of the planned implementation roadmap.

Completed:

1. Project audit
2. MongoDB schema design
3. Go backend architecture
4. Authentication + RBAC
5. Village marketplace
6. Elasticsearch integration
7. Cart + checkout
8. Order + inventory system

---

# Current Development State

The current code in the repository is the source of truth.

Do NOT assume the old prototype structure is still accurate.

Always inspect the current code before making changes.

---

# Current Core Workflow

Customer:

Village selection
→ Product discovery
→ Search/filter
→ Product details
→ Cart
→ Checkout
→ Order
→ Order tracking

Seller:

Login
→ Dashboard
→ Products
→ Inventory
→ Orders
→ Accept/Reject
→ Prepare
→ Pack

Delivery:

Assignment
→ Accept
→ Pickup
→ In transit
→ Delivered

Admin:

Users
→ Sellers
→ Agents
→ Villages
→ Categories
→ Orders
→ Analytics

---

# Current Milestone

The project has completed the Order + Inventory phase.

The next task is:

PROMPT 9 — Complete Seller Dashboard

---

# Important Development Rules

1. Do not replace React.
2. Do not replace Go.
3. Do not replace MongoDB.
4. Do not replace Elasticsearch.
5. Do not rewrite working functionality unnecessarily.
6. Inspect the existing code before modifying it.
7. Use the current MongoDB schema as the source of truth.
8. Do not introduce hardcoded IDs.
9. Do not trust userId values from the frontend when JWT identity is available.
10. Maintain role-based authorization.
11. Sellers can only modify their own products/orders.
12. Customers can only access their own orders.
13. Delivery agents can only access their assigned deliveries.
14. Admin APIs must require admin authorization.
15. Do not commit secrets.
16. Run tests/build after major changes.

---

# Next Development Roadmap

## Prompt 9
Seller dashboard

## Prompt 10
Delivery agent system

## Prompt 11
Customer tracking + reviews

## Prompt 12
Admin panel

## Prompt 13
Analytics

## Prompt 14
Notifications

## Prompt 15
DBMS optimization

## Prompt 16
Security audit

## Prompt 17
Frontend polish

## Prompt 18
Final end-to-end testing