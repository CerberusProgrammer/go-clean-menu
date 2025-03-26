package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Cargar variables de entorno
	godotenv.Load() // Ignoramos error si no existe archivo

	// Configurar conexión a la base de datos con valores predeterminados seguros
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "go_clean_menu")

	// Construir DSN
	dsn := "host=" + dbHost +
		" user=" + dbUser +
		" password=" + dbPassword +
		" dbname=" + dbName +
		" port=" + dbPort +
		" sslmode=disable TimeZone=UTC"

	// Intentar conectar a la base de datos con reintentos
	var db *gorm.DB
	var err error

	log.Println("Intentando conectar a la base de datos...")
	maxRetries := 5
	for i := range maxRetries {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("Intento %d: error al conectar a la BD: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Fatalf("No se pudo conectar a la base de datos después de %d intentos: %v", maxRetries, err)
	}
	log.Println("Conexión a la base de datos establecida")

	// Migrar modelos
	if err := SetupDatabase(db); err != nil {
		log.Fatalf("Error al migrar la base de datos: %v", err)
	}

	// Cargar datos de prueba
	if err := SeedData(db); err != nil {
		log.Fatalf("Error al cargar datos iniciales: %v", err)
	}

	engine := html.New("./templates", ".html")
	engine.Reload(true) // Enable reloading in development
	engine.Debug(true)  // Enable debug output

	// Agregar funciones personalizadas para las plantillas
	engine.AddFunc("add", func(a, b int) int {
		return a + b
	})

	engine.AddFunc("sub", func(a, b int) int {
		if a-b < 0 {
			return 0
		}
		return a - b
	})

	// Configurar la aplicación Fiber
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	// Servir archivos estáticos
	app.Static("/static", "./static")

	// Inicializar handlers
	h := NewHandlers(db)

	// Rutas
	app.Get("/", h.Home)
	app.Get("/tables", h.GetTables)
	app.Get("/menu", h.GetMenu)
	app.Post("/orders", h.CreateOrder)
	app.Get("/order/:id", h.GetOrder)
	app.Post("/order/add-item", h.AddToOrder)
	app.Post("/order/item/:id", h.UpdateOrderItem)
	app.Post("/order/complete/:id", h.CompleteOrder)
	app.Post("/order/cancel/:id", h.CancelOrder)
	app.Get("/debug", h.DebugTest)
	app.Get("/system-debug", h.Debug)
	app.Get("/direct-tables", h.DirectTableTest)
	// Add these routes after your existing routes
	app.Get("/ultra-debug", h.UltraDebug)
	app.Get("/test-template", h.TestTemplate)
	// Iniciar servidor
	port := getEnv("PORT", "3000")
	log.Printf("Servidor iniciado en http://localhost:%s", port)
	log.Fatal(app.Listen(":" + port))
}

// getEnv obtiene una variable de entorno o devuelve un valor predeterminado
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
