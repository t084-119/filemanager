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
	"sort"
	"strings"
)

type Node struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Children []Node `json:"children,omitempty"`
}

type FileResponse struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Name    string `json:"name"`
}

type CreateRequest struct {
	Parent  string `json:"parent"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(".", "data")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(absDataDir, 0o755); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		relPath := r.URL.Query().Get("path")
		rootPath, err := resolvePath(absDataDir, relPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		node, err := buildTree(absDataDir, rootPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, node)
	})

	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		relPath := r.URL.Query().Get("path")
		filePath, err := resolvePath(absDataDir, relPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		switch r.Method {
		case http.MethodGet:
			info, err := os.Stat(filePath)
			if err != nil {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			if info.IsDir() {
				writeError(w, http.StatusBadRequest, "path is a directory")
				return
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			fileType := detectFileType(filePath)
			resp := FileResponse{Type: fileType, Content: string(data), Name: info.Name()}
			writeJSON(w, resp)
		case http.MethodPut:
			if !strings.HasSuffix(strings.ToLower(filePath), ".md") {
				writeError(w, http.StatusBadRequest, "only markdown files can be updated")
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid body")
				return
			}
			if err := os.WriteFile(filePath, body, 0o644); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]string{"status": "ok"})
		case http.MethodDelete:
			if strings.TrimSpace(relPath) == "" {
				writeError(w, http.StatusBadRequest, "cannot delete root directory")
				return
			}
			if err := os.RemoveAll(filePath); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if name != filepath.Base(name) {
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		parentPath, err := resolvePath(absDataDir, req.Parent)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		info, err := os.Stat(parentPath)
		if err != nil || !info.IsDir() {
			writeError(w, http.StatusBadRequest, "invalid parent directory")
			return
		}
		targetPath := filepath.Join(parentPath, name)
		switch strings.ToLower(req.Type) {
		case "dir":
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		case "file":
			if err := os.WriteFile(targetPath, []byte(req.Content), 0o644); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid type")
			return
		}
		writeJSON(w, map[string]string{"status": "created", "path": toRelative(absDataDir, targetPath)})
	})

	mux.HandleFunc("/api/raw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		relPath := r.URL.Query().Get("path")
		filePath, err := resolvePath(absDataDir, relPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		info, err := os.Stat(filePath)
		if err != nil {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		if info.IsDir() {
			writeError(w, http.StatusBadRequest, "path is a directory")
			return
		}
		contentType := mime.TypeByExtension(filepath.Ext(filePath))
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeFile(w, r, filePath)
	})

	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		relPath := r.URL.Query().Get("path")
		dirPath, err := resolvePath(absDataDir, relPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		info, err := os.Stat(dirPath)
		if err != nil || !info.IsDir() {
			writeError(w, http.StatusBadRequest, "invalid directory")
			return
		}
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse form")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing file")
			return
		}
		defer file.Close()
		filename := filepath.Base(header.Filename)
		if filename == "." || filename == string(filepath.Separator) {
			writeError(w, http.StatusBadRequest, "invalid filename")
			return
		}
		targetPath := filepath.Join(dirPath, filename)
		out, err := os.Create(targetPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "uploaded", "path": toRelative(absDataDir, targetPath)})
	})

	staticDir := filepath.Join(".", "frontend", "dist")
	mux.Handle("/", spaHandler(staticDir))

	addr := ":8080"
	fmt.Printf("File manager running on %s (data dir: %s)\n", addr, absDataDir)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		panic(err)
	}
}

func resolvePath(baseDir, relPath string) (string, error) {
	clean := filepath.Clean("/" + relPath)
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	target := filepath.Join(baseDir, clean)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if absTarget != baseDir && !strings.HasPrefix(absTarget, baseDir+string(filepath.Separator)) {
		return "", errors.New("invalid path")
	}
	return absTarget, nil
}

func toRelative(baseDir, absPath string) string {
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return ""
	}
	if rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

func buildTree(baseDir, rootPath string) (Node, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return Node{}, err
	}
	node := Node{
		Name: info.Name(),
		Path: toRelative(baseDir, rootPath),
		Type: "file",
	}
	if info.IsDir() {
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			return Node{}, err
		}
		sort.Slice(entries, func(i, j int) bool {
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})
		node.Type = "dir"
		node.Children = make([]Node, 0, len(entries))
		for _, entry := range entries {
			childPath := filepath.Join(rootPath, entry.Name())
			childNode, err := buildTree(baseDir, childPath)
			if err != nil {
				return Node{}, err
			}
			node.Children = append(node.Children, childNode)
		}
	}
	return node, nil
}

func detectFileType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return "image"
	case ".pdf":
		return "pdf"
	case ".txt":
		return "text"
	default:
		return "text"
	}
}

func spaHandler(distDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(distDir, filepath.Clean(r.URL.Path))
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
		writeError(w, http.StatusNotFound, "frontend not built")
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
