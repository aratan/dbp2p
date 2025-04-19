package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aratan/dbp2p/pkg/auth"
	"github.com/aratan/dbp2p/pkg/binary"
	"github.com/aratan/dbp2p/pkg/db"

	"github.com/gorilla/mux"
)

// APIServer representa el servidor de la API REST
type APIServer struct {
	db            *db.Database
	authManager   *auth.AuthManager
	router        *mux.Router
	binaryManager *binary.BinaryManager
}

// NewAPIServer crea un nuevo servidor de API
func NewAPIServer(database *db.Database, authManager *auth.AuthManager) *APIServer {
	server := &APIServer{
		db:          database,
		authManager: authManager,
		router:      mux.NewRouter(),
	}

	// Configurar rutas
	server.setupRoutes()

	return server
}

// setupRoutes configura las rutas de la API
func (s *APIServer) setupRoutes() {
	// Rutas públicas
	s.router.HandleFunc("/api/auth/login", s.handleLogin).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/auth/forgot-password", s.handleForgotPassword).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/auth/reset-password", s.handleResetPassword).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET", "OPTIONS")

	// Rutas de archivos binarios
	// Nota: Las rutas de archivos binarios se configurarán más adelante

	// Rutas protegidas
	api := s.router.PathPrefix("/api").Subrouter()
	api.Use(s.authMiddleware)

	// Rutas de usuarios y roles (solo admin)
	api.HandleFunc("/users", s.handleGetUsers).Methods("GET")
	api.HandleFunc("/users", s.handleCreateUser).Methods("POST")
	api.HandleFunc("/users/{id}", s.handleGetUser).Methods("GET")
	api.HandleFunc("/users/{id}", s.handleUpdateUser).Methods("PUT")
	api.HandleFunc("/users/{id}", s.handleDeleteUser).Methods("DELETE")
	api.HandleFunc("/users/{id}/apikeys", s.handleCreateAPIKey).Methods("POST")
	api.HandleFunc("/users/{id}/apikeys/{token}", s.handleRevokeAPIKey).Methods("DELETE")

	api.HandleFunc("/roles", s.handleGetRoles).Methods("GET")
	api.HandleFunc("/roles", s.handleCreateRole).Methods("POST")
	api.HandleFunc("/roles/{name}", s.handleGetRole).Methods("GET")
	api.HandleFunc("/roles/{name}", s.handleUpdateRole).Methods("PUT")
	api.HandleFunc("/roles/{name}", s.handleDeleteRole).Methods("DELETE")

	// Rutas de la base de datos
	api.HandleFunc("/collections", s.handleListCollections).Methods("GET")
	api.HandleFunc("/collections/{collection}", s.handleGetCollection).Methods("GET")
	api.HandleFunc("/collections/{collection}", s.handleCreateDocument).Methods("POST")
	api.HandleFunc("/collections/{collection}/{id}", s.handleGetDocument).Methods("GET")
	api.HandleFunc("/collections/{collection}/{id}", s.handleUpdateDocument).Methods("PUT")
	api.HandleFunc("/collections/{collection}/{id}", s.handleDeleteDocument).Methods("DELETE")

	// Rutas de backup y restauración
	api.HandleFunc("/backups", s.handleListBackups).Methods("GET")
	api.HandleFunc("/backups", s.handleCreateBackup).Methods("POST")
	api.HandleFunc("/backups/{name}", s.handleRestoreBackup).Methods("POST")
	api.HandleFunc("/backups/{name}", s.handleDeleteBackup).Methods("DELETE")
}

// corsMiddleware es un middleware para manejar CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Permitir cualquier origen
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Configurar otras cabeceras CORS
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Manejar solicitudes OPTIONS (preflight)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continuar con la siguiente función en la cadena
		next.ServeHTTP(w, r)
	})
}

// Start inicia el servidor HTTP
func (s *APIServer) Start(port int) error {
	// Aplicar middleware CORS a nivel global
	handler := corsMiddleware(s.router)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("API server listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// authMiddleware es un middleware para autenticar las solicitudes
func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verificar header de autorización
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Autorización requerida", http.StatusUnauthorized)
			return
		}

		var user *auth.User
		var err error

		// Verificar si es un token Bearer o una clave API
		if strings.HasPrefix(authHeader, "Bearer ") {
			// Autenticación con JWT
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := auth.VerifyToken(tokenString)
			if err != nil {
				http.Error(w, "Token inválido", http.StatusUnauthorized)
				return
			}

			userID, ok := claims["user_id"].(string)
			if !ok {
				http.Error(w, "Token inválido", http.StatusUnauthorized)
				return
			}

			user, err = s.authManager.GetUserByID(userID)
		} else if strings.HasPrefix(authHeader, "ApiKey ") {
			// Autenticación con clave API
			apiKey := strings.TrimPrefix(authHeader, "ApiKey ")
			user, err = s.authManager.GetUserByAPIKey(apiKey)
		} else {
			http.Error(w, "Formato de autorización inválido", http.StatusUnauthorized)
			return
		}

		if err != nil {
			http.Error(w, "Credenciales inválidas", http.StatusUnauthorized)
			return
		}

		// Verificar permisos según la ruta
		path := r.URL.Path
		method := r.Method

		// Determinar el recurso y la acción
		var resource, action string

		// Extraer el recurso de la ruta
		if strings.HasPrefix(path, "/api/collections/") {
			parts := strings.Split(strings.TrimPrefix(path, "/api/collections/"), "/")
			if len(parts) > 0 {
				resource = parts[0]
			} else {
				resource = "*"
			}
		} else if strings.HasPrefix(path, "/api/users") || strings.HasPrefix(path, "/api/roles") || strings.HasPrefix(path, "/api/backups") {
			resource = "admin"
		} else {
			resource = "*"
		}

		// Determinar la acción según el método HTTP
		switch method {
		case "GET":
			action = "read"
		case "POST":
			action = "write"
		case "PUT":
			action = "write"
		case "DELETE":
			action = "delete"
		default:
			action = "*"
		}

		// Verificar permiso
		if !s.authManager.CheckUserPermission(user.ID, resource, action) {
			http.Error(w, "Acceso prohibido", http.StatusForbidden)
			return
		}

		// Añadir usuario al contexto
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getUserFromContext obtiene el usuario del contexto
func (s *APIServer) getUserFromContext(r *http.Request) *auth.User {
	user, ok := r.Context().Value("user").(*auth.User)
	if !ok {
		return nil
	}
	return user
}

// respondJSON responde con JSON
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}

// respondError responde con un error
func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}

// Manejadores de autenticación

// handleLogin maneja la autenticación de usuarios
func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Configurar cabeceras CORS explícitamente para este endpoint
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Manejar solicitudes OPTIONS (preflight)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Leer el cuerpo de la solicitud
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error al leer el cuerpo de la solicitud: %v", err)
		respondError(w, http.StatusBadRequest, "Error al leer el cuerpo de la solicitud")
		return
	}

	// Imprimir el cuerpo de la solicitud para depuración
	log.Printf("Cuerpo de la solicitud de login: %s", string(body))

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Decodificar el JSON
	if err := json.Unmarshal(body, &creds); err != nil {
		log.Printf("Error al decodificar JSON: %v", err)
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON: "+err.Error())
		return
	}

	// Validar que se proporcionaron username y password
	if creds.Username == "" || creds.Password == "" {
		log.Printf("Intento de inicio de sesión con credenciales vacías")
		respondError(w, http.StatusBadRequest, "El nombre de usuario y la contraseña son obligatorios")
		return
	}

	log.Printf("Intento de inicio de sesión: username=%s", creds.Username)

	user, err := s.authManager.GetUserByUsername(creds.Username)
	if err != nil {
		log.Printf("Error al obtener usuario: %v", err)
		respondError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}

	log.Printf("Usuario encontrado: %s, verificando contraseña", user.Username)
	log.Printf("Hash almacenado: %s", user.Password)
	log.Printf("Hash calculado: %s", auth.HashPassword(creds.Password))

	if !auth.VerifyPassword(creds.Password, user.Password) {
		log.Printf("Contraseña incorrecta para el usuario: %s", creds.Username)
		respondError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}

	log.Printf("Contraseña verificada correctamente para el usuario: %s", creds.Username)

	// Actualizar la fecha del último inicio de sesión
	// Nota: Esta funcionalidad se implementará en una versión futura

	// Guardar los cambios en el usuario
	s.authManager.SaveUsers()

	token, err := auth.GenerateToken(user)
	if err != nil {
		log.Printf("Error al generar token: %v", err)
		respondError(w, http.StatusInternalServerError, "Error al generar token: "+err.Error())
		return
	}

	log.Printf("Token generado correctamente para el usuario: %s", creds.Username)

	// Preparar la respuesta
	response := map[string]string{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
		"roles":    strings.Join(user.Roles, ","),
	}

	// Imprimir la respuesta para depuración
	respBytes, _ := json.Marshal(response)
	log.Printf("Enviando respuesta de login: %s", string(respBytes))

	// Enviar la respuesta
	respondJSON(w, http.StatusOK, response)
}

// handleHealth maneja la verificación de salud de la API
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"version":   "1.0.0",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": map[string]string{
			"api":      "running",
			"database": "running",
			"p2p":      "running",
		},
		"cors": "enabled",
	})
}

// Manejadores de usuarios y roles

// handleGetUsers maneja la obtención de todos los usuarios
func (s *APIServer) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users := s.authManager.GetAllUsers()

	// No devolver las contraseñas
	type UserResponse struct {
		ID        string        `json:"id"`
		Username  string        `json:"username"`
		Roles     []string      `json:"roles"`
		APIKeys   []auth.APIKey `json:"api_keys"`
		CreatedAt time.Time     `json:"created_at"`
		UpdatedAt time.Time     `json:"updated_at"`
	}

	var response []UserResponse
	for _, user := range users {
		response = append(response, UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Roles:     user.Roles,
			APIKeys:   user.APIKeys,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// handleCreateUser maneja la creación de un usuario
func (s *APIServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		Roles    []string `json:"roles"`
	}

	// Leer el cuerpo de la solicitud
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error al leer el cuerpo de la solicitud: %v", err)
		respondError(w, http.StatusBadRequest, "Error al leer el cuerpo de la solicitud")
		return
	}

	// Imprimir el cuerpo de la solicitud para depuración
	log.Printf("Cuerpo de la solicitud: %s", string(body))

	// Decodificar el JSON
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error al decodificar JSON: %v", err)
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	// Imprimir los datos decodificados para depuración
	log.Printf("Datos decodificados: username=%s, roles=%v", req.Username, req.Roles)

	user, err := s.authManager.CreateUser(req.Username, req.Password, req.Roles)
	if err != nil {
		log.Printf("Error al crear usuario: %v", err)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// No devolver la contraseña
	user.Password = ""
	log.Printf("Usuario creado exitosamente: %s", user.Username)
	respondJSON(w, http.StatusCreated, user)
}

// handleGetUser maneja la obtención de un usuario
func (s *APIServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	user, err := s.authManager.GetUserByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}

	// No devolver la contraseña
	user.Password = ""
	respondJSON(w, http.StatusOK, user)
}

// handleUpdateUser maneja la actualización de un usuario
func (s *APIServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	user, err := s.authManager.UpdateUser(id, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// No devolver la contraseña
	user.Password = ""
	respondJSON(w, http.StatusOK, user)
}

// handleDeleteUser maneja la eliminación de un usuario
func (s *APIServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Obtener el usuario para verificar si es admin
	user, err := s.authManager.GetUserByID(id)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Proteger al usuario admin
	if user.Username == "admin" {
		respondError(w, http.StatusForbidden, "No se puede eliminar el usuario administrador predeterminado")
		return
	}

	// Eliminar el usuario
	if err := s.authManager.DeleteUser(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Usuario eliminado"})
}

// handleCreateAPIKey maneja la creación de una clave API
func (s *APIServer) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		Name      string `json:"name"`
		ValidDays int    `json:"valid_days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	apiKey, err := s.authManager.CreateAPIKey(id, req.Name, req.ValidDays)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, apiKey)
}

// handleRevokeAPIKey maneja la revocación de una clave API
func (s *APIServer) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	token := vars["token"]

	if err := s.authManager.RevokeAPIKey(id, token); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Clave API revocada"})
}

// handleGetRoles maneja la obtención de todos los roles
func (s *APIServer) handleGetRoles(w http.ResponseWriter, r *http.Request) {
	roles := s.authManager.GetAllRoles()
	respondJSON(w, http.StatusOK, roles)
}

// handleCreateRole maneja la creación de un rol
func (s *APIServer) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Permissions []auth.Permission `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	role, err := s.authManager.CreateRole(req.Name, req.Description, req.Permissions)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, role)
}

// handleGetRole maneja la obtención de un rol
func (s *APIServer) handleGetRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	role, exists := s.authManager.Roles[name]
	if !exists {
		respondError(w, http.StatusNotFound, "Rol no encontrado")
		return
	}

	respondJSON(w, http.StatusOK, role)
}

// handleUpdateRole maneja la actualización de un rol
func (s *APIServer) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var req struct {
		Description string            `json:"description"`
		Permissions []auth.Permission `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	role, err := s.authManager.UpdateRole(name, req.Description, req.Permissions)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, role)
}

// handleDeleteRole maneja la eliminación de un rol
func (s *APIServer) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := s.authManager.DeleteRole(name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Rol eliminado"})
}

// Manejadores de la base de datos

// handleListCollections maneja la obtención de todas las colecciones
func (s *APIServer) handleListCollections(w http.ResponseWriter, r *http.Request) {
	// Implementar la lógica para listar colecciones
	// Por ahora, devolvemos un mensaje de no implementado
	respondError(w, http.StatusNotImplemented, "Funcionalidad no implementada")
}

// handleGetCollection maneja la obtención de todos los documentos de una colección
func (s *APIServer) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collection := vars["collection"]

	docs, err := s.db.GetAllDocuments(collection)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, docs)
}

// handleCreateDocument maneja la creación de un documento
func (s *APIServer) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collection := vars["collection"]

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	doc, err := s.db.CreateDocument(collection, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, doc)
}

// handleGetDocument maneja la obtención de un documento
func (s *APIServer) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	doc, err := s.db.GetDocument(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// handleUpdateDocument maneja la actualización de un documento
func (s *APIServer) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		respondError(w, http.StatusBadRequest, "Error al decodificar JSON")
		return
	}

	doc, err := s.db.UpdateDocument(id, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// handleDeleteDocument maneja la eliminación de un documento
func (s *APIServer) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.db.DeleteDocument(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Documento eliminado"})
}

// Manejadores de backup y restauración

// handleListBackups maneja la obtención de todas las copias de seguridad
func (s *APIServer) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := s.db.ListBackups()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, backups)
}

// handleCreateBackup maneja la creación de una copia de seguridad
func (s *APIServer) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	backupName, err := s.db.CreateBackup()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"backup_name": backupName})
}

// handleRestoreBackup maneja la restauración de una copia de seguridad
func (s *APIServer) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := s.db.RestoreFromBackup(name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Base de datos restaurada"})
}

// handleDeleteBackup maneja la eliminación de una copia de seguridad
func (s *APIServer) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := s.db.DeleteBackup(name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Copia de seguridad eliminada"})
}

// Manejadores de recuperación de contraseñas

// handleForgotPassword maneja la solicitud de recuperación de contraseña
func (s *APIServer) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	// Nota: Esta funcionalidad se implementará en una versión futura
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Funcionalidad de recuperación de contraseña no implementada aún",
	})
}

// handleResetPassword maneja el restablecimiento de contraseña
func (s *APIServer) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	// Nota: Esta funcionalidad se implementará en una versión futura
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Funcionalidad de restablecimiento de contraseña no implementada aún",
	})
}
