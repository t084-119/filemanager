package main

import (
	"fmt"
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PermissionManager struct {
	mu            sync.RWMutex
	permissionMap map[string]bool
	filePath      string
}

var globalPermissionManager *PermissionManager

func InitPermissionManager(dataDir string) error {
	pm := &PermissionManager{
		permissionMap: make(map[string]bool),
		filePath:      filepath.Join(dataDir, ".permissions"),
	}

	if err := pm.load(); err != nil {
		return err
	}

	globalPermissionManager = pm
	return nil
}

func GetPermissionManager() *PermissionManager {
	return globalPermissionManager
}

func (pm *PermissionManager) load() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	file, err := os.Open(pm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Permission file not found: %s\n", pm.filePath)
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pm.permissionMap[line] = true
		fmt.Printf("Loaded permission: %s\n", line)
	}

	fmt.Printf("Total permissions loaded: %d\n", len(pm.permissionMap))
	return scanner.Err()
}

func (pm *PermissionManager) save() error {
	file, err := os.Create(pm.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	for path := range pm.permissionMap {
		if _, err := file.WriteString(path + "\n"); err != nil {
			return err
		}
	}

	return nil
}

func (pm *PermissionManager) AddPermission(path string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.permissionMap[path] = true
	return pm.save()
}

func (pm *PermissionManager) RemovePermission(path string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.permissionMap, path)
	return pm.save()
}

func (pm *PermissionManager) HasPermission(path string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	fmt.Printf("HasPermission called: path=%s\n", path)
	fmt.Printf("Loaded permissions: %v\n", pm.permissionMap)

	if pm.permissionMap[path] {
		fmt.Printf("HasPermission exact match: path=%s, result=true\n", path)
		return true
	}

	parts := strings.Split(path, "/")
	for i := 0; i < len(parts); i++ {
		for j := i + 1; j <= len(parts); j++ {
			subpath := strings.Join(parts[i:j], "/")
			if pm.permissionMap[subpath] {
				fmt.Printf("HasPermission subpath match: path=%s, subpath=%s, result=true\n", path, subpath)
				return true
			}
		}
	}

	fmt.Printf("HasPermission: path=%s, result=false\n", path)
	return false
}

func (pm *PermissionManager) ListPermissions() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	paths := make([]string, 0, len(pm.permissionMap))
	for path := range pm.permissionMap {
		paths = append(paths, path)
	}
	return paths
}

func (pm *PermissionManager) ClearAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.permissionMap = make(map[string]bool)
	return pm.save()
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserManager struct {
	mu       sync.RWMutex
	users    map[string]User
	filePath string
	modTime  time.Time
}

var globalUserManager *UserManager

func InitUserManager(dataDir string) error {
	um := &UserManager{
		users:    make(map[string]User),
		filePath: filepath.Join(dataDir, "user.json"),
	}

	if err := um.load(); err != nil {
		return err
	}

	if len(um.users) == 0 {
		um.users["admin"] = User{Username: "admin", Password: "admin"}
		um.save()
	}

	globalUserManager = um
	return nil
}

func GetUserManager() *UserManager {
	return globalUserManager
}

func (um *UserManager) load() error {
	um.mu.Lock()
	defer um.mu.Unlock()

	file, err := os.Open(um.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	um.modTime = info.ModTime()

	var users []User
	if err := json.NewDecoder(file).Decode(&users); err != nil {
		return err
	}

	for _, user := range users {
		um.users[user.Username] = user
	}

	return nil
}

func (um *UserManager) save() error {
	file, err := os.Create(um.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	users := make([]User, 0, len(um.users))
	for _, user := range um.users {
		users = append(users, user)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(users)
}

func (um *UserManager) AddUser(username, password string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	um.users[username] = User{Username: username, Password: password}
	return um.save()
}

func (um *UserManager) RemoveUser(username string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	delete(um.users, username)
	return um.save()
}

func (um *UserManager) Authenticate(username, password string) bool {
	if err := um.checkFileModified(); err != nil {
		fmt.Printf("User file modified: %v\n", err)
		GetSessionManager().ClearAllSessions()
		return false
	}

	um.mu.RLock()
	defer um.mu.RUnlock()

	user, exists := um.users[username]
	if !exists {
		return false
	}
	return user.Password == password
}

func (um *UserManager) checkFileModified() error {
	um.mu.Lock()
	defer um.mu.Unlock()

	info, err := os.Stat(um.filePath)
	if err != nil {
		return err
	}
	if info.ModTime().After(um.modTime) {
		fmt.Printf("User file has been modified, reloading...\n")
		um.modTime = info.ModTime()
		
		file, err := os.Open(um.filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		var users []User
		if err := json.NewDecoder(file).Decode(&users); err != nil {
			return err
		}

		um.users = make(map[string]User)
		for _, user := range users {
			um.users[user.Username] = user
		}
		
		fmt.Printf("User file reloaded successfully, %d users loaded\n", len(um.users))
		return fmt.Errorf("user file has been modified")
	}
	return nil
}

func (um *UserManager) ListUsers() []User {
	um.mu.RLock()
	defer um.mu.RUnlock()

	users := make([]User, 0, len(um.users))
	for _, user := range um.users {
		users = append(users, user)
	}
	return users
}

type Session struct {
	Token        string
	Username     string
	LastActivity time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

var globalSessionManager *SessionManager

func InitSessionManager() {
	globalSessionManager = &SessionManager{
		sessions: make(map[string]Session),
	}
}

func GetSessionManager() *SessionManager {
	return globalSessionManager
}

func (sm *SessionManager) CreateSession(username string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	token := username
	sm.sessions[token] = Session{
		Token:        token,
		Username:     username,
		LastActivity: time.Now(),
	}
	return token
}

func (sm *SessionManager) ValidateAndTouch(token string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[token]
	if !exists {
		return false
	}
	session.LastActivity = time.Now()
	sm.sessions[token] = session
	return true
}

func (sm *SessionManager) ClearAllSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions = make(map[string]Session)
}
