package main

import (
	"time"

	"gorm.io/gorm"
)

// Table representa una mesa del restaurante
type Table struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Number    int       `gorm:"not null" json:"number"`
	Capacity  int       `gorm:"not null" json:"capacity"`
	Available bool      `gorm:"default:true" json:"available"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MenuItem representa un ítem del menú
type MenuItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	Price       float64   `gorm:"not null" json:"price"`
	Category    string    `json:"category"`
	Available   bool      `gorm:"default:true" json:"available"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Order representa un pedido completo
type Order struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	TableID   uint        `gorm:"not null" json:"table_id"`
	Table     Table       `json:"table"`
	Total     float64     `gorm:"default:0" json:"total"`
	Status    string      `gorm:"default:'active'" json:"status"` // active, completed, cancelled
	Items     []OrderItem `gorm:"foreignKey:OrderID" json:"items"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// OrderItem representa un ítem en un pedido
type OrderItem struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	OrderID    uint      `gorm:"not null" json:"order_id"`
	MenuItemID uint      `gorm:"not null" json:"menu_item_id"`
	MenuItem   MenuItem  `json:"menu_item"`
	Quantity   int       `gorm:"default:1" json:"quantity"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SetupDatabase configura la base de datos y migra los modelos
func SetupDatabase(db *gorm.DB) error {
	return db.AutoMigrate(&Table{}, &MenuItem{}, &Order{}, &OrderItem{})
}

// SeedData inserta datos de prueba si la base de datos está vacía
func SeedData(db *gorm.DB) error {
	// Crear mesas si no existen
	var tableCount int64
	db.Model(&Table{}).Count(&tableCount)
	if tableCount == 0 {
		tables := []Table{
			{Number: 1, Capacity: 4, Available: true},
			{Number: 2, Capacity: 2, Available: true},
			{Number: 3, Capacity: 6, Available: true},
			{Number: 4, Capacity: 4, Available: true},
		}
		if err := db.Create(&tables).Error; err != nil {
			return err
		}
	}

	// Crear items de menú si no existen
	var menuItemCount int64
	db.Model(&MenuItem{}).Count(&menuItemCount)
	if menuItemCount == 0 {
		menuItems := []MenuItem{
			{Name: "Hamburguesa", Description: "Con queso y tocino", Price: 8.99, Category: "Platos principales", Available: true},
			{Name: "Pizza", Description: "Pizza mediana de pepperoni", Price: 10.99, Category: "Platos principales", Available: true},
			{Name: "Ensalada César", Description: "Con pollo y aderezo", Price: 6.99, Category: "Entradas", Available: true},
			{Name: "Refresco", Description: "Vaso de 500ml", Price: 1.99, Category: "Bebidas", Available: true},
			{Name: "Agua mineral", Description: "Botella de 500ml", Price: 1.50, Category: "Bebidas", Available: true},
			{Name: "Pastel de chocolate", Description: "Porción individual", Price: 3.99, Category: "Postres", Available: true},
		}
		if err := db.Create(&menuItems).Error; err != nil {
			return err
		}
	}

	return nil
}
