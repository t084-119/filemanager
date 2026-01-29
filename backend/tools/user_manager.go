package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loadUsers(filePath string) (map[string]User, error) {
	users := make(map[string]User)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return users, nil
		}
		return nil, err
	}
	defer file.Close()

	var userList []User
	if err := json.NewDecoder(file).Decode(&userList); err != nil {
		return nil, err
	}

	for _, user := range userList {
		users[user.Username] = user
	}

	return users, nil
}

func saveUsers(filePath string, users map[string]User) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	userList := make([]User, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(userList)
}

func changeUser(dataDir, username, password string) error {
	usersFilePath := filepath.Join(dataDir, "user.json")
	users, err := loadUsers(usersFilePath)
	if err != nil {
		return err
	}

	if _, exists := users[username]; !exists {
		return fmt.Errorf("user '%s' does not exist", username)
	}

	users[username] = User{Username: username, Password: password}
	return saveUsers(usersFilePath, users)
}

func addUser(dataDir, username, password string) error {
	usersFilePath := filepath.Join(dataDir, "user.json")
	users, err := loadUsers(usersFilePath)
	if err != nil {
		return err
	}

	if _, exists := users[username]; exists {
		return fmt.Errorf("user '%s' already exists", username)
	}

	users[username] = User{Username: username, Password: password}
	return saveUsers(usersFilePath, users)
}

func removeUser(dataDir, username string) error {
	usersFilePath := filepath.Join(dataDir, "user.json")
	users, err := loadUsers(usersFilePath)
	if err != nil {
		return err
	}

	if _, exists := users[username]; !exists {
		return fmt.Errorf("user '%s' does not exist", username)
	}

	delete(users, username)
	return saveUsers(usersFilePath, users)
}

func listUsers(dataDir string) error {
	usersFilePath := filepath.Join(dataDir, "user.json")
	users, err := loadUsers(usersFilePath)
	if err != nil {
		return err
	}

	if len(users) == 0 {
		fmt.Println("No users found")
		return nil
	}

	fmt.Println("Users:")
	for username := range users {
		fmt.Printf("  - %s\n", username)
	}

	return nil
}

func main() {
	var dataDir string
	var username string
	var password string
	var list bool
	var addUserCmd bool
	var removeUserCmd bool
	var changeUserCmd bool

	flag.StringVar(&dataDir, "dir", "../.user", "User data directory")
	flag.StringVar(&username, "username", "", "Username")
	flag.StringVar(&password, "password", "", "Password")
	flag.BoolVar(&list, "list", false, "List all users")
	flag.BoolVar(&addUserCmd, "add", false, "Add a new user")
	flag.BoolVar(&removeUserCmd, "remove", false, "Remove a user")
	flag.BoolVar(&changeUserCmd, "change", false, "Change user password")
	flag.Parse()

	if list {
		if err := listUsers(dataDir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if addUserCmd {
		if username == "" || password == "" {
			fmt.Println("Error: --username and --password are required for --add")
			os.Exit(1)
		}
		if err := addUser(dataDir, username, password); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("User '%s' added successfully\n", username)
		return
	}

	if removeUserCmd {
		if username == "" {
			fmt.Println("Error: --username is required for --remove")
			os.Exit(1)
		}
		if err := removeUser(dataDir, username); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("User '%s' removed successfully\n", username)
		return
	}

	if changeUserCmd {
		if username == "" || password == "" {
			fmt.Println("Error: --username and --password are required for --change")
			os.Exit(1)
		}
		if err := changeUser(dataDir, username, password); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("User '%s' password changed successfully\n", username)
		return
	}

	fmt.Println("Usage:")
	fmt.Println("  go run user_manager.go --list")
	fmt.Println("  go run user_manager.go --add --username <name> --password <password>")
	fmt.Println("  go run user_manager.go --remove --username <name>")
	fmt.Println("  go run user_manager.go --change --username <name> --password <password>")
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
}
