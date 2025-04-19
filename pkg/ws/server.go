package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aratan/dbp2p/pkg/auth"
	"github.com/aratan/dbp2p/pkg/binary"
	"github.com/aratan/dbp2p/pkg/db"
	"github.com/gorilla/websocket"
)

// EventType representa el tipo de evento WebSocket
type EventType string

const (
	// Tipos de eventos
	EventCreate EventType = "create"
	EventUpdate EventType = "update"
	EventDelete EventType = "delete"
)

// WSServer representa el servidor WebSocket
type WSServer struct {
	clients       map[*Client]bool
	broadcast     chan []byte
	register      chan *Client
	unregister    chan *Client
	db            *db.Database
	authManager   *auth.AuthManager
	mutex         sync.RWMutex
	binaryManager *binary.BinaryManager
}

// Client representa un cliente WebSocket
type Client struct {
	server  *WSServer
	conn    *websocket.Conn
	send    chan []byte
	user    *auth.User
	isAdmin bool
}

// Message representa un mensaje WebSocket
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewWSServer crea un nuevo servidor WebSocket
func NewWSServer(database *db.Database, authManager *auth.AuthManager) *WSServer {
	return &WSServer{
		clients:     make(map[*Client]bool),
		broadcast:   make(chan []byte),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		db:          database,
		authManager: authManager,
	}
}

// SetBinaryManager establece el gestor de binarios
func (s *WSServer) SetBinaryManager(binaryManager *binary.BinaryManager) {
	s.binaryManager = binaryManager
}

// getBinaryManager obtiene el gestor de binarios
func (s *WSServer) getBinaryManager() *binary.BinaryManager {
	return s.binaryManager
}

// Start inicia el servidor WebSocket
func (s *WSServer) Start() {
	go s.run()
}

// run maneja las operaciones del servidor WebSocket
func (s *WSServer) run() {
	for {
		select {
		case client := <-s.register:
			s.mutex.Lock()
			s.clients[client] = true
			s.mutex.Unlock()
			log.Printf("Cliente WebSocket registrado: %v", client.conn.RemoteAddr())

		case client := <-s.unregister:
			s.mutex.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				log.Printf("Cliente WebSocket desregistrado: %v", client.conn.RemoteAddr())
			}
			s.mutex.Unlock()

		case message := <-s.broadcast:
			s.mutex.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.mutex.RUnlock()
		}
	}
}

// ServeWS inicia el servidor HTTP para WebSocket
func (s *WSServer) ServeWS(port int) error {
	addr := fmt.Sprintf(":%d", port)
	http.HandleFunc("/ws", s.handleWebSocket)
	log.Printf("Servidor WebSocket escuchando en %s", addr)
	return http.ListenAndServe(addr, nil)
}

// Configuración del upgrader WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Permitir conexiones desde cualquier origen
	},
}

// handleWebSocket maneja las conexiones WebSocket
func (s *WSServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Autenticar al usuario
	var user *auth.User
	var isAdmin bool

	// Verificar token de autenticación
	authHeader := r.Header.Get("Sec-WebSocket-Protocol")
	if authHeader != "" {
		// El token se pasa como protocolo WebSocket
		claims, err := auth.VerifyToken(authHeader)
		if err == nil {
			userID, ok := claims["user_id"].(string)
			if ok {
				user, err = s.authManager.GetUserByID(userID)
				if err == nil {
					// Verificar si el usuario es administrador
					for _, role := range user.Roles {
						if role == "admin" {
							isAdmin = true
							break
						}
					}
				}
			}
		}
	}

	// Actualizar el upgrader para incluir el protocolo
	upgrader.Subprotocols = []string{authHeader}

	// Actualizar la conexión HTTP a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error al actualizar a WebSocket: %v", err)
		return
	}

	// Crear cliente
	client := &Client{
		server:  s,
		conn:    conn,
		send:    make(chan []byte, 256),
		user:    user,
		isAdmin: isAdmin,
	}

	// Registrar cliente
	s.register <- client

	// Iniciar goroutines para leer y escribir mensajes
	go client.readPump()
	go client.writePump()

	// Enviar mensaje de bienvenida
	welcomeMsg := map[string]interface{}{
		"type":      "welcome",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Bienvenido al servidor WebSocket de DBP2P",
	}

	if user != nil {
		welcomeMsg["user"] = map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"roles":    user.Roles,
			"is_admin": isAdmin,
		}
	}

	welcomeJSON, _ := json.Marshal(welcomeMsg)
	client.send <- welcomeJSON
}

// readPump bombea mensajes desde la conexión WebSocket al hub
func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error de lectura WebSocket: %v", err)
			}
			break
		}

		// Procesar mensaje
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error al deserializar mensaje: %v", err)
			continue
		}

		// Manejar diferentes tipos de mensajes
		switch msg.Type {
		case "ping":
			// Responder con pong
			pongMsg := map[string]interface{}{
				"type":      "pong",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			pongJSON, _ := json.Marshal(pongMsg)
			c.send <- pongJSON

		case "query":
			// Manejar consulta a la base de datos
			c.handleQuery(msg.Payload)

		case "create":
			// Manejar creación de documento
			c.handleCreate(msg.Payload)

		case "update":
			// Manejar actualización de documento
			c.handleUpdate(msg.Payload)

		case "delete":
			// Manejar eliminación de documento
			c.handleDelete(msg.Payload)

		case "get":
			// Manejar obtención de documento
			c.handleGet(msg.Payload)

		case "binary_upload":
			// Manejar subida de archivo binario
			c.handleBinaryUpload(msg.Payload)

		case "binary_download":
			// Manejar descarga de archivo binario
			c.handleBinaryDownload(msg.Payload)

		case "binary_list":
			// Manejar lista de archivos binarios
			c.handleBinaryList(msg.Payload)

		default:
			log.Printf("Tipo de mensaje desconocido: %s", msg.Type)
		}
	}
}

// writePump bombea mensajes desde el hub a la conexión WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// El canal se cerró
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Añadir mensajes en cola al mensaje actual
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// PublishEvent publica un evento a todos los clientes
func (s *WSServer) PublishEvent(eventType EventType, collection string, documentID string, document *db.Document) {
	event := map[string]interface{}{
		"type":        "event",
		"event_type":  eventType,
		"collection":  collection,
		"document_id": documentID,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	if document != nil {
		event["document"] = document
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error al serializar evento: %v", err)
		return
	}

	s.broadcast <- eventJSON
}

// Manejadores de mensajes

// handleQuery maneja consultas a la base de datos
func (c *Client) handleQuery(payload json.RawMessage) {
	var req struct {
		Collection string         `json:"collection"`
		Query      map[string]any `json:"query"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al deserializar consulta: %v", err))
		return
	}

	// Ejecutar consulta
	docs, err := c.server.db.QueryDocuments(req.Collection, req.Query)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al ejecutar consulta: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":       "query_response",
		"collection": req.Collection,
		"count":      len(docs),
		"documents":  docs,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al serializar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleCreate maneja la creación de documentos
func (c *Client) handleCreate(payload json.RawMessage) {
	var req struct {
		Collection string         `json:"collection"`
		Data       map[string]any `json:"data"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al deserializar datos: %v", err))
		return
	}

	// Crear documento
	doc, err := c.server.db.CreateDocument(req.Collection, req.Data)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al crear documento: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":     "create_response",
		"success":  true,
		"document": doc,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al serializar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleUpdate maneja la actualización de documentos
func (c *Client) handleUpdate(payload json.RawMessage) {
	var req struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al deserializar datos: %v", err))
		return
	}

	// Actualizar documento
	doc, err := c.server.db.UpdateDocument(req.ID, req.Data)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al actualizar documento: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":     "update_response",
		"success":  true,
		"document": doc,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al serializar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleDelete maneja la eliminación de documentos
func (c *Client) handleDelete(payload json.RawMessage) {
	var req struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al deserializar datos: %v", err))
		return
	}

	// Eliminar documento
	if err := c.server.db.DeleteDocument(req.ID); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al eliminar documento: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":    "delete_response",
		"success": true,
		"id":      req.ID,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al serializar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// handleGet maneja la obtención de documentos
func (c *Client) handleGet(payload json.RawMessage) {
	var req struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al deserializar datos: %v", err))
		return
	}

	// Obtener documento
	doc, err := c.server.db.GetDocument(req.ID)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al obtener documento: %v", err))
		return
	}

	// Enviar respuesta
	response := map[string]interface{}{
		"type":     "get_response",
		"success":  true,
		"document": doc,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.sendErrorMessage(fmt.Sprintf("Error al serializar respuesta: %v", err))
		return
	}

	c.send <- responseJSON
}

// sendErrorMessage envía un mensaje de error al cliente
func (c *Client) sendErrorMessage(message string) {
	response := map[string]interface{}{
		"type":    "error",
		"success": false,
		"message": message,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return
	}

	c.send <- responseJSON
}

// Los métodos handleBinaryUpload, handleBinaryDownload y handleBinaryList
// están implementados en ws_binary_handler.go
