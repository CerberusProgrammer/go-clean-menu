package main

import (
	"time"

	"gorm.io/gorm"
)

// Producto representa un ítem del menú
type Product struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Category    string    `json:"category"`
	IsAvailable bool      `json:"is_available" gorm:"default:true"`
	ImagePath   string    `json:"image_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Category representa una categoría de productos
type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"unique"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Orden contiene los detalles de la orden
type Order struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	TableNum  int            `json:"table_num"`
	Status    string         `json:"status"` // "pending", "completed", "cancelled", NEW: -> "in_progress"
	Total     float64        `json:"total"`
	Items     []OrderItem    `json:"items" gorm:"foreignKey:OrderID"`
	Notes     string         `json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// OrderItem representa un producto en una orden
type OrderItem struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	OrderID         uint       `json:"order_id"`
	ProductID       uint       `json:"product_id"`
	Product         Product    `json:"product" gorm:"foreignKey:ProductID"`
	Quantity        int        `json:"quantity"`
	Notes           string     `json:"notes"`
	IsReady         bool       `json:"is_ready" gorm:"default:false"`
	CookingStarted  *time.Time `json:"cooking_started"`
	CookingFinished *time.Time `json:"cooking_finished"`
	CookingTime     int        `json:"cooking_time_seconds" gorm:"default:0"` // Tiempo en segundos
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Settings almacena la configuración de la aplicación
type Settings struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	RestaurantName string    `json:"restaurant_name" gorm:"default:'Resto'"`
	Address        string    `json:"address"`
	Phone          string    `json:"phone"`
	Email          string    `json:"email"`
	LogoPath       string    `json:"logo_path"`
	DefaultPrinter string    `json:"default_printer"`
	AutoPrint      bool      `json:"auto_print" gorm:"default:true"`
	TableCount     int       `json:"table_count" gorm:"default:12"`
	DarkMode       bool      `json:"dark_mode" gorm:"default:false"`
	AutoRefresh    bool      `json:"auto_refresh" gorm:"default:true"`
	Language       string    `json:"language" gorm:"default:'es'"`
	TaxRate        float64   `json:"tax_rate" gorm:"default:0.16"`
	CurrencySymbol string    `json:"currency_symbol" gorm:"default:'$'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Table representa una mesa en el restaurante
type Table struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Number    int       `json:"number"`
	Capacity  int       `json:"capacity"`
	Occupied  bool      `json:"occupied" gorm:"default:false"`
	OrderID   *uint     `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Backup representa un respaldo de la base de datos
type Backup struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	FileName  string    `json:"file_name"`
	FilePath  string    `json:"file_path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// User representa un usuario del sistema
type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"unique"`
	Password  string         `json:"-"` // No exponer la contraseña en JSON
	FullName  string         `json:"full_name"`
	Email     string         `json:"email"`
	Role      string         `json:"role"` // "admin", "waiter", "cook"
	Active    bool           `json:"active" gorm:"default:true"`
	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}
