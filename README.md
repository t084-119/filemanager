# 本地文件管理项目

该项目使用 Go + Vue 3 构建，可用于本地 Markdown 与图片文件的上传、删除与预览，支持按目录组织文件，并渲染 Mermaid 图表。

## 功能

- 上传 Markdown 与图片文件（png/jpg/jpeg/gif/webp）
- 文件列表展示目录层级
- 在网页创建目录与 Markdown 文件
- 点击 Markdown 文件自动渲染为 HTML（支持 Mermaid）
- 点击图片文件直接预览
- 当前端未构建时，后端根路径显示服务状态信息

## 项目架构

- Go 后端负责文件上传、列表、删除以及静态文件读取，并同时托管前端构建产物。
- Vue 3 前端提供上传、列表与预览界面，通过 `/api` 与 `/files` 访问后端接口。
- `data/` 目录作为本地持久化存储，Docker 运行时可挂载到容器内。

## 目录结构

```
backend/   Go API 服务
frontend/  Vue 3 前端
data/      本地文件目录
```

## 本地开发

### 后端

```bash
cd backend
DATA_DIR=../data go run main.go
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

## Docker 部署

### 构建镜像

```bash
docker build -t local-file-manager .
```

### 运行容器

```bash
docker run --rm -p 8080:8080 -v $(pwd)/data:/app/data local-file-manager
```

### 访问

打开 `http://localhost:8080`。
