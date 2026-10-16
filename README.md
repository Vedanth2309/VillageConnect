
## Getting started

### 1. Configure environment variables

Create a root `.env` file containing your MongoDB connection,
Elasticsearch HTTPS endpoint, Elasticsearch API key, and
other required application settings.

### 2. Start the Go backend

Open a terminal in the project root:

```powershell
cd backend
go mod tidy
go run ./cmd/server
```

### 3. Start the React frontend

Open a second terminal in the project root:

```powershell
cd frontend
npm install
npm run dev
```

Open the local URL printed by Vite, usually
http://localhost:5173.

MongoDB and Elasticsearch must be reachable at the
connection URLs configured in the root `.env` file.
