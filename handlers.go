package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Handlers estructura para los manejadores
type Handlers struct {
	DB *gorm.DB
}

// NewHandlers crea una nueva instancia de Handlers
func NewHandlers(db *gorm.DB) *Handlers {
	return &Handlers{DB: db}
}

// Home muestra la página principal
func (h *Handlers) Home(c *fiber.Ctx) error {
	return c.Render("tables", fiber.Map{
		"Title": "Mesas",
	})
}

// GetTables obtiene todas las mesas
func (h *Handlers) GetTables(c *fiber.Ctx) error {
	var tables []Table
	if err := h.DB.Find(&tables).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener las mesas",
		})
	}

	return c.Render("tables", fiber.Map{
		"Title":  "Mesas",
		"Tables": tables,
	})
}

// GetMenu obtiene todos los items del menú
func (h *Handlers) GetMenu(c *fiber.Ctx) error {
	var menuItems []MenuItem
	if err := h.DB.Find(&menuItems).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener el menú",
		})
	}

	// Agrupar por categoría
	categories := make(map[string][]MenuItem)
	for _, item := range menuItems {
		if item.Available {
			categories[item.Category] = append(categories[item.Category], item)
		}
	}

	tableID := c.Query("table_id")

	return c.Render("menu", fiber.Map{
		"Title":      "Menú",
		"Categories": categories,
		"TableID":    tableID,
	})
}

// CreateOrder crea una nueva orden
func (h *Handlers) CreateOrder(c *fiber.Ctx) error {
	tableIDStr := c.FormValue("table_id")
	tableID, err := strconv.ParseUint(tableIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de mesa inválido",
		})
	}

	// Verificar si ya existe una orden activa para esta mesa
	var existingOrder Order
	result := h.DB.Where("table_id = ? AND status = ?", tableID, "active").First(&existingOrder)
	if result.Error == nil {
		// Ya existe una orden, redirigir a esa orden
		return c.Redirect("/order/" + strconv.FormatUint(uint64(existingOrder.ID), 10))
	}

	// Crear nueva orden
	order := Order{
		TableID: uint(tableID),
		Status:  "active",
	}

	if err := h.DB.Create(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al crear la orden",
		})
	}

	return c.Redirect("/order/" + strconv.FormatUint(uint64(order.ID), 10))
}

// GetOrder obtiene una orden específica
func (h *Handlers) GetOrder(c *fiber.Ctx) error {
	orderID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de orden inválido",
		})
	}

	var order Order
	if err := h.DB.Preload("Table").Preload("Items.MenuItem").First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Orden no encontrada",
		})
	}

	var menuItems []MenuItem
	if err := h.DB.Where("available = ?", true).Find(&menuItems).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener los items del menú",
		})
	}

	// Agrupar por categoría
	categories := make(map[string][]MenuItem)
	for _, item := range menuItems {
		categories[item.Category] = append(categories[item.Category], item)
	}

	// Recalcular el total
	var total float64
	for _, item := range order.Items {
		total += item.MenuItem.Price * float64(item.Quantity)
	}

	// Actualizar el total en la base de datos
	if order.Total != total {
		order.Total = total
		h.DB.Save(&order)
	}

	return c.Render("order", fiber.Map{
		"Title":      "Orden #" + strconv.FormatUint(uint64(order.ID), 10),
		"Order":      order,
		"Categories": categories,
	})
}

// AddToOrder añade un item a la orden
func (h *Handlers) AddToOrder(c *fiber.Ctx) error {
	orderID, err := strconv.ParseUint(c.FormValue("order_id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de orden inválido",
		})
	}

	menuItemID, err := strconv.ParseUint(c.FormValue("menu_item_id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de item inválido",
		})
	}

	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity <= 0 {
		quantity = 1
	}

	notes := c.FormValue("notes")

	// Verificar si ya existe este item en la orden
	var existingItem OrderItem
	result := h.DB.Where("order_id = ? AND menu_item_id = ?", orderID, menuItemID).First(&existingItem)

	if result.Error == nil {
		// Ya existe, actualizar cantidad
		existingItem.Quantity += quantity
		existingItem.Notes = notes
		if err := h.DB.Save(&existingItem).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error al actualizar el item",
			})
		}
	} else {
		// No existe, crear nuevo
		orderItem := OrderItem{
			OrderID:    uint(orderID),
			MenuItemID: uint(menuItemID),
			Quantity:   quantity,
			Notes:      notes,
		}

		if err := h.DB.Create(&orderItem).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error al añadir item a la orden",
			})
		}
	}

	return c.Redirect("/order/" + strconv.FormatUint(orderID, 10))
}

// UpdateOrderItem actualiza la cantidad de un item de la orden
func (h *Handlers) UpdateOrderItem(c *fiber.Ctx) error {
	orderItemID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de item inválido",
		})
	}

	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cantidad inválida",
		})
	}

	var orderItem OrderItem
	if err := h.DB.First(&orderItem, orderItemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Item no encontrado",
		})
	}

	if quantity == 0 {
		// Eliminar el item
		if err := h.DB.Delete(&orderItem).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error al eliminar el item",
			})
		}
	} else {
		// Actualizar cantidad
		orderItem.Quantity = quantity
		if err := h.DB.Save(&orderItem).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Error al actualizar el item",
			})
		}
	}

	return c.Redirect("/order/" + strconv.FormatUint(uint64(orderItem.OrderID), 10))
}

// CompleteOrder marca una orden como completada
func (h *Handlers) CompleteOrder(c *fiber.Ctx) error {
	orderID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de orden inválido",
		})
	}

	var order Order
	if err := h.DB.First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Orden no encontrada",
		})
	}

	order.Status = "completed"
	if err := h.DB.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al completar la orden",
		})
	}

	return c.Redirect("/tables")
}

// CancelOrder cancela una orden
func (h *Handlers) CancelOrder(c *fiber.Ctx) error {
	orderID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID de orden inválido",
		})
	}

	var order Order
	if err := h.DB.First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Orden no encontrada",
		})
	}

	order.Status = "cancelled"
	if err := h.DB.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al cancelar la orden",
		})
	}

	return c.Redirect("/tables")
}
