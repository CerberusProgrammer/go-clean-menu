package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func initDatabase() {
	var err error

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error al conectar a la base de datos: %v", err)
	}

	// Auto-migrar modelos
	err = db.AutoMigrate(&Product{}, &Category{}, &Order{}, &OrderItem{}, &Settings{}, &Table{}, &Backup{}, &User{})
	if err != nil {
		log.Fatalf("Error en auto-migración: %v", err)
	}

	// Insertar datos de ejemplo si no existen productos
	var count int64
	db.Model(&Product{}).Count(&count)
	if count == 0 {
		seedProducts()
	}

	// Inicializar configuración si no existe
	var settings Settings
	result := db.First(&settings)
	if result.Error != nil {
		// Crear configuración predeterminada
		settings = Settings{
			RestaurantName: "Resto",
			Address:        "123 Calle Principal",
			Phone:          "(555) 123-4567",
			Email:          "contacto@resto.com",
			DefaultPrinter: "thermal1",
			AutoPrint:      true,
			TableCount:     12,
			DarkMode:       false,
			AutoRefresh:    true,
			Language:       "es",
			TaxRate:        0.16,
			CurrencySymbol: "$",
		}
		db.Create(&settings)
	}

	// Inicializar mesas si no existen
	var tableCount int64
	db.Model(&Table{}).Count(&tableCount)
	if tableCount == 0 {
		// Crear mesas predeterminadas
		for i := 1; i <= settings.TableCount; i++ {
			table := Table{
				Number:   i,
				Capacity: 4,
				Occupied: false,
			}
			db.Create(&table)
		}
		log.Printf("Se inicializaron %d mesas", settings.TableCount)
	}
}

// seedProducts inserta productos de ejemplo en la base de datos
func seedProducts() {
	products := []Product{
		{Name: "Hamburguesa Clásica", Description: "Carne de res, lechuga, tomate y mayonesa", Price: 8.99, Category: "Hamburguesas"},
		{Name: "Hamburguesa con Queso", Description: "Carne de res, queso cheddar, lechuga, tomate", Price: 9.99, Category: "Hamburguesas"},
		{Name: "Pizza Margarita", Description: "Salsa de tomate, queso mozzarella y albahaca", Price: 10.99, Category: "Pizzas"},
		{Name: "Pizza Pepperoni", Description: "Salsa de tomate, queso mozzarella y pepperoni", Price: 12.99, Category: "Pizzas"},
		{Name: "Ensalada César", Description: "Lechuga romana, crutones, parmesano y aderezo césar", Price: 7.99, Category: "Ensaladas"},
		{Name: "Papas Fritas", Description: "Papas fritas crujientes con sal", Price: 3.99, Category: "Acompañamientos"},
		{Name: "Refresco", Description: "Variedad de refrescos", Price: 2.50, Category: "Bebidas"},
		{Name: "Agua Mineral", Description: "Agua mineral con o sin gas", Price: 1.99, Category: "Bebidas"},
	}

	for _, product := range products {
		db.Create(&product)
	}
}

func main() {
	// Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("No se pudo cargar el archivo .env, usando variables de entorno")
	}

	// Inicializar base de datos
	initDatabase()

	// Configurar engine de plantillas
	engine := html.New("./templates", ".html")

	engine.AddFunc("formatDate", func(t time.Time) string {
		return t.Format("02/01/2006")
	})

	engine.AddFunc("formatTime", func(t time.Time) string {
		return t.Format("15:04")
	})

	engine.AddFunc("add", func(a, b int) int {
		return a + b
	})

	engine.AddFunc("sub", func(a, b int) int {
		return a - b
	})

	engine.AddFunc("mul", func(a, b int) int {
		return a * b
	})
	engine.AddFunc("multiply", func(a, b int) int {
		return a * b
	})

	engine.AddFunc("div", func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	})

	// Crear aplicación Fiber
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
	})

	engine.AddFunc("mul", func(a, b int) int {
		return a * b
	})

	engine.AddFunc("div", func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("CurrentTime", time.Now().Format("02/01/2006 15:04"))
		return c.Next()
	})

	// Rutas del Dashboard
	app.Get("/", DashboardHandler)

	// Rutas de Productos
	app.Get("/products", GetProducts)
	app.Get("/products/category/:category", GetProductsByCategory)
	app.Get("/products/form", GetProductForm)
	app.Post("/products", CreateProduct)
	app.Put("/products/:id", UpdateProduct)
	app.Delete("/products/:id", DeleteProduct)
	app.Get("/products/:id/edit", GetProductEditForm)

	// Rutas de Categorías
	app.Get("/forms/category", GetCategoryForm)
	app.Post("/categories", CreateCategory)

	// Rutas de Órdenes
	app.Get("/orders", OrdersHandler)
	app.Post("/orders", CreateOrder)
	app.Get("/order/:id", GetOrder)
	app.Post("/order/:id/add-item", AddItemToOrder)
	app.Delete("/order/:id/item/:itemId", RemoveItemFromOrder)
	app.Post("/order/:id/complete", CompleteOrder)
	app.Delete("/order/:id", CancelOrder)
	app.Post("/order/:id/print", PrintOrder)
	app.Post("/order/:id/email", EmailOrder)
	app.Post("/order/:id/duplicate", DuplicateOrder)

	// Rutas de Cocina
	app.Get("/kitchen", KitchenHandler)
	app.Get("/kitchen/orders", GetKitchenOrders)
	app.Put("/kitchen/items/:id/toggle", ToggleItemStatus)
	app.Get("/kitchen/order/:id/status", GetOrderCompletionStatus)

	// Rutas de Menu
	app.Get("/menu", MenuHandler)

	// Rutas de Historial
	app.Get("/history", HistoryHandler)
	app.Get("/history/today", GetTodayHistory)
	app.Get("/history/week", GetWeekHistory)
	app.Get("/history/month", GetMonthHistory)
	app.Get("/history/custom", GetCustomHistory)
	app.Get("/history/report/:id", GenerateOrderReport)

	// Rutas de Configuración
	app.Get("/settings", SettingsHandler)
	app.Put("/settings/restaurant", UpdateRestaurantSettings)
	app.Put("/settings/printer", UpdatePrinterSettings)
	app.Put("/settings/tables", UpdateTableSettings)
	app.Put("/settings/app", UpdateAppSettings)
	app.Post("/backup", CreateBackup)
	app.Get("/backup/list", GetBackupList)
	app.Get("/backup/:id/download", DownloadBackup)

	// Rutas de Mesas
	app.Get("/tables", TablesHandler)
	app.Post("/tables", CreateTable)
	app.Delete("/tables/:id", DeleteTable)
	app.Post("/tables/reset", ResetTables)

	// Iniciar servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Servidor iniciando en puerto %s", port)
	log.Fatal(app.Listen(":" + port))
}
