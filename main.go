package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// initDatabase inicializa la conexión a la base de datos
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
	err = db.AutoMigrate(&Product{}, &Order{}, &OrderItem{})
	if err != nil {
		log.Fatalf("Error en auto-migración: %v", err)
	}

	// Insertar datos de ejemplo si no existen productos
	var count int64
	db.Model(&Product{}).Count(&count)
	if count == 0 {
		seedProducts()
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

	// Añadir función multiply para usar en templates
	engine.AddFunc("multiply", func(a float64, b int) float64 {
		return a * float64(b)
	})

	// Crear aplicación Fiber
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())

	// Rutas estáticas
	app.Static("/static", "./static")

	// Rutas
	app.Get("/", HomeHandler)
	app.Get("/products", GetProducts)
	app.Get("/products/category/:category", GetProductsByCategory)
	app.Post("/orders", CreateOrder)
	app.Get("/order/:id", GetOrder)
	app.Post("/order/:id/add-item", AddItemToOrder)
	app.Delete("/order/:id/item/:itemId", RemoveItemFromOrder)
	app.Post("/order/:id/complete", CompleteOrder)

	// Iniciar servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Servidor iniciando en puerto %s", port)
	log.Fatal(app.Listen(":" + port))
}
