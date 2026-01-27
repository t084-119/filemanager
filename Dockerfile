# Build frontend
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json ./
COPY frontend/package-lock.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend/ ./
RUN npm run build

# Build backend
FROM golang:1.22-alpine AS backend-build
WORKDIR /app
COPY backend/go.mod ./
COPY backend/main.go ./
RUN go build -o server main.go

# Runtime
FROM alpine:3.19
WORKDIR /app
ENV DATA_DIR=/app/data
RUN adduser -D -g '' appuser
COPY --from=backend-build /app/server ./server
COPY --from=frontend-build /app/frontend/dist ./frontend/dist
RUN mkdir -p /app/data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["./server"]
