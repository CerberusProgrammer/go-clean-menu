package main

import (
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// KitchenHandler muestra la vista de cocina
func KitchenHandler(c *fiber.Ctx) error {
	var orders []Order
	db.Where("status IN (?)", []string{"pending", "in_progress"}).
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&orders)

	return c.Render("kitchen", fiber.Map{
		"Title":      "Cocina",
		"ActivePage": "kitchen",
		"Orders":     orders,
	})
}

// GetKitchenOrders devuelve la lista actualizada de órdenes para la cocina
func GetKitchenOrders(c *fiber.Ctx) error {
	var orders []Order
	db.Where("status IN (?)", []string{"pending", "in_progress"}).
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&orders)

	return c.Render("partials/kitchen_orders", fiber.Map{
		"Orders": orders,
	}, "")
}

// ToggleItemStatus marca/desmarca un producto como listo
func ToggleItemStatus(c *fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	var item OrderItem
	if result := db.First(&item, itemID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Ítem no encontrado")
	}

	// Registrar métricas de tiempo
	now := time.Now()

	if !item.IsReady {
		// El producto está pasando de "no listo" a "listo"
		item.IsReady = true

		// Si no tenía tiempo de inicio, registrarlo
		if item.CookingStarted == nil {
			if item.CookingFinished != nil {
				item.CookingStarted = item.CookingFinished
			} else {
				item.CookingStarted = &now
			}
		}

		// Registrar tiempo de finalización
		item.CookingFinished = &now

		// Calcular tiempo de cocción total en segundos
		if item.CookingStarted != nil {
			cookingTime := int(now.Sub(*item.CookingStarted).Seconds())
			if cookingTime < 0 {
				cookingTime = 0
			}
			item.CookingTime = cookingTime
		}

		if item.DeliveredAt == nil {
			item.DeliveredAt = &now
		}
	} else {
		// El producto está pasando de "listo" a "no listo"
		item.IsReady = false
		item.CookingFinished = nil // Remover tiempo de finalización
		item.CookingTime = 0
		item.DeliveredAt = nil
	}

	db.Save(&item)

	// Actualizar el estado de la orden a "in_progress" si estaba en "pending"
	var order Order
	db.First(&order, item.OrderID)
	if order.Status == "pending" {
		order.Status = "in_progress"
		db.Save(&order)
	}

	// Verificar si todos los items están listos para sugerir completar la orden
	var itemsCount int64
	var readyItemsCount int64

	db.Model(&OrderItem{}).Where("order_id = ?", item.OrderID).Count(&itemsCount)
	db.Model(&OrderItem{}).Where("order_id = ? AND is_ready = ?", item.OrderID, true).Count(&readyItemsCount)

	message := "Estado actualizado"
	if itemsCount > 0 && itemsCount == readyItemsCount {
		message = "¡Todos los productos están listos! Puede completar la orden."
	}

	// Si todos los ítems están listos, marcar CookingCompletedAt en la orden
	var allItems []OrderItem
	db.Where("order_id = ?", item.OrderID).Find(&allItems)
	allReady := true
	for _, it := range allItems {
		if !it.IsReady {
			allReady = false
			break
		}
	}
	if allReady {
		if order.CookingCompletedAt == nil {
			order.CookingCompletedAt = &now
			db.Save(&order)
		}
	}

	c.Set("HX-Trigger", `{"showToast": "`+message+`"}`)

	// --- Notificación WebSocket ---
	wsBroadcast <- WSMessage{
		Type:    "order_update",
		Payload: order,
	}
	wsBroadcast <- WSMessage{
		Type:    "kitchen_update",
		Payload: order,
	}
	// -----------------------------

	// Obtener órdenes pendientes actualizadas para actualizar la vista
	var pendingOrders []Order
	db.Where("status IN (?)", []string{"pending", "in_progress"}).
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

	now := time.Now()
	// Refuerzo: asegurar que todos los ítems tengan CookingFinished, CookingTime y DeliveredAt
	var items []OrderItem
	db.Where("order_id = ?", order.ID).Find(&items)
	for _, item := range items {
		if !item.IsReady {
			item.IsReady = true
			if item.CookingStarted == nil {
				item.CookingStarted = &now
			}
			item.CookingFinished = &now
			cookingTime := int(now.Sub(*item.CookingStarted).Seconds())
			if cookingTime < 0 {
				cookingTime = 0
			}
			item.CookingTime = cookingTime
		}
		if item.DeliveredAt == nil {
			item.DeliveredAt = &now
		}
		db.Save(&item)
	}
	if order.CookingCompletedAt == nil {
		order.CookingCompletedAt = &now
	}
	order.Status = "completed"
	order.UpdatedAt = now
	if order.CompletedAt == nil {
		order.CompletedAt = &now
	}
	if order.DeliveredAt == nil {
		order.DeliveredAt = &now
	}
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
	db.Where("status IN (?)", []string{"pending", "in_progress"}).
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

// GetKitchenStats genera estadísticas sobre rendimiento de cocina
func GetKitchenStats(c *fiber.Ctx) error {
	// Obtener periodo de análisis (por defecto 30 días)
	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days <= 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// Productos más rápidos de cocinar (tiempo promedio)
	type ProductCookingTime struct {
		ProductID   uint
		ProductName string
		AvgTime     float64 // tiempo promedio en segundos
		MinTime     int     // tiempo mínimo en segundos
		MaxTime     int     // tiempo máximo en segundos
		Count       int     // cantidad de veces cocinado
	}

	var fastestProducts []ProductCookingTime
	db.Raw(`
        SELECT oi.product_id, p.name as product_name, 
               AVG(oi.cooking_time) as avg_time,
               MIN(oi.cooking_time) as min_time,
               MAX(oi.cooking_time) as max_time,
               COUNT(oi.id) as count
        FROM order_items oi
        JOIN products p ON p.id = oi.product_id
        WHERE oi.cooking_time > 0
        AND oi.cooking_finished IS NOT NULL
        AND oi.created_at >= ?
        GROUP BY oi.product_id, p.name
        ORDER BY avg_time ASC
        LIMIT 10
    `, startDate).Scan(&fastestProducts)

	// Productos más lentos
	var slowestProducts []ProductCookingTime
	db.Raw(`
        SELECT oi.product_id, p.name as product_name, 
               AVG(oi.cooking_time) as avg_time,
               MIN(oi.cooking_time) as min_time,
               MAX(oi.cooking_time) as max_time,
               COUNT(oi.id) as count
        FROM order_items oi
        JOIN products p ON p.id = oi.product_id
        WHERE oi.cooking_time > 0
        AND oi.cooking_finished IS NOT NULL
        AND oi.created_at >= ?
        GROUP BY oi.product_id, p.name
        ORDER BY avg_time DESC
        LIMIT 10
    `, startDate).Scan(&slowestProducts)

	// Tiempos promedios por categoría
	type CategoryCookingTime struct {
		Category string
		AvgTime  float64
		Count    int
	}

	var categoryTimes []CategoryCookingTime
	db.Raw(`
        SELECT p.category, AVG(oi.cooking_time) as avg_time, COUNT(oi.id) as count
        FROM order_items oi
        JOIN products p ON p.id = oi.product_id
        WHERE oi.cooking_time > 0
        AND oi.cooking_finished IS NOT NULL
        AND oi.created_at >= ?
        GROUP BY p.category
        ORDER BY avg_time ASC
    `, startDate).Scan(&categoryTimes)

	// Tiempo promedio de preparación de órdenes completas
	type OrderPrepTime struct {
		Date    string
		AvgTime float64
		Count   int
	}

	var dailyPrepTimes []OrderPrepTime
	db.Raw(`
        SELECT DATE(o.created_at) as date,
               AVG(EXTRACT(EPOCH FROM (MAX(oi.cooking_finished) - MIN(oi.cooking_started)))) as avg_time,
               COUNT(DISTINCT o.id) as count
        FROM orders o
        JOIN order_items oi ON o.id = oi.order_id
        WHERE o.status = 'completed'
        AND oi.cooking_finished IS NOT NULL
        AND oi.cooking_started IS NOT NULL
        AND o.created_at >= ?
        GROUP BY DATE(o.created_at)
        ORDER BY date DESC
        LIMIT 14
    `, startDate).Scan(&dailyPrepTimes)

	// Horas pico y tiempos promedio por hora
	type HourlyStats struct {
		Hour    int
		Count   int
		AvgTime float64
	}

	var hourlyStats []HourlyStats
	db.Raw(`
        SELECT EXTRACT(HOUR FROM o.created_at) as hour,
               COUNT(DISTINCT o.id) as count,
               AVG(oi.cooking_time) as avg_time
        FROM orders o
        JOIN order_items oi ON o.id = oi.order_id
        WHERE oi.cooking_time > 0
        AND o.created_at >= ?
        GROUP BY EXTRACT(HOUR FROM o.created_at)
        ORDER BY hour
    `, startDate).Scan(&hourlyStats)

	return c.Render("kitchen_stats", fiber.Map{
		"Title":           "Estadísticas de Cocina",
		"ActivePage":      "kitchen_stats",
		"Days":            days,
		"FastestProducts": fastestProducts,
		"SlowestProducts": slowestProducts,
		"CategoryTimes":   categoryTimes,
		"DailyPrepTimes":  dailyPrepTimes,
		"HourlyStats":     hourlyStats,
	})
}
