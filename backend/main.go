package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileInfo struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Size     int64      `json:"size"`
	ModTime  time.Time  `json:"modTime"`
	Type     string     `json:"type"`
	Children []FileInfo `json:"children,omitempty"`
}

type Status struct {
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var allowedExtensions = map[string]string{
	".md":       "markdown",
	".markdown": "markdown",
	".png":      "image",
	".jpg":      "image",
	".jpeg":     "image",
	".gif":      "image",
	".webp":     "image",
}

var statusMu sync.Mutex
var lastStatus = Status{Message: "服务已启动", UpdatedAt: time.Now()}

func main() {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTree(w, dataDir)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUpload(w, r, dataDir)
	})

	mux.HandleFunc("/api/dir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		createDirectory(w, r, dataDir)
	})

	mux.HandleFunc("/api/md", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		createMarkdownFile(w, r, dataDir)
	})

	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/files/")
		if path == "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := deleteFile(dataDir, path); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updateStatus(fmt.Sprintf("已删除文件: %s", path))
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		statusMu.Lock()
		status := lastStatus
		statusMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/files/")
		if path == "" {
			http.NotFound(w, r)
			return
		}
		filePath, err := safeJoin(dataDir, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, ok := allowedExtensions[strings.ToLower(filepath.Ext(filePath))]; !ok {
			http.Error(w, "unsupported file type", http.StatusBadRequest)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filePath)
	})

	frontendDir := filepath.Join("frontend", "dist")
	mux.Handle("/", rootHandler(frontendDir, dataDir))

	addr := ":8080"
	fmt.Printf("Server running on %s\n", addr)
	if err := http.ListenAndServe(addr, logRequest(mux)); err != nil {
		panic(err)
	}
}

func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	if existsDir(filepath.Join("..", "data")) {
		return filepath.Join("..", "data")
	}
	return "data"
}

func existsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func listTree(w http.ResponseWriter, dir string) {
	tree, err := buildTree(dir, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tree.Children); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func buildTree(baseDir, relPath string) (FileInfo, error) {
	fullPath, err := safeJoin(baseDir, relPath)
	if err != nil {
		return FileInfo{}, err
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return FileInfo{}, err
	}
	children := make([]FileInfo, 0)
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		childRel := filepath.Join(relPath, entry.Name())
		if entry.IsDir() {
			childNode, err := buildTree(baseDir, childRel)
			if err != nil {
				continue
			}
			children = append(children, childNode)
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		fileType, ok := allowedExtensions[ext]
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		children = append(children, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(childRel),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Type:    fileType,
		})
	}

	return FileInfo{
		Name:     filepath.Base(relPath),
		Path:     filepath.ToSlash(relPath),
		Type:     "dir",
		Children: children,
	}, nil
}

func handleUpload(w http.ResponseWriter, r *http.Request, dir string) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := allowedExtensions[ext]; !ok {
		http.Error(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	relDir := r.FormValue("path")
	if relDir != "" {
		relDir = filepath.Clean(relDir)
	}

	safeName := filepath.Base(header.Filename)
	relPath := filepath.Join(relDir, safeName)
	destPath, err := safeJoin(dir, relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	updateStatus(fmt.Sprintf("已上传文件: %s", filepath.ToSlash(relPath)))
	w.WriteHeader(http.StatusCreated)
}

type createDirRequest struct {
	Path string `json:"path"`
}

func createDirectory(w http.ResponseWriter, r *http.Request, dir string) {
	var req createDirRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	fullPath, err := safeJoin(dir, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}
	updateStatus(fmt.Sprintf("已创建目录: %s", filepath.ToSlash(req.Path)))
	w.WriteHeader(http.StatusCreated)
}

type createMarkdownRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func createMarkdownFile(w http.ResponseWriter, r *http.Request, dir string) {
	var req createMarkdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	if !strings.HasSuffix(strings.ToLower(req.Path), ".md") &&
		!strings.HasSuffix(strings.ToLower(req.Path), ".markdown") {
		http.Error(w, "markdown extension required", http.StatusBadRequest)
		return
	}
	fullPath, err := safeJoin(dir, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(fullPath, []byte(req.Content), 0o644); err != nil {
		http.Error(w, "failed to create file", http.StatusInternalServerError)
		return
	}
	updateStatus(fmt.Sprintf("已创建 Markdown: %s", filepath.ToSlash(req.Path)))
	w.WriteHeader(http.StatusCreated)
}

func deleteFile(dir, path string) error {
	filePath, err := safeJoin(dir, path)
	if err != nil {
		return err
	}
	if _, ok := allowedExtensions[strings.ToLower(filepath.Ext(filePath))]; !ok {
		return errors.New("unsupported file type")
	}
	return os.Remove(filePath)
}

func safeJoin(baseDir, relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	if clean == "." {
		clean = ""
	}
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("invalid path")
	}
	full := filepath.Join(baseDir, clean)
	rel, err := filepath.Rel(baseDir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("invalid path")
	}
	return full, nil
}

func rootHandler(distDir, dataDir string) http.Handler {
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return spaHandler(distDir)
	}
	return statusPageHandler(dataDir)
}

func statusPageHandler(dataDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		statusMu.Lock()
		status := lastStatus
		statusMu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html>
<html lang="zh">
<head>
  <meta charset="UTF-8" />
  <title>文件服务状态</title>
  <style>
    body { font-family: sans-serif; background: #f8fafc; padding: 32px; }
    .card { background: #fff; padding: 24px; border-radius: 12px; box-shadow: 0 6px 18px rgba(0,0,0,0.08); }
    .label { color: #475569; font-size: 14px; }
    .value { font-size: 18px; margin-top: 6px; }
  </style>
</head>
<body>
  <div class="card">
    <h2>文件服务已启动</h2>
    <p class="label">数据目录</p>
    <div class="value">%s</div>
    <p class="label">最近操作</p>
    <div class="value">%s</div>
    <p class="label">更新时间</p>
    <div class="value">%s</div>
    <p>如需图形化操作，请构建前端并放入 <code>frontend/dist</code>。</p>
  </div>
</body>
</html>`, dataDir, status.Message, status.UpdatedAt.Format(time.RFC3339))
	})
}

func updateStatus(message string) {
	statusMu.Lock()
	lastStatus = Status{Message: message, UpdatedAt: time.Now()}
	statusMu.Unlock()
}

func spaHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
				if ctype := mime.TypeByExtension(filepath.Ext(path)); ctype != "" {
					w.Header().Set("Content-Type", ctype)
				}
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		indexPath := filepath.Join(dir, "index.html")
		http.ServeFile(w, r, indexPath)
	})
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
