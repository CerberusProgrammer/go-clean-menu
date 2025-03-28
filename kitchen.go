package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// KitchenHandler muestra la vista de cocina
func KitchenHandler(c *fiber.Ctx) error {
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

	return c.Render("kitchen", fiber.Map{
		"Title":      "Vista de Cocina",
		"ActivePage": "kitchen",
		"Orders":     pendingOrders,
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

	// Add the empty string as the third parameter to render without layout
	return c.Render("partials/kitchen_orders", fiber.Map{
		"Orders": pendingOrders,
	}, "") // This empty string disables the layout
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

	response := "Estado actualizado"
	if allItemsReady {
		response = "Todos los items están listos"
		c.Set("HX-Trigger", `{"showToast": "Todos los productos están listos"}`)
	}

	return c.SendString(response)
}

// KitchenCompleteOrder marca una orden como completada desde la cocina
func KitchenCompleteOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	if result := db.First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	order.Status = "completed"
	db.Save(&order)

	// Liberar la mesa asociada
	db.Model(&Table{}).Where("order_id = ?", order.ID).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	})

	// Obtener órdenes pendientes actualizadas para actualizar la vista
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

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

	var readyCount, totalCount int64

	// Contar total de ítems
	db.Model(&OrderItem{}).Where("order_id = ?", id).Count(&totalCount)

	// Contar ítems listos
	db.Model(&OrderItem{}).Where("order_id = ? AND is_ready = ?", id, true).Count(&readyCount)

	var completionPercentage float64
	if totalCount > 0 {
		completionPercentage = float64(readyCount) / float64(totalCount) * 100.0
	}

	return c.JSON(fiber.Map{
		"order_id":   id,
		"ready":      readyCount,
		"total":      totalCount,
		"percentage": completionPercentage,
	})
}
