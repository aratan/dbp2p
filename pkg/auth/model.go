package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("no autorizado")
	ErrForbidden    = errors.New("acceso prohibido")
	jwtSecret       = []byte("dbp2p_secret_key") // En producción, usar una clave segura y configurable
)

// UserOptions representa las opciones para crear un usuario
type UserOptions struct {
	Username string
	Password string
	FullName string
	Email    string
	Roles    []string
	Active   bool
}

// User representa un usuario en el sistema
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password_hash"` // Almacenar solo el hash
	FullName  string    `json:"full_name,omitempty"`
	Email     string    `json:"email,omitempty"`
	Roles     []string  `json:"roles"`
	APIKeys   []APIKey  `json:"api_keys"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey representa una clave API para acceso programático
type APIKey struct {
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Role representa un rol en el sistema
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsSystem    bool         `json:"is_system"` // Indica si es un rol del sistema (no se puede eliminar)
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Permission representa un permiso para realizar acciones en un recurso
type Permission struct {
	Resource string   `json:"resource"` // Colección o "*" para todos los recursos
	Actions  []string `json:"actions"`  // "read", "write", "delete", "admin" o "*" para todas las acciones
}

// NewUser crea un nuevo usuario
func NewUser(opts UserOptions) *User {
	now := time.Now()
	return &User{
		ID:        uuid.New().String(),
		Username:  opts.Username,
		Password:  HashPassword(opts.Password),
		FullName:  opts.FullName,
		Email:     opts.Email,
		Roles:     opts.Roles,
		Active:    opts.Active,
		APIKeys:   []APIKey{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// HashPassword genera un hash seguro para la contraseña
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPassword verifica si la contraseña coincide con el hash
func VerifyPassword(password, hash string) bool {
	return HashPassword(password) == hash
}

// GenerateToken genera un token JWT para el usuario
func GenerateToken(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"roles":    user.Roles,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// VerifyToken verifica si un token JWT es válido
func VerifyToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrUnauthorized
}

// GenerateAPIKey genera una nueva clave API para el usuario
func (u *User) GenerateAPIKey(name string, validDays int) APIKey {
	now := time.Now()
	apiKey := APIKey{
		Token:     uuid.New().String(),
		Name:      name,
		CreatedAt: now,
		ExpiresAt: now.AddDate(0, 0, validDays),
	}
	u.APIKeys = append(u.APIKeys, apiKey)
	u.UpdatedAt = now
	return apiKey
}

// RevokeAPIKey revoca una clave API
func (u *User) RevokeAPIKey(token string) bool {
	for i, key := range u.APIKeys {
		if key.Token == token {
			u.APIKeys = append(u.APIKeys[:i], u.APIKeys[i+1:]...)
			u.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// CheckPermission verifica si un usuario tiene permiso para realizar una acción
func CheckPermission(user *User, resource, action string, roles map[string]*Role) bool {
	// El usuario admin tiene todos los permisos
	for _, roleName := range user.Roles {
		if roleName == "admin" {
			return true
		}
	}

	// Verificar permisos específicos
	for _, roleName := range user.Roles {
		role, exists := roles[roleName]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			if (perm.Resource == resource || perm.Resource == "*") && contains(perm.Actions, action) {
				return true
			}
		}
	}

	return false
}

// contains verifica si un slice contiene un elemento
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item || s == "*" {
			return true
		}
	}
	return false
}
