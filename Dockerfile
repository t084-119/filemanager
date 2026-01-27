FROM node:20 AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.21 AS backend-build
WORKDIR /app
COPY backend/go.mod ./
COPY backend/ ./backend/
WORKDIR /app/backend
RUN go build -o /app/filemanager

FROM debian:bookworm-slim
WORKDIR /app
ENV DATA_DIR=/data
COPY --from=backend-build /app/filemanager ./filemanager
COPY --from=frontend-build /app/frontend/dist ./frontend/dist
RUN mkdir -p /data
EXPOSE 8080
CMD ["/app/filemanager"]
