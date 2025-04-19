package p2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

// CryptoService representa el servicio de cifrado
type CryptoService struct {
	encryptionKey []byte
	mutex         sync.RWMutex
}

// NewCryptoService crea un nuevo servicio de cifrado
func NewCryptoService(key []byte) *CryptoService {
	// Si no se proporciona una clave, generar una aleatoria
	if key == nil || len(key) == 0 {
		key = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			panic(fmt.Errorf("error al generar clave de cifrado: %v", err))
		}
	}

	// Asegurarse de que la clave tenga el tamaño correcto
	if len(key) != 32 {
		newKey := make([]byte, 32)
		copy(newKey, key)
		key = newKey
	}

	return &CryptoService{
		encryptionKey: key,
	}
}

// Encrypt cifra un mensaje usando AES-GCM
func (s *CryptoService) Encrypt(plaintext string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("error al crear cifrador: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("error al crear GCM: %v", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("error al generar nonce: %v", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt descifra un mensaje cifrado con AES-GCM
func (s *CryptoService) Decrypt(ciphertext string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("error al crear cifrador: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("error al crear GCM: %v", err)
	}

	data, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("error al decodificar base64: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext demasiado corto")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintextBytes, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("error al descifrar: %v", err)
	}

	return string(plaintextBytes), nil
}

// EncryptMessage cifra un mensaje para transmisión segura
func (s *CryptoService) EncryptMessage(message []byte) ([]byte, error) {
	encrypted, err := s.Encrypt(string(message))
	if err != nil {
		return nil, err
	}
	return []byte(encrypted), nil
}

// DecryptMessage descifra un mensaje recibido
func (s *CryptoService) DecryptMessage(message []byte) ([]byte, error) {
	decrypted, err := s.Decrypt(string(message))
	if err != nil {
		return nil, err
	}
	return []byte(decrypted), nil
}

// SetEncryptionKey establece una nueva clave de cifrado
func (s *CryptoService) SetEncryptionKey(key []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(key) != 32 {
		return fmt.Errorf("la clave debe tener 32 bytes (256 bits)")
	}

	s.encryptionKey = key
	return nil
}

// GenerateRandomKey genera una clave aleatoria de 256 bits
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("error al generar clave aleatoria: %v", err)
	}
	return key, nil
}
