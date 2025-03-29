package main

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// KitchenHandler muestra la vista de cocina
func KitchenHandler(c *fiber.Ctx) error {
	return c.Render("kitchen", fiber.Map{
		"Title":      "Vista de Cocina",
		"ActivePage": "kitchen",
	})
}

// GetKitchenOrders actualiza la vista de la cocina (para peticiones HTMX)
func GetKitchenOrders(c *fiber.Ctx) error {
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

	return c.Render("partials/kitchen_orders", fiber.Map{
		"Orders": pendingOrders,
	}, "") // Add the empty string as the third parameter to render without layout
}

// ToggleItemStatus marca/desmarca un producto como listo
func ToggleItemStatus(c *fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de item inválido")
	}

	var item OrderItem
	if result := db.First(&item, itemID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Item no encontrado")
	}

	// Cambiar el estado
	item.IsReady = !item.IsReady
	db.Save(&item)

	// Verificar si todos los items están listos para sugerir completar la orden
	var order Order
	db.Preload("Items").First(&order, item.OrderID)

	allItemsReady := true
	for _, orderItem := range order.Items {
		if !orderItem.IsReady {
			allItemsReady = false
			break
		}
	}

	if allItemsReady {
		c.Set("HX-Trigger", `{"showToast": "Todos los productos están listos"}`)
	} else {
		c.Set("HX-Trigger", `{"showToast": "Estado actualizado"}`)
	}

	// Obtener órdenes pendientes actualizadas para actualizar la vista
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

	// Devolver la vista actualizada
	return c.Render("partials/kitchen_orders", fiber.Map{
		"Orders": pendingOrders,
	}, "")
}

// KitchenCompleteOrder marca una orden como completada desde la cocina
func KitchenCompleteOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		log.Printf("Error de conversión de ID: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	log.Printf("Completando orden #%d", id)

	var order Order
	if result := db.First(&order, id); result.Error != nil {
		log.Printf("Orden no encontrada: %v", result.Error)
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	order.Status = "completed"
	db.Save(&order)
	log.Printf("Orden #%d marcada como completada", id)

	// Liberar la mesa asociada
	db.Model(&Table{}).Where("number = ?", order.TableNum).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	})
	log.Printf("Mesa %d liberada", order.TableNum)

	// Obtener órdenes pendientes actualizadas para actualizar la vista
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

	c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(id)+` completada correctamente"}`)

	return c.Render("partials/kitchen_orders", fiber.Map{
		"Orders": pendingOrders,
	}, "")
}

// GetOrderCompletionStatus devuelve el progreso de la orden
func GetOrderCompletionStatus(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	if result := db.Preload("Items").First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Contar total de ítems
	totalItems := len(order.Items)
	if totalItems == 0 {
		return c.Render("partials/order_progress", fiber.Map{
			"Percentage": 0,
		}, "")
	}

	// Contar ítems listos
	readyItems := 0
	for _, item := range order.Items {
		if item.IsReady {
			readyItems++
		}
	}

	percentage := (readyItems * 100) / totalItems

	return c.Render("partials/order_progress", fiber.Map{
		"Percentage": percentage,
		"OrderID":    id,
	}, "")
}
