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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Orden contiene los detalles de la orden
type Order struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	TableNum  int            `json:"table_num"`
	Status    string         `json:"status"` // "pending", "completed", "cancelled"
	Total     float64        `json:"total"`
	Items     []OrderItem    `json:"items" gorm:"foreignKey:OrderID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// OrderItem representa un producto en una orden
type OrderItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	OrderID   uint      `json:"order_id"`
	ProductID uint      `json:"product_id"`
	Product   Product   `json:"product" gorm:"foreignKey:ProductID"`
	Quantity  int       `json:"quantity"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
