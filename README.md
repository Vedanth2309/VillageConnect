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

## Included in this version

- Landing page and village selector
- Product catalog with category filtering and search
- Cart and checkout summary
- Go API health and product endpoints
- Local fallback data when the backend is not running

## Main API endpoints

- GET /api/health
- GET /api/villages
- GET /api/categories
- GET /api/products
- GET /api/products/:id
- GET /api/search/products?q=tomato
