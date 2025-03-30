package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

// Agregar al inicio del archivo, después del package
// Cerca del inicio del archivo, donde defines las variables globales
var sessionStore = session.New(session.Config{
	Expiration:   24 * time.Hour,
	CookieName:   "order_session",
	CookieSecure: false, // En producción con HTTPS debería ser true
})

// OrderTemp representa una orden temporal durante su creación
type OrderTemp struct {
	TableNum int             `json:"tableNum"`
	Items    []OrderItemTemp `json:"items"`
	Total    float64         `json:"total"`
}

// OrderItemTemp representa un item en una orden temporal
type OrderItemTemp struct {
	ProductID   uint    `json:"productId"`
	ProductName string  `json:"productName"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Notes       string  `json:"notes"`
	Subtotal    float64 `json:"subtotal"`
}

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
		Status:    "in_progress",
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

// Add these missing handlers
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
	db.First(&order, item.OrderID)

	var allItems []OrderItem
	db.Where("order_id = ?", item.OrderID).Preload("Product").Find(&allItems)

	total := 0.0
	for _, i := range allItems {
		total += i.Product.Price * float64(i.Quantity)
	}

	order.Total = total
	db.Save(&order)

	c.Set("HX-Trigger", `{"showToast": "Ítem actualizado"}`)

	db.Preload("Items").Preload("Items.Product").First(&order, item.OrderID)

	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
	}, "")
}

// RemoveOrderItem elimina un item de la orden (renaming the existing function to match route declaration)
func RemoveOrderItem(c *fiber.Ctx) error {
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

func GetOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	result := db.Preload("Items").Preload("Items.Product").First(&order, id)
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
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	result := db.First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	order.Status = "completed"
	db.Save(&order)

	// Liberar la mesa asociada
	db.Model(&Table{}).Where("order_id = ?", order.ID).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	})

	// Si es una solicitud HTMX, enviar header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/orders")
		c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(id)+` completada correctamente"}`)
		return c.SendString("Redirigiendo...")
	}

	return c.Redirect("/orders")
}

// CancelOrder cancela una orden
func CancelOrder(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var order Order
	result := db.First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	order.Status = "cancelled"
	db.Save(&order)

	// Liberar la mesa asociada
	db.Model(&Table{}).Where("order_id = ?", order.ID).Updates(map[string]interface{}{
		"occupied": false,
		"order_id": nil,
	})

	// Si es una solicitud HTMX, enviar header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/orders")
		c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(id)+` cancelada"}`)
		return c.SendString("Redirigiendo...")
	}

	return c.Redirect("/orders")
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
	if err != nil || quantity <= 0 {
		quantity = 1 // Valor predeterminado
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

	// Verificar si el producto ya existe en la orden
	var existingItem OrderItem
	result := db.Where("order_id = ? AND product_id = ? AND notes = ?", orderID, productID, notes).First(&existingItem)

	if result.Error == nil {
		// El producto ya existe, aumentar cantidad
		existingItem.Quantity += quantity
		db.Save(&existingItem)
	} else {
		// Producto nuevo en la orden
		orderItem := OrderItem{
			OrderID:   uint(orderID),
			ProductID: product.ID,
			Quantity:  quantity,
			Notes:     notes,
			IsReady:   false,
		}
		db.Create(&orderItem)
	}

	// Actualizar total de la orden
	var allItems []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&allItems)

	total := 0.0
	for _, item := range allItems {
		total += item.Product.Price * float64(item.Quantity)
	}

	order.Total = total
	db.Save(&order)

	// Devolver la vista actualizada
	c.Set("HX-Trigger", `{"showToast": "Producto añadido"}`)

	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
		"Items": allItems,
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

func AddItemToTempOrder(c *fiber.Ctx) error {
	// Obtener ID de orden directamente del formulario
	orderIDStr := c.FormValue("order_id")
	if orderIDStr == "" {
		log.Printf("No se proporcionó ID de orden en el formulario")
		return c.Status(fiber.StatusBadRequest).SendString("No se especificó el ID de orden")
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		log.Printf("Error convirtiendo ID de orden: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Verificar que la orden existe
	var order Order
	if result := db.First(&order, orderID).Error; result != nil {
		log.Printf("Orden no encontrada: %v", result)
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Obtener datos del producto a añadir
	productID, err := strconv.Atoi(c.FormValue("product_id"))
	if err != nil {
		log.Printf("ID de producto inválido: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de producto inválido")
	}

	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity <= 0 {
		quantity = 1 // Valor predeterminado
	}

	notes := c.FormValue("notes")

	// Buscar información del producto
	var product Product
	if err := db.First(&product, productID).Error; err != nil {
		log.Printf("Producto no encontrado: %v", err)
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Buscar si ya existe este producto en la orden
	var existingItem OrderItem
	result := db.Where("order_id = ? AND product_id = ? AND notes = ?", orderID, productID, notes).First(&existingItem)

	if result.Error == nil {
		// Ya existe, actualizar cantidad
		existingItem.Quantity += quantity
		db.Save(&existingItem)
	} else {
		// Crear nuevo item
		newItem := OrderItem{
			OrderID:   uint(orderID),
			ProductID: product.ID,
			Quantity:  quantity,
			Notes:     notes,
			IsReady:   false,
		}
		db.Create(&newItem)
	}

	// Recalcular total
	var allItems []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&allItems)

	total := 0.0
	for _, item := range allItems {
		total += item.Product.Price * float64(item.Quantity)
	}

	// Actualizar total en la orden
	order.Total = total
	db.Save(&order)

	// Notificar éxito
	c.Set("HX-Trigger", `{"showToast": "Producto añadido"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     allItems,
		"Total":     total,
		"ItemCount": len(allItems),
		"OrderID":   orderID,
	}, "")
}

// RemoveItemFromTempOrder elimina un item de la orden
func RemoveItemFromTempOrder(c *fiber.Ctx) error {
	// Obtener el ID del ítem a eliminar
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		log.Printf("ID de ítem inválido: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	// Obtener el ID de la orden desde query params
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		log.Printf("RemoveItemFromTempOrder: No se encontró order_id. URL: %s", c.OriginalURL())
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden no especificado")
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		log.Printf("Error convirtiendo order_id '%s': %v", orderIDStr, err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Verificar que la orden exista
	var order Order
	if result := db.First(&order, orderID).Error; result != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Encontrar el item directamente por su ID
	var item OrderItem
	if err := db.First(&item, itemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Ítem no encontrado")
	}

	// Verificar que el item pertenezca a esta orden
	if item.OrderID != uint(orderID) {
		return c.Status(fiber.StatusBadRequest).SendString("El ítem no pertenece a esta orden")
	}

	// Eliminar el ítem
	if err := db.Delete(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al eliminar ítem")
	}

	// Recalcular total
	var allRemainingItems []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&allRemainingItems)

	total := 0.0
	for _, item := range allRemainingItems {
		total += item.Product.Price * float64(item.Quantity)
	}

	// Actualizar total de la orden
	db.Model(&Order{}).Where("id = ?", orderID).Update("total", total)

	// Notificar éxito
	c.Set("HX-Trigger", `{"showToast": "Producto eliminado de la orden"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     allRemainingItems,
		"Total":     total,
		"ItemCount": len(allRemainingItems),
		"OrderID":   orderID,
	}, "")
}

// UpdateTempOrderItemQuantity actualiza la cantidad de un ítem
func UpdateTempOrderItemQuantity(c *fiber.Ctx) error {
	// Obtener ID del ítem directamente
	itemID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de ítem inválido")
	}

	action := c.Params("action") // "increase" o "decrease"
	if action != "increase" && action != "decrease" {
		return c.Status(fiber.StatusBadRequest).SendString("Acción inválida")
	}

	// Obtener el ID de la orden desde query params
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		log.Printf("UpdateTempOrderItemQuantity: No se encontró order_id. URL: %s", c.OriginalURL())
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden no especificado")
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Verificar que la orden exista
	var order Order
	if result := db.First(&order, orderID).Error; result != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Encontrar el item directamente por su ID
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
		// Guardar cambio de cantidad
		if err := db.Save(&item).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar cantidad")
		}
	} else if action == "decrease" {
		if item.Quantity > 1 {
			item.Quantity--
			// Guardar cambio de cantidad
			if err := db.Save(&item).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar cantidad")
			}
		} else {
			// Si la cantidad llega a 0, eliminar el ítem
			if err := db.Delete(&item).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error al eliminar ítem")
			}
		}
	}

	// Recalcular total
	var updatedItems []OrderItem
	db.Where("order_id = ?", orderID).Preload("Product").Find(&updatedItems)

	total := 0.0
	for _, item := range updatedItems {
		total += item.Product.Price * float64(item.Quantity)
	}

	// Actualizar total de la orden
	db.Model(&Order{}).Where("id = ?", orderID).Update("total", total)

	// Notificar éxito
	c.Set("HX-Trigger", `{"showToast": "Cantidad actualizada"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     updatedItems,
		"Total":     total,
		"ItemCount": len(updatedItems),
		"OrderID":   orderID,
	}, "")
}

// Simplificar la función ClearTempOrder para usar solo query parameters
func ClearTempOrder(c *fiber.Ctx) error {
	// Obtener el ID de la orden solamente desde query params para solicitudes DELETE
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		log.Printf("ClearTempOrder: No se encontró order_id en QueryParams. URL: %s", c.OriginalURL())
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden no especificado")
	}

	log.Printf("ClearTempOrder recibido con order_id (query): %s", orderIDStr)

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		log.Printf("Error convirtiendo order_id '%s' a entero: %v", orderIDStr, err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Verificar que la orden exista
	var order Order
	if result := db.First(&order, orderID).Error; result != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Eliminar todos los ítems de la orden
	if err := db.Where("order_id = ?", orderID).Delete(&OrderItem{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al eliminar ítems")
	}

	// Actualizar total de la orden a cero
	db.Model(&Order{}).Where("id = ?", orderID).Update("total", 0)

	// Notificar limpieza
	c.Set("HX-Trigger", `{"showToast": "Orden limpiada"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     []OrderItem{},
		"Total":     0.0,
		"ItemCount": 0,
		"OrderID":   orderID,
	}, "")
}

func ConfirmTempOrder(c *fiber.Ctx) error {
	// Obtener ID de orden directamente del formulario
	orderIDStr := c.FormValue("order_id")
	if orderIDStr == "" {
		return c.Status(fiber.StatusBadRequest).SendString("No se especificó el ID de orden")
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Obtener la orden
	var order Order
	if result := db.Preload("Items").First(&order, orderID).Error; result != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Validar que haya productos
	if len(order.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "La orden debe tener al menos un producto",
		})
	}

	// Notas adicionales
	notes := c.FormValue("notes")
	if notes != "" {
		order.Notes = notes
	}

	// Cambiar estado de la orden a pending
	order.Status = "pending"

	// Actualizar la orden
	if err := db.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Error al confirmar la orden",
		})
	}

	// Notificar éxito y redireccionar
	c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(int(order.ID))+` confirmada correctamente"}`)
	c.Set("HX-Redirect", "/orders")
	return c.SendString("Orden confirmada correctamente. Redirigiendo...")
}

// Añade esta función de diagnóstico
func DebugSession(c *fiber.Ctx) error {
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error obteniendo sesión: " + err.Error(),
		})
	}

	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.JSON(fiber.Map{
			"status":    "No hay orden temporal en la sesión",
			"sessionID": sess.ID(),
		})
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.JSON(fiber.Map{
			"error":     "Error deserializando orden: " + err.Error(),
			"sessionID": sess.ID(),
			"raw":       orderTempJSON,
		})
	}

	return c.JSON(fiber.Map{
		"status":    "Orden temporal encontrada",
		"sessionID": sess.ID(),
		"orderTemp": orderTemp,
	})
}

// GetTempOrderSummary obtiene el resumen de la orden temporal para el modal de confirmación
func GetTempOrderSummary(c *fiber.Ctx) error {
	// Obtener ID de orden desde query param
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		return c.Status(fiber.StatusBadRequest).SendString("No se especificó el ID de orden")
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID de orden inválido")
	}

	// Obtener la orden con sus items
	var order Order
	if err := db.Preload("Items").Preload("Items.Product").First(&order, orderID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Renderizar el resumen
	return c.Render("partials/confirm_order_summary", fiber.Map{
		"Items":    order.Items,
		"Total":    order.Total,
		"TableNum": order.TableNum,
	}, "")
}
