# 文件管理系统

一个 Go + Vue 的本地文件管理系统，支持目录树展示、Markdown + Mermaid 渲染、图片预览以及文件上传。

## 功能

- 目录树结构展示本地数据目录。
- Markdown 文件渲染（包含 Mermaid 渲染）。
- 图片文件预览。
- 在当前目录上传文件（图片/Markdown）。
- Markdown 文件可编辑并保存。
- 支持新建文件夹/Markdown 文件与删除目录/文件。

## 本地启动

### 后端

```bash
cd backend
go run .
```

默认数据目录为 `./data`，也可以通过环境变量覆盖：

```bash
DATA_DIR=/your/path go run .
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

访问 `http://localhost:5173`。

## Docker 部署

```bash
docker compose up --build
```

访问 `http://localhost:8080`，宿主机 `./data` 会挂载到容器 `/data`。
