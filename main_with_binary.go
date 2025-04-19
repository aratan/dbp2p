package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"dbp2p/pkg/api"
	"dbp2p/pkg/auth"
	"dbp2p/pkg/binary"
	"dbp2p/pkg/config"
	"dbp2p/pkg/db"
	"dbp2p/pkg/p2p"
	"dbp2p/pkg/ws"
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
	if flag.Lookup("api").Value.String() == "true" || flag.Lookup("api").Value.String() == "false" {
		cfg.API.Enabled = *enableAPI
	}
	if flag.Lookup("ws").Value.String() == "true" || flag.Lookup("ws").Value.String() == "false" {
		cfg.WebSocket.Enabled = *enableWS
	}

	// Crear directorio de datos si no existe
	dataDir := "./data"
	if cfg.General.DataDir != "" {
		dataDir = cfg.General.DataDir
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Error al crear directorio de datos: %v", err)
	}

	// Inicializar la base de datos
	database, err := db.NewDatabase(filepath.Join(dataDir, "db"))
	if err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}

	// Inicializar el gestor de autenticación
	authManager, err := auth.NewAuthManager(database, cfg.Auth.JWT.Secret, time.Duration(cfg.Auth.JWT.Expiration)*time.Second)
	if err != nil {
		log.Fatalf("Error al inicializar el gestor de autenticación: %v", err)
	}

	// Crear usuario administrador por defecto si no existe
	if err := authManager.EnsureAdminUser(cfg.Auth.DefaultAdmin.Username, cfg.Auth.DefaultAdmin.Password); err != nil {
		log.Printf("Error al crear usuario administrador: %v", err)
	}

	// Inicializar el nodo P2P
	node, err := p2p.NewNode(cfg.Network.LibP2P.ListenAddresses, cfg.Network.MDNS.Enabled, cfg.Network.DHT.Enabled)
	if err != nil {
		log.Fatalf("Error al inicializar el nodo P2P: %v", err)
	}

	// Iniciar el nodo P2P
	if err := node.Start(); err != nil {
		log.Fatalf("Error al iniciar el nodo P2P: %v", err)
	}

	// Conectar a los peers de bootstrap
	if len(cfg.Network.LibP2P.BootstrapPeers) > 0 {
		go func() {
			if err := node.ConnectToBootstrapPeers(cfg.Network.LibP2P.BootstrapPeers); err != nil {
				log.Printf("Error al conectar a peers de bootstrap: %v", err)
			}
		}()
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
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					// Crear copia de seguridad
					backupName, err := database.CreateBackup()
					if err != nil {
						log.Printf("Error al crear copia de seguridad automática: %v", err)
						continue
					}
					log.Printf("Copia de seguridad automática creada: %s", backupName)

					// Limitar el número de copias de seguridad
					if cfg.Database.Backup.MaxBackups > 0 {
						// Obtener lista de copias de seguridad
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

	// Inicializar el gestor de binarios
	binaryManager, err := binary.NewBinaryManager(
		database,
		filepath.Join(dataDir, "binaries"),
		binary.WithCompression(true),
	)
	if err != nil {
		log.Fatalf("Error al inicializar el gestor de binarios: %v", err)
	}

	// Manejar modo CLI o esperar señales de terminación
	if len(os.Args) == 1 || (len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-")) {
		// Modo CLI
		runCLI(database, dbSync, binaryManager)
	} else {
		// Esperar señales de terminación
		log.Println("Servidores iniciados. Presiona Ctrl+C para salir.")
		waitForSignal()
	}
}

func runCLI(database *db.Database, _ *db.DBSync, binaryManager *binary.BinaryManager) {
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
	fmt.Println("  sync - Sincronizar todos los documentos con la red")
	fmt.Println("  upload_binary <colección> <ruta_archivo> [metadatos_json] - Subir un archivo binario")
	fmt.Println("  download_binary <id> <ruta_destino> - Descargar un archivo binario")
	fmt.Println("  list_binaries [colección] - Listar archivos binarios")
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

		case "upload_binary":
			if len(args) < 3 {
				fmt.Println("Uso: upload_binary <colección> <ruta_archivo> [metadatos_json]")
				continue
			}
			collection := args[1]
			filePath := args[2]
			
			// Metadatos opcionales
			var metadata map[string]string
			if len(args) > 3 {
				metadataJSON := strings.Join(args[3:], " ")
				if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
					fmt.Printf("Error al parsear metadatos JSON: %v\n", err)
					continue
				}
			} else {
				metadata = make(map[string]string)
			}
			
			// Abrir el archivo
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Printf("Error al abrir archivo: %v\n", err)
				continue
			}
			defer file.Close()
			
			// Obtener información del archivo
			fileInfo, err := file.Stat()
			if err != nil {
				fmt.Printf("Error al obtener información del archivo: %v\n", err)
				continue
			}
			
			// Determinar tipo MIME basado en la extensión
			mimeType := getMimeType(filePath)
			
			// Configurar opciones
			options := []binary.StoreOption{
				binary.WithCollection(collection),
				binary.WithMetadata(metadata),
			}
			
			// Almacenar archivo
			fmt.Printf("Subiendo archivo %s (%s, %d bytes)...\n", filepath.Base(filePath), mimeType, fileInfo.Size())
			fileMetadata, err := binaryManager.StoreFile(file, filepath.Base(filePath), mimeType, options...)
			if err != nil {
				fmt.Printf("Error al almacenar archivo: %v\n", err)
				continue
			}
			
			fmt.Printf("Archivo subido exitosamente con ID: %s\n", fileMetadata.ID)

		case "download_binary":
			if len(args) != 3 {
				fmt.Println("Uso: download_binary <id> <ruta_destino>")
				continue
			}
			fileID := args[1]
			destPath := args[2]
			
			// Recuperar archivo
			fileReader, metadata, err := binaryManager.GetFile(fileID)
			if err != nil {
				fmt.Printf("Error al recuperar archivo: %v\n", err)
				continue
			}
			defer fileReader.Close()
			
			// Crear archivo de destino
			destFile, err := os.Create(destPath)
			if err != nil {
				fmt.Printf("Error al crear archivo de destino: %v\n", err)
				continue
			}
			defer destFile.Close()
			
			// Copiar contenido
			bytesWritten, err := io.Copy(destFile, fileReader)
			if err != nil {
				fmt.Printf("Error al escribir archivo: %v\n", err)
				continue
			}
			
			fmt.Printf("Archivo descargado exitosamente: %s (%d bytes)\n", metadata.Filename, bytesWritten)

		case "list_binaries":
			// Colección opcional
			var collection string
			if len(args) > 1 {
				collection = args[1]
			}
			
			// Listar archivos
			files, err := binaryManager.ListFiles(collection)
			if err != nil {
				fmt.Printf("Error al listar archivos: %v\n", err)
				continue
			}
			
			if len(files) == 0 {
				if collection != "" {
					fmt.Printf("No hay archivos en la colección '%s'\n", collection)
				} else {
					fmt.Println("No hay archivos binarios")
				}
			} else {
				if collection != "" {
					fmt.Printf("Archivos en la colección '%s':\n", collection)
				} else {
					fmt.Println("Archivos binarios:")
				}
				
				for i, file := range files {
					fmt.Printf("[%d] ID: %s\n", i+1, file.ID)
					fmt.Printf("    Nombre: %s\n", file.Filename)
					fmt.Printf("    Tipo: %s\n", file.MimeType)
					fmt.Printf("    Tamaño: %d bytes\n", file.Size)
					fmt.Printf("    Colección: %s\n", file.Collection)
					fmt.Printf("    Creado: %s\n", file.CreatedAt.Format(time.RFC3339))
					fmt.Println()
				}
			}

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
			fmt.Println("  upload_binary <colección> <ruta_archivo> [metadatos_json] - Subir un archivo binario")
			fmt.Println("  download_binary <id> <ruta_destino> - Descargar un archivo binario")
			fmt.Println("  list_binaries [colección] - Listar archivos binarios")
			fmt.Println("  exit - Salir del programa")
		}
	}
}

// getMimeType determina el tipo MIME basado en la extensión del archivo
func getMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

func waitForSignal() {
	// Crear canal para señales
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Esperar señal
	<-sigChan
	fmt.Println("\nSeñal de terminación recibida. Cerrando...")

	// Aquí se podrían realizar tareas de limpieza antes de salir
	// Por ejemplo, cerrar conexiones, guardar estado, etc.

	// Salir con código 0
	os.Exit(0)
}
