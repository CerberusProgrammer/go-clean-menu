package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// WebSocket clients and broadcaster
var orderClients = make(map[*websocket.Conn]bool)
var kitchenClients = make(map[*websocket.Conn]bool)
var wsBroadcast = make(chan WSMessage)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

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

func wsOrders(c *websocket.Conn) {
	orderClients[c] = true
	defer func() {
		delete(orderClients, c)
		c.Close()
	}()
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

func wsKitchen(c *websocket.Conn) {
	kitchenClients[c] = true
	defer func() {
		delete(kitchenClients, c)
		c.Close()
	}()
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

func wsBroadcaster() {
	for {
		msg := <-wsBroadcast
		data, _ := json.Marshal(msg)
		switch msg.Type {
		case "order_update":
			for c := range orderClients {
				c.WriteMessage(websocket.TextMessage, data)
			}
		case "kitchen_update":
			for c := range kitchenClients {
				c.WriteMessage(websocket.TextMessage, data)
			}
		}
	}
}

func main() {
	// Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("No se pudo cargar el archivo .env, usando variables de entorno")
	}

	// Inicializar base de datos
	initDatabase()
	initStaticData()
	// Configurar engine de plantillas
	engine := html.New("./templates", ".html")

	engine.AddFuncMap(template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("02/01/2006")
		},
		"formatTime": func(t time.Time) string {
			return t.Format("15:04")
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call, needs to be pairs")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; len(values) > i; i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"add": func(a, b int) int {
			return a + b
		},
		"calculateProgress": func(items []OrderItem) int {
			if len(items) == 0 {
				return 0
			}

			readyCount := 0
			for _, item := range items {
				if item.IsReady {
					readyCount++
				}
			}

			return (readyCount * 100) / len(items)
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"subtract": func(a, b int) int { // Añadimos la función "subtract"
			return a - b
		},
		"mul": func(a float64, b int) float64 {
			return a * float64(b)
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"multiply": func(a float64, b int) float64 {
			return a * float64(b)
		},
		"truncate": func(s string, length int) string {
			if len(s) <= length {
				return s
			}
			return s[:length] + "..."
		},
		"formatTimeJS": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format(time.RFC3339)
		},
		// En la sección donde defines tus funciones de plantilla
		"formatDuration": func(seconds float64) string {
			minutes := int(seconds) / 60
			secs := int(seconds) % 60

			if minutes > 0 {
				return fmt.Sprintf("%dm %ds", minutes, secs)
			}
			return fmt.Sprintf("%ds", secs)
		},
		// Añadir esta función en la sección donde defines las funciones de plantilla
		"float64": func(i int) float64 {
			return float64(i)
		},
		// Añadir la función de porcentaje para calcular progreso de órdenes
		"percentage": func(part, total int) int {
			if total == 0 {
				return 0
			}
			return (part * 100) / total
		},
	})

	// Cargar todas las plantillas, incluidas las parciales
	if err := engine.Load(); err != nil {
		log.Fatalf("Error cargando plantillas: %v", err)
	}

	// Crear aplicación Fiber
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
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

	// En la sección de rutas

	// Rutas de Categorías
	app.Get("/forms/category", GetCategoryForm)
	app.Post("/categories", CreateCategory)
	app.Get("/categories/list", GetCategoryList)

	// Rutas para órdenes
	app.Get("/orders", GetOrders)
	app.Post("/orders/create", CreateOrder)
	app.Get("/order/:id", GetOrder)
	app.Post("/order/:id/complete", CompleteOrder)
	app.Post("/order/:id/process", ProcessOrder) // Nueva ruta para procesar la orden
	app.Post("/order/:id/cancel", CancelOrder)   // Ruta para cancelar orden
	app.Post("/order/:id/item", AddItemToOrder)
	app.Put("/order/item/:id", UpdateOrderItem)
	app.Delete("/order/:id/item/:itemId", RemoveItemFromOrder)
	app.Put("/order/:id/notes", UpdateOrderNotes)

	// Rutas de Cocina
	app.Get("/kitchen", KitchenHandler)
	app.Get("/kitchen/orders", GetKitchenOrders)
	app.Put("/kitchen/items/:id/toggle", ToggleItemStatus)
	app.Post("/kitchen/order/:id/complete", KitchenCompleteOrder)
	app.Get("/kitchen/order/:id/status", GetOrderCompletionStatus)
	app.Get("/kitchen/stats", GetKitchenStats)
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

	// Rutas WebSocket
	app.Get("/ws/orders", websocket.New(wsOrders))
	app.Get("/ws/kitchen", websocket.New(wsKitchen))

	// Iniciar broadcaster
	go wsBroadcaster()

	// Iniciar servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	log.Printf("Servidor iniciando en puerto %s", port)
	log.Fatal(app.Listen(":" + port))
}

// Añade esta función justo después de seedProducts()

// initStaticData garantiza que haya algunas categorías disponibles inicialmente
func initStaticData() {
	// Verificar si hay categorías existentes
	var categoryCount int64
	db.Model(&Product{}).Distinct().Select("category").Where("category != ''").Count(&categoryCount)

	if categoryCount < 1 {
		// Insertar categorías básicas si no hay ninguna
		categories := []string{"Hamburguesas", "Pizzas", "Ensaladas", "Acompañamientos", "Bebidas", "Postres"}
		for _, cat := range categories {
			product := Product{
				Name:        "Categoría: " + cat,
				Description: "Categoría inicial",
				Category:    cat,
				Price:       0.01,
				IsAvailable: false,
			}
			db.Create(&product)
		}
		log.Printf("Se crearon %d categorías iniciales", len(categories))
	}
}
