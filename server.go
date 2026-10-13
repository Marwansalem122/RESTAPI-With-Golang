package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateUserRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

var (
	users  = make(map[int]User)
	nextID = 1
	mu     sync.RWMutex
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /users", getUsers)
	mux.HandleFunc("POST /users", createUser)
	mux.HandleFunc("GET /users/{id}", getUser)
	mux.HandleFunc("PUT /users/{id}", updateUser)
	mux.HandleFunc("PATCH /users/{id}", patchUser)
	mux.HandleFunc("DELETE /users/{id}", deleteUser)

	log.Println("Server running on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func parseID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid user ID")
	}

	return id, nil
}

func decodeJSON(r *http.Request, dst any) error {
	if !strings.
		HasPrefix(r.Header.
			Get("Content-Type"), "application/json") {
		return errors.New("Content-Type must be application/json")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("request must contain one JSON value")
	} else if !errors.Is(err, errors.New("EOF")) &&
		err.Error() != "EOF" {
		return err
	}

	return nil
}

func validUser(name, email string) bool {
	return strings.TrimSpace(name) != "" &&
		strings.TrimSpace(email) != ""
}

// GET /users
func getUsers(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	result := make([]User, 0, len(users))
	for _, user := range users {
		result = append(result, user)
	}

	writeJSON(w, http.StatusOK, result)
}

// POST /users
func createUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest

	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON request",
		})
		return
	}

	if !validUser(req.Name, req.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Name and email are required",
		})
		return
	}

	mu.Lock()
	user := User{
		ID:    nextID,
		Name:  strings.TrimSpace(req.Name),
		Email: strings.TrimSpace(req.Email),
	}
	users[nextID] = user
	nextID++
	mu.Unlock()

	writeJSON(w, http.StatusCreated, user)
}

// GET /users/{id}
func getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid user ID",
		})
		return
	}

	mu.RLock()
	user, ok := users[id]
	mu.RUnlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// PUT /users/{id}
func updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid user ID",
		})
		return
	}

	var req CreateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON request",
		})
		return
	}

	if !validUser(req.Name, req.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Name and email are required",
		})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := users[id]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
		return
	}

	user := User{
		ID:    id,
		Name:  strings.TrimSpace(req.Name),
		Email: strings.TrimSpace(req.Email),
	}
	users[id] = user

	writeJSON(w, http.StatusOK, user)
}

// PATCH /users/{id}
func patchUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid user ID",
		})
		return
	}

	var req UpdateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid JSON request",
		})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	user, ok := users[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
		return
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Name cannot be empty",
			})
			return
		}
		user.Name = strings.TrimSpace(*req.Name)
	}

	if req.Email != nil {
		if strings.TrimSpace(*req.Email) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Email cannot be empty",
			})
			return
		}
		user.Email = strings.TrimSpace(*req.Email)
	}

	users[id] = user
	writeJSON(w, http.StatusOK, user)
}

// DELETE /users/{id}
func deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid user ID",
		})
		return
	}

	mu.Lock()
	_, ok := users[id]
	if ok {
		delete(users, id)
	}
	mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "User not found",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
