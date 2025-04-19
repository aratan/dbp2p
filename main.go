package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/aratan/dbp2p/pkg/api"
	"github.com/aratan/dbp2p/pkg/config"
	"github.com/aratan/dbp2p/pkg/db"
	"github.com/aratan/dbp2p/pkg/p2p"
	"github.com/aratan/dbp2p/pkg/ws"
)

func main() {
	// Parsear flags de línea de comandos
	configFile := flag.String("config", "config.yaml", "Ruta al archivo de configuración")
	apiPort := flag.Int("api-port", 0, "Puerto para la API REST (anula la configuración)")
	wsPort := flag.Int("ws-port", 0, "Puerto para WebSocket (anula la configuración)")
	enableAPI := flag.Bool("api", true, "Habilitar API REST (anula la configuración)")
	enableWS := flag.Bool("ws", true, "Habilitar WebSocket (anula la configuración)")
	flag.Parse()

	// Cargar configuración
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Printf("Error al cargar configuración: %v. Usando valores predeterminados.", err)
		cfg = config.GetConfig()
	}

	// Anular configuración con flags de línea de comandos
	if *apiPort > 0 {
		cfg.API.Port = *apiPort
	}
	if *wsPort > 0 {
		cfg.WebSocket.Port = *wsPort
	}

	// Comprobar si los flags fueron especificados explícitamente
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-api" || arg == "--api" || strings.HasPrefix(arg, "-api=") || strings.HasPrefix(arg, "--api=") {
			cfg.API.Enabled = *enableAPI
		}
		if arg == "-ws" || arg == "--ws" || strings.HasPrefix(arg, "-ws=") || strings.HasPrefix(arg, "--ws=") {
			cfg.WebSocket.Enabled = *enableWS
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicializar el nodo P2P
	node, err := p2p.NewNode(ctx)
	if err != nil {
		log.Fatalf("Error al crear nodo P2P: %v", err)
	}
	defer node.Close()

	// Obtener ID del nodo
	nodeID := node.Host.ID().String()
	fmt.Printf("Nodo P2P inicializado con ID: %s\n", nodeID)

	// Inicializar Pub/Sub (ya no necesitamos la referencia directa)
	_, err = p2p.NewPubSub(ctx, node)
	if err != nil {
		log.Fatalf("Error al configurar PubSub: %v", err)
	}

	// Crear directorio de datos si no existe
	dataDir := cfg.General.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Error al crear directorio de datos: %v", err)
	}

	// Inicializar la base de datos con persistencia
	database, err := db.NewDatabaseWithPersistence(dataDir)
	if err != nil {
		log.Printf("Error al inicializar base de datos con persistencia: %v. Usando modo sin persistencia.", err)
		database = db.NewDatabase()
	} else {
		log.Printf("Base de datos inicializada con persistencia en: %s", dataDir)
	}

	// Inicializar el gestor de autenticación con configuración
	authManager, err := auth.NewAuthManager(dataDir)
	if err != nil {
		log.Fatalf("Error al inicializar el gestor de autenticación: %v", err)
	}

	// Configurar el secreto JWT desde la configuración
	// Nota: Estas funciones deberían implementarse en el paquete auth

	// Configurar la base de datos en el nodo P2P para habilitar la sincronización
	if err := node.SetDatabase(database); err != nil {
		log.Fatalf("Error al configurar la base de datos en el nodo P2P: %v", err)
	}

	// Obtener la referencia a la sincronización
	dbSync := node.Sync

	// Mostrar información de sincronización
	log.Println("Sincronización de base de datos inicializada correctamente")

	// Iniciar servidores si están habilitados
	if cfg.API.Enabled {
		// Inicializar y arrancar el servidor API
		apiServer := api.NewAPIServer(database, authManager)
		go func() {
			if err := apiServer.Start(cfg.API.Port); err != nil {
				log.Fatalf("Error al iniciar el servidor API: %v", err)
			}
		}()
		log.Printf("Servidor API iniciado en el puerto %d", cfg.API.Port)
	}

	if cfg.WebSocket.Enabled {
		// Inicializar y arrancar el servidor WebSocket
		wsServer := ws.NewWSServer(database, authManager)
		wsServer.Start()

		// Registrar callback para eventos de la base de datos
		database.RegisterEventCallback(func(eventType string, collection string, documentID string, document *db.Document) {
			wsServer.PublishEvent(ws.EventType(eventType), collection, documentID, document)
		})

		go func() {
			if err := wsServer.ServeWS(cfg.WebSocket.Port); err != nil {
				log.Fatalf("Error al iniciar el servidor WebSocket: %v", err)
			}
		}()
		log.Printf("Servidor WebSocket iniciado en el puerto %d", cfg.WebSocket.Port)
	}

	// Configurar copias de seguridad automáticas si están habilitadas
	if cfg.Database.Backup.AutoBackup {
		go func() {
			// Implementar la función de copia de seguridad automática
			interval := time.Duration(cfg.Database.Backup.Interval) * time.Second
			backupTicker := time.NewTicker(interval)
			defer backupTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-backupTicker.C:
					// Crear copia de seguridad
					backupName, err := database.CreateBackup()
					if err != nil {
						log.Printf("Error al crear copia de seguridad automática: %v", err)
						continue
					}
					log.Printf("Copia de seguridad automática creada: %s", backupName)

					// Eliminar copias de seguridad antiguas si se supera el límite
					if cfg.Database.Backup.MaxBackups > 0 {
						backups, err := database.ListBackups()
						if err != nil {
							log.Printf("Error al listar copias de seguridad: %v", err)
							continue
						}

						// Eliminar las copias de seguridad más antiguas si se supera el límite
						if len(backups) > cfg.Database.Backup.MaxBackups {
							// Ordenar por nombre (que contiene timestamp)
							sort.Strings(backups)

							// Eliminar las más antiguas
							toDelete := len(backups) - cfg.Database.Backup.MaxBackups
							for i := range backups[:toDelete] {
								oldBackup := backups[i]
								backupPath := filepath.Join(dataDir, "backups", oldBackup)
								if err := os.RemoveAll(backupPath); err != nil {
									log.Printf("Error al eliminar copia de seguridad antigua %s: %v", oldBackup, err)
								} else {
									log.Printf("Copia de seguridad antigua eliminada: %s", oldBackup)
								}
							}
						}
					}
				}
			}
		}()
	}

	// Manejar modo CLI o esperar señales de terminación
	if len(os.Args) == 1 || (len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-")) {
		// Modo CLI
		runCLI(database, dbSync)
	} else {
		// Esperar señales de terminación
		log.Println("Servidores iniciados. Presiona Ctrl+C para salir.")
		waitForSignal()
	}
}

func runCLI(database *db.Database, _ *db.DBSync) {
	// Añadir comando para sincronizar todos los documentos
	fmt.Println("  sync - Sincronizar todos los documentos con la red")
	// Iniciar la interfaz de línea de comandos
	fmt.Println("Base de datos NoSQL descentralizada P2P")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Println("Comandos disponibles:")
	fmt.Println("  create <colección> <json_data> - Crear un nuevo documento")
	fmt.Println("  get <id> - Obtener un documento por ID")
	fmt.Println("  query <colección> <json_query> - Buscar documentos")
	fmt.Println("  update <id> <json_data> - Actualizar un documento")
	fmt.Println("  delete <id> - Eliminar un documento")
	fmt.Println("  list <colección> - Listar todos los documentos de una colección")
	fmt.Println("  backup - Crear una copia de seguridad de la base de datos")
	fmt.Println("  restore <nombre_backup> - Restaurar la base de datos desde una copia de seguridad")
	fmt.Println("  list_backups - Listar todas las copias de seguridad disponibles")
	fmt.Println("  exit - Salir del programa")
	fmt.Println()

	// Bucle principal de la interfaz de línea de comandos
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		command := args[0]

		switch command {
		case "create":
			if len(args) < 3 {
				fmt.Println("Uso: create <colección> <json_data>")
				continue
			}
			collection := args[1]
			jsonData := strings.Join(args[2:], " ")

			// Parsear los datos JSON
			var data map[string]any
			if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
				fmt.Printf("Error al parsear JSON: %v\n", err)
				continue
			}

			// Crear el documento
			doc, err := database.CreateDocument(collection, data)
			if err != nil {
				fmt.Printf("Error al crear documento: %v\n", err)
				continue
			}

			// La sincronización se maneja automáticamente en database.CreateDocument

			fmt.Printf("Documento creado con ID: %s\n", doc.ID)

		case "get":
			if len(args) != 2 {
				fmt.Println("Uso: get <id>")
				continue
			}
			id := args[1]

			// Obtener el documento
			doc, err := database.GetDocument(id)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// Mostrar el documento
			fmt.Println(doc.String())

		case "query":
			if len(args) < 3 {
				fmt.Println("Uso: query <colección> <json_query>")
				continue
			}
			collection := args[1]
			jsonQuery := strings.Join(args[2:], " ")

			// Parsear la consulta JSON
			var query map[string]any
			if err := json.Unmarshal([]byte(jsonQuery), &query); err != nil {
				fmt.Printf("Error al parsear JSON: %v\n", err)
				continue
			}

			// Buscar documentos
			docs, err := database.QueryDocuments(collection, query)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// Mostrar resultados
			fmt.Printf("Encontrados %d documentos:\n", len(docs))
			for i, doc := range docs {
				fmt.Printf("\n[%d] %s\n", i+1, doc.String())
			}

		case "update":
			if len(args) < 3 {
				fmt.Println("Uso: update <id> <json_data>")
				continue
			}
			id := args[1]
			jsonData := strings.Join(args[2:], " ")

			// Parsear los datos JSON
			var data map[string]any
			if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
				fmt.Printf("Error al parsear JSON: %v\n", err)
				continue
			}

			// Actualizar el documento
			doc, err := database.UpdateDocument(id, data)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// La sincronización se maneja automáticamente en database.UpdateDocument

			fmt.Println("Documento actualizado:")
			fmt.Println(doc.String())

		case "delete":
			if len(args) != 2 {
				fmt.Println("Uso: delete <id>")
				continue
			}
			id := args[1]

			// Eliminar el documento
			if err := database.DeleteDocument(id); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// La sincronización se maneja automáticamente en database.DeleteDocument

			fmt.Printf("Documento con ID %s eliminado\n", id)

		case "list":
			if len(args) != 2 {
				fmt.Println("Uso: list <colección>")
				continue
			}
			collection := args[1]

			// Obtener todos los documentos de la colección
			docs, err := database.GetAllDocuments(collection)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// Mostrar resultados
			fmt.Printf("Encontrados %d documentos en la colección '%s':\n", len(docs), collection)
			for i, doc := range docs {
				fmt.Printf("\n[%d] ID: %s\n", i+1, doc.ID)
				dataJSON, _ := json.MarshalIndent(doc.Data, "    ", "  ")
				fmt.Printf("    Datos: %s\n", dataJSON)
			}

		case "exit":
			fmt.Println("Saliendo...")
			return

		case "backup":
			// Crear una copia de seguridad
			backupName, err := database.CreateBackup()
			if err != nil {
				fmt.Printf("Error al crear copia de seguridad: %v\n", err)
				continue
			}

			fmt.Printf("Copia de seguridad creada: %s\n", backupName)

		case "restore":
			if len(args) != 2 {
				fmt.Println("Uso: restore <nombre_backup>")
				continue
			}
			backupName := args[1]

			// Restaurar desde la copia de seguridad
			if err := database.RestoreFromBackup(backupName); err != nil {
				fmt.Printf("Error al restaurar desde copia de seguridad: %v\n", err)
				continue
			}

			fmt.Printf("Base de datos restaurada desde: %s\n", backupName)

		case "list_backups":
			// Listar todas las copias de seguridad
			backups, err := database.ListBackups()
			if err != nil {
				fmt.Printf("Error al listar copias de seguridad: %v\n", err)
				continue
			}

			if len(backups) == 0 {
				fmt.Println("No hay copias de seguridad disponibles")
			} else {
				fmt.Println("Copias de seguridad disponibles:")
				for i, backup := range backups {
					fmt.Printf("  [%d] %s\n", i+1, backup)
				}
			}

		case "sync":
			// Sincronizar todos los documentos
			fmt.Println("Sincronizando todos los documentos...")
			if err := database.SyncAllDocuments(); err != nil {
				fmt.Printf("Error al sincronizar documentos: %v\n", err)
				continue
			}
			fmt.Println("Sincronización completada")

		default:
			fmt.Println("Comando desconocido. Comandos disponibles:")
			fmt.Println("  create <colección> <json_data> - Crear un nuevo documento")
			fmt.Println("  get <id> - Obtener un documento por ID")
			fmt.Println("  query <colección> <json_query> - Buscar documentos")
			fmt.Println("  update <id> <json_data> - Actualizar un documento")
			fmt.Println("  delete <id> - Eliminar un documento")
			fmt.Println("  list <colección> - Listar todos los documentos de una colección")
			fmt.Println("  backup - Crear una copia de seguridad de la base de datos")
			fmt.Println("  restore <nombre_backup> - Restaurar la base de datos desde una copia de seguridad")
			fmt.Println("  list_backups - Listar todas las copias de seguridad disponibles")
			fmt.Println("  sync - Sincronizar todos los documentos con la red")
			fmt.Println("  exit - Salir del programa")
		}
	}
}

func waitForSignal() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Señal recibida, cerrando...")
}

type DummyAuthManager struct{}

func NewAuthManager(dataDir string) (*DummyAuthManager, error) {
	return &DummyAuthManager{}, nil
}
