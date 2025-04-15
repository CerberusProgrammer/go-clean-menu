package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// CreateOrder crea una nueva orden para una mesa
func CreateOrder(c *fiber.Ctx) error {
	tableNum, err := strconv.Atoi(c.FormValue("table_num"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Número de mesa inválido")
	}

	// Verificar que la mesa exista y no esté ocupada
	var table Table
	if result := db.Where("number = ?", tableNum).First(&table); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Mesa no encontrada")
	}

	if table.Occupied {
		return c.Status(fiber.StatusBadRequest).SendString("La mesa ya está ocupada")
	}

	// Crear la orden directamente con estado "in_progress"
	order := Order{
		TableNum:  tableNum,
		Status:    "pending",
		Total:     0,
		Notes:     c.FormValue("notes"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(&order).Error; err != nil {
		log.Printf("Error al crear la orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear la orden")
	}

	// Marcar la mesa como ocupada y vincular la orden
	table.Occupied = true
	table.OrderID = &order.ID
	db.Save(&table)

	// Redireccionar a la página de edición de la orden
	c.Set("HX-Redirect", fmt.Sprintf("/order/%d", order.ID))
	return c.SendString("Orden creada")
}

// OrdersHandler renamed to GetOrders for consistency
func GetOrders(c *fiber.Ctx) error {
	var orders []Order
	db.Where("status IN (?)", []string{"pending", "in_progress"}).
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&orders)

	// Obtener mesas disponibles para el modal de nueva orden
	var availableTables []Table
	db.Where("occupied = ?", false).Order("number").Find(&availableTables)

	return c.Render("orders", fiber.Map{
		"Title":           "Órdenes Activas",
		"ActivePage":      "orders",
		"Orders":          orders,
		"AvailableTables": availableTables,
	})
}

// UpdateOrderItem actualiza un item de la orden
func UpdateOrderItem(c *fiber.Ctx) error {
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	var item OrderItem
	if err := db.First(&item, itemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Ítem no encontrado")
	}

	// Get the form values
	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity < 1 {
		return c.Status(fiber.StatusBadRequest).SendString("Cantidad inválida")
	}

	// Update the item
	item.Quantity = quantity
	item.Notes = c.FormValue("notes")
	if err := db.Save(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar el ítem")
	}

	// Recalcular total de la orden
	var order Order
	if err := db.First(&order, item.OrderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	var allItems []OrderItem
	db.Where("order_id = ?", item.OrderID).Preload("Product").Find(&allItems)

	total := 0.0
	for _, i := range allItems {
		total += i.Product.Price * float64(i.Quantity)
	}

	order.Total = total
	db.Save(&order)

	c.Set("HX-Trigger", `{"showToast": "Ítem actualizado"}`)

	// Cargar la orden completa con sus items para la vista actualizada
	// Agregamos ORDER BY id para mantener un orden consistente
	db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Items.Product").First(&order, item.OrderID)

	// Return the updated order items with the OrderID explicitly included
	return c.Render("partials/order_items", fiber.Map{
		"Order":   order,
		"OrderID": order.ID,
	}, "")
}

// RemoveOrderItem elimina un item de la orden
func RemoveOrderItem(c *fiber.Ctx) error {
	orderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	itemID, err := strconv.Atoi(c.Params("itemId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	// Buscar el item para obtener información antes de eliminarlo
	var orderItem OrderItem
	if err := db.Preload("Product").First(&orderItem, itemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Ítem no encontrado")
	}

	// Verificar que pertenezca a esta orden
	if orderItem.OrderID != uint(orderID) {
		return c.Status(fiber.StatusBadRequest).SendString("Este ítem no pertenece a la orden especificada")
	}

	// Actualizar total de la orden
	var order Order
	db.First(&order, orderID)
	order.Total -= orderItem.Product.Price * float64(orderItem.Quantity)
	if order.Total < 0 {
		order.Total = 0 // Evitar totales negativos
	}
	db.Save(&order)

	// Eliminar el item
	db.Delete(&orderItem)

	// Cargar la orden actualizada con sus items, manteniendo el orden por ID
	db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Items.Product").First(&order, orderID)

	c.Set("HX-Trigger", `{"showToast": "Producto eliminado de la orden"}`)
	return c.Render("partials/order_items", fiber.Map{
		"Order":   order,
		"OrderID": order.ID,
	}, "")
}

func GetOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	var order Order
	result := db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Items.Product").First(&order, id)

	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Obtener productos disponibles para añadir
	var products []Product
	db.Where("is_available = ?", true).Order("category, name").Find(&products)

	// Agrupar productos por categoría
	productsByCategory := make(map[string][]Product)
	for _, product := range products {
		productsByCategory[product.Category] = append(productsByCategory[product.Category], product)
	}

	// Obtener categorías ordenadas
	var categories []string
	db.Model(&Product{}).Where("is_available = ?", true).Distinct().Order("category").Pluck("category", &categories)

	// Recalcular total por si acaso
	total := 0.0
	for _, item := range order.Items {
		total += item.Product.Price * float64(item.Quantity)
	}

	if total != order.Total {
		order.Total = total
		db.Save(&order)
	}

	return c.Render("order", fiber.Map{
		"Title":              "Orden #" + strconv.Itoa(id),
		"ActivePage":         "orders",
		"Order":              order,
		"ProductsByCategory": productsByCategory,
		"Categories":         categories,
		"AllProducts":        products,
		"Items":              order.Items,
		"OrderID":            order.ID,
		"TableNum":           order.TableNum,
		"Total":              order.Total,
		"ItemCount":          len(order.Items),
	})
}

// CompleteOrder marca una orden como completada
func CompleteOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		log.Printf("Error de conversión de ID: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	if result := db.First(&order, id); result.Error != nil {
		log.Printf("Orden no encontrada: %v", result.Error)
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Verificar que la orden esté en proceso
	if order.Status != "in_progress" {
		log.Printf("La orden #%d no está en proceso", id)
		return c.Status(fiber.StatusBadRequest).SendString("Solo se pueden completar órdenes en proceso")
	}

	// Refuerzo: asegurar que todos los ítems tengan CookingFinished y CookingTime
	var items []OrderItem
	db.Where("order_id = ?", order.ID).Find(&items)
	now := time.Now()
	allReady := true
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
		if !item.IsReady {
			allReady = false
		}
	}
	// Si todos los ítems están listos, marcar CookingCompletedAt
	if allReady && order.CookingCompletedAt == nil {
		order.CookingCompletedAt = &now
	}
	// Marcar la orden como completada
	log.Printf("Marcando orden #%d como completada", id)
	order.Status = "completed"
	order.UpdatedAt = now
	if order.CompletedAt == nil {
		order.CompletedAt = &now
	}
	if order.DeliveredAt == nil {
		order.DeliveredAt = &now
	}
	if err := db.Save(&order).Error; err != nil {
		log.Printf("Error al actualizar estado de orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al completar la orden")
	}

	// Liberar la mesa asociada
	if err := db.Model(&Table{}).Where("number = ?", order.TableNum).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	}).Error; err != nil {
		log.Printf("Error al liberar mesa: %v", err)
	} else {
		log.Printf("Mesa %d liberada", order.TableNum)
	}

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

	c.Set("HX-Trigger", `{"showToast": "Orden completada correctamente"}`)
	c.Set("HX-Redirect", "/orders")
	return c.SendString("Orden completada correctamente")
}

// CancelOrder cancela una orden
func CancelOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		log.Printf("Error de conversión de ID: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	if result := db.First(&order, id); result.Error != nil {
		log.Printf("Orden no encontrada: %v", result.Error)
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// No permitir cancelar órdenes completadas
	if order.Status == "completed" {
		log.Printf("La orden #%d ya está completada y no se puede cancelar", id)
		return c.Status(fiber.StatusBadRequest).SendString("No se pueden cancelar órdenes completadas")
	}

	// Marcar la orden como cancelada
	log.Printf("Cancelando orden #%d", id)
	order.Status = "cancelled"
	order.UpdatedAt = time.Now()
	if err := db.Save(&order).Error; err != nil {
		log.Printf("Error al cancelar orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al cancelar la orden")
	}

	// Liberar la mesa asociada
	if err := db.Model(&Table{}).Where("number = ?", order.TableNum).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	}).Error; err != nil {
		log.Printf("Error al liberar mesa: %v", err)
	} else {
		log.Printf("Mesa %d liberada", order.TableNum)
	}

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

	c.Set("HX-Trigger", `{"showToast": "Orden cancelada"}`)
	c.Set("HX-Redirect", "/orders")
	return c.SendString("Orden cancelada correctamente")
}

// AddItemToOrder añade un producto a la orden
func AddItemToOrder(c *fiber.Ctx) error {
	orderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	productID, err := strconv.Atoi(c.FormValue("product_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de producto inválido")
	}

	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity < 1 {
		quantity = 1 // Default a 1 si hay un error
	}

	notes := c.FormValue("notes")

	// Verificar que existan orden y producto
	var order Order
	if result := db.First(&order, orderID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	var product Product
	if result := db.First(&product, productID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Buscar ítem existente, pero sin usar log cuando no se encuentra
	var existingItem OrderItem
	var count int64
	db.Model(&OrderItem{}).Where("order_id = ? AND product_id = ?", orderID, productID).Count(&count)

	if count > 0 {
		// El producto ya existe, actualizar cantidad y notas
		db.Where("order_id = ? AND product_id = ?", orderID, productID).First(&existingItem)
		existingItem.Quantity += quantity
		existingItem.Notes = notes
		if err := db.Save(&existingItem).Error; err != nil {
			log.Printf("Error al actualizar ítem existente: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar el ítem")
		}
		log.Printf("Actualizado producto #%d en orden #%d, nueva cantidad: %d", productID, orderID, existingItem.Quantity)
	} else {
		// Crear un nuevo item
		log.Printf("Agregando producto #%d a la orden #%d, cantidad: %d", productID, orderID, quantity)
		newItem := OrderItem{
			OrderID:   uint(orderID),
			ProductID: uint(productID),
			Quantity:  quantity,
			Notes:     notes,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(&newItem).Error; err != nil {
			log.Printf("Error al crear nuevo ítem: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error al añadir el producto")
		}
	}

	// Actualizar total de la orden
	var items []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&items)

	total := 0.0
	for _, item := range items {
		total += item.Product.Price * float64(item.Quantity)
	}

	order.Total = total
	if err := db.Save(&order).Error; err != nil {
		log.Printf("Error al actualizar total de orden: %v", err)
	}

	// Devolver la vista actualizada
	db.Preload("Items").Preload("Items.Product").First(&order, orderID)

	c.Set("HX-Trigger", `{"showToast": "Producto añadido a la orden"}`)
	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
	}, "")
}

// RemoveItemFromOrder elimina un item de la orden
func RemoveItemFromOrder(c *fiber.Ctx) error {
	orderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	itemID, err := strconv.Atoi(c.Params("itemId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de item inválido")
	}

	// Buscar el item para obtener información antes de eliminarlo
	var orderItem OrderItem
	if result := db.Preload("Product").First(&orderItem, itemID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Item no encontrado")
	}

	// Verificar que pertenezca a esta orden
	if orderItem.OrderID != uint(orderID) {
		return c.Status(fiber.StatusBadRequest).SendString("El item no pertenece a esta orden")
	}

	// Actualizar total de la orden
	var order Order
	db.First(&order, orderID)
	order.Total -= orderItem.Product.Price * float64(orderItem.Quantity)
	if order.Total < 0 {
		order.Total = 0 // Evitar totales negativos
	}
	db.Save(&order)

	// Eliminar el item
	db.Delete(&orderItem)

	// Cargar la orden actualizada con sus items
	db.Preload("Items").Preload("Items.Product").First(&order, orderID)

	c.Set("HX-Trigger", `{"showToast": "Producto eliminado de la orden"}`)
	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
	}, "")
}

// UpdateOrderItemQuantity actualiza la cantidad de un ítem
func UpdateOrderItemQuantity(c *fiber.Ctx) error {
	orderID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	itemID, err := strconv.Atoi(c.Params("itemId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	action := c.Params("action") // "increase" o "decrease"
	if action != "increase" && action != "decrease" {
		return c.Status(fiber.StatusBadRequest).SendString("Acción inválida")
	}

	// Encontrar el item
	var item OrderItem
	if err := db.First(&item, itemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Ítem no encontrado")
	}

	// Verificar que el item pertenezca a esta orden
	if item.OrderID != uint(orderID) {
		return c.Status(fiber.StatusBadRequest).SendString("El ítem no pertenece a esta orden")
	}

	// Actualizar la cantidad según la acción
	if action == "increase" {
		item.Quantity++
		db.Save(&item)
	} else if action == "decrease" {
		if item.Quantity > 1 {
			item.Quantity--
			db.Save(&item)
		} else {
			// Si la cantidad llega a 0, eliminar el ítem
			db.Delete(&item)
		}
	}

	// Recalcular total
	var order Order
	db.First(&order, orderID)

	var allItems []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&allItems)

	total := 0.0
	for _, item := range allItems {
		total += item.Product.Price * float64(item.Quantity)
	}

	order.Total = total
	db.Save(&order)

	// Notificar éxito
	c.Set("HX-Trigger", `{"showToast": "Cantidad actualizada"}`)

	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
		"Items": allItems,
	}, "")
}

// OrdersHandler muestra todas las órdenes activas
func OrdersHandler(c *fiber.Ctx) error {
	var orders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").         // Añadir esto para cargar los ítems
		Preload("Items.Product"). // Añadir esto para cargar los productos relacionados
		Find(&orders)

	// Obtener mesas disponibles para el modal de nueva orden
	var availableTables []Table
	db.Where("occupied = ?", false).Order("number").Find(&availableTables)

	return c.Render("orders", fiber.Map{
		"Title":           "Órdenes Activas",
		"ActivePage":      "orders",
		"Orders":          orders,
		"AvailableTables": availableTables,
	})
}

// PrintOrder genera un ticket de orden
func PrintOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	result := db.Preload("Items").Preload("Items.Product").First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Simulación de impresión - en producción conectarías con una API de impresora
	c.Set("HX-Trigger", `{"showToast": "Imprimiendo ticket para orden #`+strconv.Itoa(id)+`"}`)

	// Código para generar ticket en formato real...
	// sendToDefaultPrinter("Orden #" + strconv.Itoa(id))

	return c.SendString("Imprimiendo ticket...")
}

// EmailOrder envía la orden por correo
func EmailOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	email := c.FormValue("email")
	if email == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Email requerido")
	}

	var order Order
	result := db.Preload("Items").Preload("Items.Product").First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Simulación de envío de correo - en producción conectarías con un servicio SMTP
	c.Set("HX-Trigger", `{"showToast": "Recibo enviado a `+email+`"}`)

	// Código para enviar email real...
	// sendEmail(email, "Su recibo de Resto", generateReceipt(order))

	return c.SendString("Email enviado")
}

// DuplicateOrder crea una copia de una orden existente
func DuplicateOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	// Obtener la orden original
	var originalOrder Order
	result := db.Preload("Items").Preload("Items.Product").First(&originalOrder, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Crear nueva orden
	newOrder := Order{
		TableNum:  originalOrder.TableNum,
		Status:    "pending",
		Total:     0,
		Notes:     originalOrder.Notes,
		CreatedAt: time.Now(),
	}
	db.Create(&newOrder)

	// Duplicar los items
	for _, item := range originalOrder.Items {
		newItem := OrderItem{
			OrderID:   newOrder.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Notes:     item.Notes,
		}
		db.Create(&newItem)

		// Actualizar total
		newOrder.Total += item.Product.Price * float64(item.Quantity)
	}
	db.Save(&newOrder)

	// Si es una solicitud HTMX, enviar header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/order/"+strconv.Itoa(int(newOrder.ID)))
		c.Set("HX-Trigger", `{"showToast": "Orden duplicada correctamente"}`)
		return c.SendString("Redirigiendo...")
	}

	return c.Redirect("/order/" + strconv.Itoa(int(newOrder.ID)))
}

func UpdateOrderNotes(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	result := db.First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	order.Notes = c.FormValue("notes")
	db.Save(&order)

	c.Set("HX-Trigger", `{"showToast": "Notas actualizadas"}`)
	return c.SendString("Notas actualizadas")
}

// ProcessOrder marca una orden como en proceso (enviada a cocina)
func ProcessOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	if result := db.First(&order, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Verificar que la orden esté en estado pendiente
	if order.Status != "pending" {
		return c.Status(fiber.StatusBadRequest).SendString("Solo órdenes pendientes pueden ser procesadas")
	}

	// Obtener el timestamp actual
	now := time.Now()

	// Cambiar el estado a "in_progress"
	order.Status = "in_progress"
	order.UpdatedAt = now
	if order.SentToKitchenAt == nil {
		order.SentToKitchenAt = &now
	}
	db.Save(&order)

	// Registrar tiempo de inicio para todos los items de la orden
	var items []OrderItem
	db.Where("order_id = ?", id).Find(&items)

	for _, item := range items {
		if item.CookingStarted == nil {
			item.CookingStarted = &now
			db.Save(&item)
		}
	}

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

	c.Set("HX-Trigger", `{"showToast": "Orden enviada a cocina correctamente"}`)
	c.Set("HX-Redirect", "/orders")
	return c.SendString("Orden enviada a cocina")
}
