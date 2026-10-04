FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/server /app/server
EXPOSE 8080
ENV PORT=8080 GIN_MODE=release
CMD ["/app/server"]
