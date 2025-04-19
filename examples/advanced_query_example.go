package main

import (
	"fmt"
	"log"
	"time"

	"dbp2p/pkg/db"
)

func main() {
	// Crear una instancia de la base de datos con persistencia
	database, err := db.NewDatabaseWithPersistence("./data_example")
	if err != nil {
		log.Fatalf("Error al crear base de datos: %v", err)
	}

	// Crear gestor de memoria
	memoryManager := db.NewMemoryManager(database, db.MemoryManagerConfig{
		CheckInterval:     time.Second * 10,
		CleanupThreshold:  0.7,
		MaxDocuments:      1000,
		EnableCompression: true,
	})
	memoryManager.Start()
	defer memoryManager.Stop()

	// Crear algunos documentos de ejemplo
	fmt.Println("Creando documentos de ejemplo...")
	createSampleData(database)

	// Realizar consultas avanzadas
	fmt.Println("\nRealizando consultas avanzadas...")

	// Consulta 1: Usuarios activos ordenados por edad
	fmt.Println("\n1. Usuarios activos ordenados por edad:")
	query1 := db.NewQuery("users").
		Where("active", db.OperatorEQ, true).
		Sort("age", db.SortAscending)

	results1, err := query1.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 1: %v", err)
	} else {
		printResults(results1)
	}

	// Consulta 2: Usuarios con edad entre 25 y 40
	fmt.Println("\n2. Usuarios con edad entre 25 y 40:")
	query2 := db.NewQuery("users").
		And(
			db.QueryCondition{Field: "age", Operator: db.OperatorGTE, Value: 25},
			db.QueryCondition{Field: "age", Operator: db.OperatorLTE, Value: 40},
		)

	results2, err := query2.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 2: %v", err)
	} else {
		printResults(results2)
	}

	// Consulta 3: Usuarios con rol "admin" o "manager"
	fmt.Println("\n3. Usuarios con rol 'admin' o 'manager':")
	query3 := db.NewQuery("users").
		Where("role", db.OperatorIN, []interface{}{"admin", "manager"})

	results3, err := query3.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 3: %v", err)
	} else {
		printResults(results3)
	}

	// Consulta 4: Usuarios con correo que termina en "@example.com"
	fmt.Println("\n4. Usuarios con correo que termina en '@example.com':")
	query4 := db.NewQuery("users").
		Where("email", db.OperatorENDSWITH, "@example.com")

	results4, err := query4.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 4: %v", err)
	} else {
		printResults(results4)
	}

	// Consulta 5: Usuarios inactivos o con edad mayor a 50
	fmt.Println("\n5. Usuarios inactivos o con edad mayor a 50:")
	query5 := db.NewQuery("users").
		Or(
			db.QueryCondition{Field: "active", Operator: db.OperatorEQ, Value: false},
			db.QueryCondition{Field: "age", Operator: db.OperatorGT, Value: 50},
		)

	results5, err := query5.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 5: %v", err)
	} else {
		printResults(results5)
	}

	// Consulta 6: Usuarios con dirección en Madrid
	fmt.Println("\n6. Usuarios con dirección en Madrid:")
	query6 := db.NewQuery("users").
		Where("address.city", db.OperatorEQ, "Madrid")

	results6, err := query6.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 6: %v", err)
	} else {
		printResults(results6)
	}

	// Consulta 7: Usuarios con al menos 2 habilidades
	fmt.Println("\n7. Usuarios con al menos 2 habilidades:")
	// Nota: Esta consulta requeriría una implementación más compleja
	// para contar elementos de un array. Por ahora, lo simulamos.
	var results7 []*db.Document
	allUsers, _ := database.GetAllDocuments("users")
	for _, user := range allUsers {
		if skills, ok := user.Data["skills"].([]interface{}); ok && len(skills) >= 2 {
			results7 = append(results7, user)
		}
	}
	printResults(results7)

	// Consulta 8: Usuarios creados en el último mes
	fmt.Println("\n8. Usuarios creados en el último mes:")
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	query8 := db.NewQuery("users").
		Where("created_at", db.OperatorGTE, oneMonthAgo)

	results8, err := query8.Execute(database)
	if err != nil {
		log.Printf("Error en consulta 8: %v", err)
	} else {
		printResults(results8)
	}

	// Mostrar estadísticas de memoria
	fmt.Println("\nEstadísticas de memoria:")
	stats := memoryManager.GetMemoryStats()
	fmt.Printf("Documentos en memoria: %d\n", stats.DocumentCount)
	fmt.Printf("Documentos comprimidos: %d\n", stats.CompressedDocs)
	fmt.Printf("Bytes ahorrados por compresión: %d\n", stats.CompressedBytes)
	fmt.Printf("Número de limpiezas: %d\n", stats.CleanupCount)

	fmt.Println("\nEjemplo completado.")
}

// createSampleData crea datos de ejemplo
func createSampleData(database *db.Database) {
	// Crear usuarios
	users := []map[string]interface{}{
		{
			"name":     "Juan Pérez",
			"email":    "juan@example.com",
			"age":      30,
			"active":   true,
			"role":     "admin",
			"created_at": time.Now().AddDate(0, -2, 0),
			"address": map[string]interface{}{
				"city":    "Madrid",
				"country": "España",
			},
			"skills": []interface{}{"Go", "Python", "JavaScript"},
		},
		{
			"name":     "María García",
			"email":    "maria@example.com",
			"age":      25,
			"active":   true,
			"role":     "user",
			"created_at": time.Now().AddDate(0, 0, -5),
			"address": map[string]interface{}{
				"city":    "Barcelona",
				"country": "España",
			},
			"skills": []interface{}{"Java", "C++"},
		},
		{
			"name":     "Pedro López",
			"email":    "pedro@example.com",
			"age":      40,
			"active":   false,
			"role":     "manager",
			"created_at": time.Now().AddDate(0, -3, 0),
			"address": map[string]interface{}{
				"city":    "Madrid",
				"country": "España",
			},
			"skills": []interface{}{"SQL", "PHP"},
		},
		{
			"name":     "Ana Martínez",
			"email":    "ana@gmail.com",
			"age":      35,
			"active":   true,
			"role":     "user",
			"created_at": time.Now().AddDate(0, 0, -15),
			"address": map[string]interface{}{
				"city":    "Valencia",
				"country": "España",
			},
			"skills": []interface{}{"HTML", "CSS"},
		},
		{
			"name":     "Carlos Rodríguez",
			"email":    "carlos@example.com",
			"age":      55,
			"active":   true,
			"role":     "admin",
			"created_at": time.Now().AddDate(0, -1, 0),
			"address": map[string]interface{}{
				"city":    "Sevilla",
				"country": "España",
			},
			"skills": []interface{}{"Ruby", "Python", "Go"},
		},
	}

	// Guardar usuarios
	for _, userData := range users {
		_, err := database.CreateDocument("users", userData)
		if err != nil {
			log.Printf("Error al crear usuario: %v", err)
		}
	}
}

// printResults imprime los resultados de una consulta
func printResults(results []*db.Document) {
	fmt.Printf("Total de resultados: %d\n", len(results))
	for i, doc := range results {
		fmt.Printf("%d. ID: %s, Nombre: %s", i+1, doc.ID, doc.Data["name"])
		if age, ok := doc.Data["age"].(float64); ok {
			fmt.Printf(", Edad: %.0f", age)
		}
		if email, ok := doc.Data["email"].(string); ok {
			fmt.Printf(", Email: %s", email)
		}
		if active, ok := doc.Data["active"].(bool); ok {
			fmt.Printf(", Activo: %v", active)
		}
		fmt.Println()
	}
}
