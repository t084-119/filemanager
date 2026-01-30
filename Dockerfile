FROM node:20 AS frontend-build
WORKDIR /app/frontend
COPY frontend/ ./
RUN npm install && npm audit fix --force
RUN npm run build
#前端容器测试
# EXPOSE 5173
# ENV VITE_API_URL=http://filemanager-backend:8080
# CMD ["npm", "run", "dev"]

FROM golang:1.22 AS backend-build
WORKDIR /app/backend
COPY backend/*.go ./
RUN go mod init filemanager && go mod tidy
RUN go build .
WORKDIR /app/backend/tools
COPY backend/tools/*.go ./
RUN go mod init tool && go mod tidy
RUN go build .
COPY backend/.user ../.user/
# CMD ["./filemanager"]

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=backend-build /app/backend/filemanager ./backend/filemanager
COPY --from=backend-build /app/backend/tools/tool ./tools/
COPY --from=backend-build /app/backend/.user ./.user/
COPY --from=frontend-build /app/frontend/dist ./frontend/dist
RUN chmod +x ./backend/filemanager
EXPOSE 8080
CMD ["./backend/filemanager"]

