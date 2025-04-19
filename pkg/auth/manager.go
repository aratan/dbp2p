package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuthManager gestiona la autenticación y autorización
type AuthManager struct {
	Users    map[string]*User // Mapa de ID a usuario
	Roles    map[string]*Role // Mapa de nombre a rol
	dataDir  string           // Directorio de datos
	userFile string           // Archivo de usuarios
	roleFile string           // Archivo de roles
	mutex    sync.RWMutex
}

// NewAuthManager crea un nuevo gestor de autenticación
func NewAuthManager(dataDir string) (*AuthManager, error) {
	// Crear directorio de datos si no existe
	authDir := filepath.Join(dataDir, "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio de autenticación: %v", err)
	}

	userFile := filepath.Join(authDir, "users.json")
	roleFile := filepath.Join(authDir, "roles.json")

	manager := &AuthManager{
		Users:    make(map[string]*User),
		Roles:    make(map[string]*Role),
		dataDir:  dataDir,
		userFile: userFile,
		roleFile: roleFile,
	}

	// Cargar roles predefinidos si no existe el archivo
	if _, err := os.Stat(roleFile); os.IsNotExist(err) {
		manager.initDefaultRoles()
	} else {
		if err := manager.loadRoles(); err != nil {
			return nil, err
		}
	}

	// Cargar usuarios si existe el archivo
	if _, err := os.Stat(userFile); !os.IsNotExist(err) {
		if err := manager.loadUsers(); err != nil {
			return nil, err
		}
	}

	// Crear usuario admin si no existe
	if len(manager.Users) == 0 {
		manager.createAdminUser()
	}

	return manager, nil
}

// initDefaultRoles inicializa los roles predefinidos
func (am *AuthManager) initDefaultRoles() {
	now := time.Now()

	// Rol de administrador
	adminRole := &Role{
		Name:        "admin",
		Description: "Administrador con acceso completo",
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"*"}},
		},
		IsSystem:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Rol de lectura
	readerRole := &Role{
		Name:        "reader",
		Description: "Usuario con permisos de solo lectura",
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"read"}},
		},
		IsSystem:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Rol de escritor
	writerRole := &Role{
		Name:        "writer",
		Description: "Usuario con permisos de lectura y escritura",
		Permissions: []Permission{
			{Resource: "*", Actions: []string{"read", "write"}},
		},
		IsSystem:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Rol de gestor de usuarios
	userManagerRole := &Role{
		Name:        "user_manager",
		Description: "Gestor de usuarios con permisos para administrar usuarios pero no datos",
		Permissions: []Permission{
			{Resource: "users", Actions: []string{"read", "write", "delete"}},
			{Resource: "roles", Actions: []string{"read"}},
		},
		IsSystem:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Rol de gestor de copias de seguridad
	backupManagerRole := &Role{
		Name:        "backup_manager",
		Description: "Gestor de copias de seguridad con permisos para crear y restaurar backups",
		Permissions: []Permission{
			{Resource: "backups", Actions: []string{"read", "write", "delete"}},
		},
		IsSystem:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	am.Roles["admin"] = adminRole
	am.Roles["reader"] = readerRole
	am.Roles["writer"] = writerRole
	am.Roles["user_manager"] = userManagerRole
	am.Roles["backup_manager"] = backupManagerRole

	// Guardar roles
	am.saveRoles()
}

// createAdminUser crea el usuario administrador predeterminado
func (am *AuthManager) createAdminUser() {
	admin := NewUser(UserOptions{
		Username: "admin",
		Password: "admin123",
		FullName: "Administrador",
		Email:    "admin@dbp2p.local",
		Roles:    []string{"admin"},
		Active:   true,
	})
	am.Users[admin.ID] = admin
	am.saveUsers()
	fmt.Println("Usuario administrador creado con credenciales predeterminadas (admin/admin123)")
	fmt.Println("Se recomienda cambiar la contraseña inmediatamente")
}

// loadUsers carga los usuarios desde el archivo
func (am *AuthManager) loadUsers() error {
	data, err := os.ReadFile(am.userFile)
	if err != nil {
		return err
	}

	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	for _, user := range users {
		am.Users[user.ID] = user
	}

	return nil
}

// SaveUsers guarda los usuarios en el archivo (método público)
func (am *AuthManager) SaveUsers() error {
	return am.saveUsers()
}

// saveUsers guarda los usuarios en el archivo (método interno)
func (am *AuthManager) saveUsers() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	log.Printf("Guardando usuarios en el archivo: %s", am.userFile)

	var users []*User
	for _, user := range am.Users {
		users = append(users, user)
	}

	log.Printf("Usuarios a guardar: %d", len(users))
	for i, user := range users {
		log.Printf("Usuario %d: ID=%s, Username=%s, Roles=%v", i, user.ID, user.Username, user.Roles)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		log.Printf("Error al serializar usuarios: %v", err)
		return err
	}

	log.Printf("Escribiendo %d bytes en el archivo de usuarios", len(data))

	// Verificar si el directorio existe
	dir := filepath.Dir(am.userFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("El directorio %s no existe, creándolo", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Error al crear directorio: %v", err)
			return err
		}
	}

	// Crear un archivo temporal primero
	tempFile := am.userFile + ".tmp"
	log.Printf("Creando archivo temporal: %s", tempFile)
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		log.Printf("Error al escribir archivo temporal: %v", err)
		return err
	}

	// Renombrar el archivo temporal al archivo final
	log.Printf("Renombrando archivo temporal a: %s", am.userFile)
	if err := os.Rename(tempFile, am.userFile); err != nil {
		log.Printf("Error al renombrar archivo temporal: %v", err)
		return err
	}

	log.Printf("Usuarios guardados exitosamente")

	// Verificar que el archivo se haya guardado correctamente
	if _, err := os.Stat(am.userFile); os.IsNotExist(err) {
		log.Printf("Error: el archivo de usuarios no existe después de guardarlo")
		return errors.New("el archivo de usuarios no existe después de guardarlo")
	}

	// Leer el archivo para verificar que se haya guardado correctamente
	readData, err := os.ReadFile(am.userFile)
	if err != nil {
		log.Printf("Error al leer archivo de usuarios para verificación: %v", err)
		return err
	}

	log.Printf("Verificación: se leyeron %d bytes del archivo de usuarios", len(readData))

	return nil
}

// loadRoles carga los roles desde el archivo
func (am *AuthManager) loadRoles() error {
	data, err := os.ReadFile(am.roleFile)
	if err != nil {
		return err
	}

	var roles []*Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return err
	}

	for _, role := range roles {
		am.Roles[role.Name] = role
	}

	return nil
}

// saveRoles guarda los roles en el archivo
func (am *AuthManager) saveRoles() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	var roles []*Role
	for _, role := range am.Roles {
		roles = append(roles, role)
	}

	data, err := json.MarshalIndent(roles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(am.roleFile, data, 0644)
}

// CreateUser crea un nuevo usuario
func (am *AuthManager) CreateUser(username, password string, roles []string) (*User, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	// Imprimir para depuración
	log.Printf("Creando usuario: username=%s, roles=%v", username, roles)

	// Verificar si el nombre de usuario ya existe
	for _, user := range am.Users {
		if user.Username == username {
			log.Printf("Error: el nombre de usuario %s ya existe", username)
			return nil, errors.New("el nombre de usuario ya existe")
		}
	}

	// Verificar que los roles existan
	for _, roleName := range roles {
		if _, exists := am.Roles[roleName]; !exists {
			log.Printf("Error: el rol %s no existe", roleName)
			return nil, fmt.Errorf("el rol %s no existe", roleName)
		}
	}

	// Verificar que el directorio de usuarios exista
	dir := filepath.Dir(am.userFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Printf("El directorio %s no existe, creándolo", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Error al crear directorio: %v", err)
			return nil, err
		}
	}

	// Crear el usuario
	user := NewUser(UserOptions{
		Username: username,
		Password: password,
		Roles:    roles,
		Active:   true,
	})
	log.Printf("Usuario creado con ID: %s", user.ID)
	am.Users[user.ID] = user

	// Guardar usuarios
	if err := am.saveUsers(); err != nil {
		log.Printf("Error al guardar usuarios: %v", err)
		// Eliminar el usuario de la memoria si no se pudo guardar
		delete(am.Users, user.ID)
		return nil, err
	}

	// Verificar que el usuario se haya guardado correctamente
	log.Printf("Verificando que el usuario se haya guardado correctamente")
	data, err := os.ReadFile(am.userFile)
	if err != nil {
		log.Printf("Error al leer archivo de usuarios para verificación: %v", err)
		return user, nil // Devolvemos el usuario de todos modos, ya que se creó en memoria
	}

	// Verificar que el archivo contenga el nombre de usuario
	if !strings.Contains(string(data), username) {
		log.Printf("Advertencia: el usuario %s no aparece en el archivo de usuarios", username)
	}

	log.Printf("Usuario guardado exitosamente: %s", username)
	return user, nil
}

// UpdateUser actualiza un usuario existente
func (am *AuthManager) UpdateUser(id string, updates map[string]interface{}) (*User, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	user, exists := am.Users[id]
	if !exists {
		return nil, errors.New("usuario no encontrado")
	}

	// Actualizar campos
	if username, ok := updates["username"].(string); ok {
		// Verificar si el nombre de usuario ya existe
		for _, u := range am.Users {
			if u.Username == username && u.ID != id {
				return nil, errors.New("el nombre de usuario ya existe")
			}
		}
		user.Username = username
	}

	if password, ok := updates["password"].(string); ok {
		user.Password = HashPassword(password)
	}

	if roles, ok := updates["roles"].([]string); ok {
		// Verificar que los roles existan
		for _, roleName := range roles {
			if _, exists := am.Roles[roleName]; !exists {
				return nil, fmt.Errorf("el rol %s no existe", roleName)
			}
		}
		user.Roles = roles
	}

	user.UpdatedAt = time.Now()

	// Guardar usuarios
	if err := am.saveUsers(); err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser elimina un usuario
func (am *AuthManager) DeleteUser(id string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.Users[id]; !exists {
		return errors.New("usuario no encontrado")
	}

	delete(am.Users, id)

	// Guardar usuarios
	return am.saveUsers()
}

// GetUserByUsername obtiene un usuario por su nombre de usuario
func (am *AuthManager) GetUserByUsername(username string) (*User, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	for _, user := range am.Users {
		if user.Username == username {
			return user, nil
		}
	}

	return nil, errors.New("usuario no encontrado")
}

// GetUserByID obtiene un usuario por su ID
func (am *AuthManager) GetUserByID(id string) (*User, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	user, exists := am.Users[id]
	if !exists {
		return nil, errors.New("usuario no encontrado")
	}

	return user, nil
}

// GetUserByAPIKey obtiene un usuario por su clave API
func (am *AuthManager) GetUserByAPIKey(apiKey string) (*User, error) {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	for _, user := range am.Users {
		for _, key := range user.APIKeys {
			if key.Token == apiKey && key.ExpiresAt.After(time.Now()) {
				return user, nil
			}
		}
	}

	return nil, errors.New("clave API no válida o expirada")
}

// CreateAPIKey crea una nueva clave API para un usuario
func (am *AuthManager) CreateAPIKey(userID, name string, validDays int) (APIKey, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	user, exists := am.Users[userID]
	if !exists {
		return APIKey{}, errors.New("usuario no encontrado")
	}

	apiKey := user.GenerateAPIKey(name, validDays)

	// Guardar usuarios
	if err := am.saveUsers(); err != nil {
		return APIKey{}, err
	}

	return apiKey, nil
}

// RevokeAPIKey revoca una clave API
func (am *AuthManager) RevokeAPIKey(userID, token string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	user, exists := am.Users[userID]
	if !exists {
		return errors.New("usuario no encontrado")
	}

	if !user.RevokeAPIKey(token) {
		return errors.New("clave API no encontrada")
	}

	// Guardar usuarios
	return am.saveUsers()
}

// CreateRole crea un nuevo rol
func (am *AuthManager) CreateRole(name, description string, permissions []Permission) (*Role, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if _, exists := am.Roles[name]; exists {
		return nil, errors.New("el rol ya existe")
	}

	now := time.Now()
	role := &Role{
		Name:        name,
		Description: description,
		Permissions: permissions,
		IsSystem:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	am.Roles[name] = role

	// Guardar roles
	if err := am.saveRoles(); err != nil {
		return nil, err
	}

	return role, nil
}

// UpdateRole actualiza un rol existente
func (am *AuthManager) UpdateRole(name string, description string, permissions []Permission) (*Role, error) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	role, exists := am.Roles[name]
	if !exists {
		return nil, errors.New("rol no encontrado")
	}

	// No permitir modificar roles del sistema excepto la descripción
	if role.IsSystem {
		// Solo permitir actualizar la descripción para roles del sistema
		role.Description = description
	} else {
		// Para roles personalizados, permitir actualizar todo
		role.Description = description
		role.Permissions = permissions
	}

	// Actualizar fecha de modificación
	role.UpdatedAt = time.Now()

	// Guardar roles
	if err := am.saveRoles(); err != nil {
		return nil, err
	}

	return role, nil
}

// DeleteRole elimina un rol
func (am *AuthManager) DeleteRole(name string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	role, exists := am.Roles[name]
	if !exists {
		return errors.New("rol no encontrado")
	}

	// No permitir eliminar roles del sistema
	if role.IsSystem {
		return errors.New("no se puede eliminar un rol del sistema")
	}

	// Verificar si algún usuario tiene este rol
	for _, user := range am.Users {
		for _, roleName := range user.Roles {
			if roleName == name {
				return errors.New("no se puede eliminar el rol porque está asignado a usuarios")
			}
		}
	}

	delete(am.Roles, name)

	// Guardar roles
	return am.saveRoles()
}

// CheckUserPermission verifica si un usuario tiene permiso para realizar una acción
func (am *AuthManager) CheckUserPermission(userID, resource, action string) bool {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	user, exists := am.Users[userID]
	if !exists {
		return false
	}

	return CheckPermission(user, resource, action, am.Roles)
}

// GetAllUsers obtiene todos los usuarios
func (am *AuthManager) GetAllUsers() []*User {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var users []*User
	for _, user := range am.Users {
		users = append(users, user)
	}

	return users
}

// GetAllRoles obtiene todos los roles
func (am *AuthManager) GetAllRoles() []*Role {
	am.mutex.RLock()
	defer am.mutex.RUnlock()

	var roles []*Role
	for _, role := range am.Roles {
		roles = append(roles, role)
	}

	return roles
}
