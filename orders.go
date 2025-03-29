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

// CreateOrder crea una nueva orden
func CreateOrder(c *fiber.Ctx) error {
	tableNum, err := strconv.Atoi(c.FormValue("table_num"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Número de mesa inválido")
	}

	// Verificar que la mesa exista y no esté ocupada
	var table Table
	result := db.Where("number = ?", tableNum).First(&table)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Mesa no encontrada")
	}

	if table.Occupied {
		// Verificar si ya hay una orden pendiente para esta mesa
		return c.Status(fiber.StatusBadRequest).SendString("Esta mesa ya está ocupada")
	}

	// Crear la orden con el estado pendiente
	order := Order{
		TableNum:  tableNum,
		Status:    "pending",
		Total:     0,
		CreatedAt: time.Now(),
	}

	result = db.Create(&order)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear la orden")
	}

	// Marcar la mesa como ocupada y asociarla a la orden
	table.Occupied = true
	table.OrderID = &order.ID
	db.Save(&table)

	// Si es una solicitud HTMX, enviar header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/order/"+strconv.Itoa(int(order.ID)))
		c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(int(order.ID))+` creada correctamente"}`)
		return c.SendString("Redirigiendo...")
	}

	return c.Redirect("/order/" + strconv.Itoa(int(order.ID)))
}

// GetOrder obtiene los detalles de una orden
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

	// Obtener productos para añadir a la orden
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

	return c.Render("order", fiber.Map{
		"Title":              "Detalle de Orden #" + strconv.Itoa(id),
		"ActivePage":         "orders",
		"Order":              order,
		"ProductsByCategory": productsByCategory,
		"Categories":         categories,
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
		quantity = 1 // Valor predeterminado si hay error
	}

	notes := c.FormValue("notes")

	// Verificar que existan orden y producto
	var order Order
	result := db.First(&order, orderID)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	var product Product
	result = db.First(&product, productID)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Verificar si el producto ya existe en la orden
	var existingItem OrderItem
	result = db.Where("order_id = ? AND product_id = ?", orderID, productID).First(&existingItem)

	if result.Error == nil {
		// El producto ya existe, aumentar cantidad
		existingItem.Quantity += quantity
		// Actualizar notas
		if notes != "" {
			existingItem.Notes = notes
		}
		db.Save(&existingItem)

		// Actualizar total de la orden
		order.Total += product.Price * float64(quantity)
	} else {
		// Producto nuevo en la orden
		orderItem := OrderItem{
			OrderID:   uint(orderID),
			ProductID: uint(productID),
			Quantity:  quantity,
			Notes:     notes,
			IsReady:   false,
			Product:   product,
		}
		db.Create(&orderItem)

		// Actualizar total de la orden
		order.Total += product.Price * float64(quantity)
	}

	db.Save(&order)

	// Cargar la orden actualizada con sus items
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
	result := db.Preload("Product").First(&orderItem, itemID)
	if result.Error != nil {
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

// GetNewOrderPage muestra la página para crear una nueva orden
func GetNewOrderPage(c *fiber.Ctx) error {
	tableNum, err := strconv.Atoi(c.Params("tableNum"))
	if err != nil {
		log.Printf("Error en número de mesa: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("Número de mesa inválido")
	}

	// Verificar que la mesa exista y esté disponible
	var table Table
	result := db.Where("number = ?", tableNum).First(&table)
	if result.Error != nil {
		log.Printf("Mesa no encontrada: %v", result.Error)
		return c.Status(fiber.StatusNotFound).SendString("Mesa no encontrada")
	}

	if table.Occupied {
		log.Printf("Mesa %d ya ocupada", tableNum)
		return c.Status(fiber.StatusBadRequest).SendString("Esta mesa ya está ocupada")
	}

	// Obtener todos los productos disponibles
	var products []Product
	db.Where("is_available = ?", true).Order("category, name").Find(&products)
	log.Printf("Productos cargados: %d", len(products))

	// Agrupar productos por categoría
	productsByCategory := make(map[string][]Product)
	for _, product := range products {
		productsByCategory[product.Category] = append(productsByCategory[product.Category], product)
	}

	// Obtener categorías ordenadas
	var categories []string
	db.Model(&Product{}).Where("is_available = ?", true).Distinct().Order("category").Pluck("category", &categories)

	// Inicializar orden temporal en la sesión
	// IMPORTANTE: Primero crear una nueva sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		log.Printf("Error obteniendo sesión: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Crear una nueva orden temporal limpia
	orderTemp := OrderTemp{
		TableNum: tableNum,
		Items:    []OrderItemTemp{}, // Inicializar con slice vacío (no nil)
		Total:    0.0,
	}

	// Serializar y guardar en sesión
	orderTempJSON, err := json.Marshal(orderTemp)
	if err != nil {
		log.Printf("Error serializando orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al inicializar orden")
	}

	// Limpiar cualquier valor previo y establecer el nuevo
	sess.Delete("orderTemp")
	sess.Set("orderTemp", string(orderTempJSON))

	// Guardar la sesión inmediatamente
	if err := sess.Save(); err != nil {
		log.Printf("Error guardando sesión: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar sesión")
	}

	// Log para debugging
	log.Printf("Orden temporal creada para mesa %d, items: %d",
		tableNum, len(orderTemp.Items))

	return c.Render("order", fiber.Map{
		"Title":              "Nueva Orden - Mesa " + strconv.Itoa(tableNum),
		"ActivePage":         "orders",
		"TableNum":           tableNum,
		"AllProducts":        products,
		"ProductsByCategory": productsByCategory,
		"Categories":         categories,
		"Items":              orderTemp.Items,
		"Total":              orderTemp.Total,
		"ItemCount":          0, // Explícitamente 0
	})
}

// AddItemToTempOrder añade un producto a la orden temporal
func AddItemToTempOrder(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		log.Printf("AddItemToTempOrder - Error obteniendo sesión: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		log.Printf("AddItemToTempOrder - No se encontró orden temporal en sesión")

		// Si llega un parámetro de mesa, podemos crear una nueva orden temporal
		tableNum := 0
		if tableNumStr := c.Query("table"); tableNumStr != "" {
			tableNum, _ = strconv.Atoi(tableNumStr)

			// Crear nueva orden temporal
			orderTemp := OrderTemp{
				TableNum: tableNum,
				Items:    []OrderItemTemp{},
				Total:    0.0,
			}

			// Serializar y guardar
			newJSON, _ := json.Marshal(orderTemp)
			sess.Set("orderTemp", string(newJSON))
			if err := sess.Save(); err != nil {
				log.Printf("Error al guardar sesión nueva: %v", err)
			}

			// Redirigir a la página de nueva orden
			if tableNum > 0 {
				c.Set("HX-Redirect", fmt.Sprintf("/new-order/table/%d", tableNum))
				return c.SendString("Redirigiendo...")
			}
		}

		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso. Regrese a la página de mesas e inicie una nueva orden.")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		log.Printf("AddItemToTempOrder - Error deserializando orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al leer orden temporal")
	}

	// Obtener datos del producto a añadir
	productID, err := strconv.Atoi(c.FormValue("product_id"))
	if err != nil {
		log.Printf("AddItemToTempOrder - ID de producto inválido: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("ID de producto inválido")
	}

	quantity, err := strconv.Atoi(c.FormValue("quantity"))
	if err != nil || quantity <= 0 {
		quantity = 1 // Si hay error o es menor o igual a 0, usar 1 como valor predeterminado
	}

	notes := c.FormValue("notes")

	// Buscar información del producto
	var product Product
	if err := db.First(&product, productID).Error; err != nil {
		log.Printf("AddItemToTempOrder - Producto no encontrado: %v", err)
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Verificar si el producto ya existe en la orden
	found := false
	for i, item := range orderTemp.Items {
		if item.ProductID == uint(productID) && item.Notes == notes {
			// Actualizar cantidad si ya existe
			orderTemp.Items[i].Quantity += quantity
			orderTemp.Items[i].Subtotal = product.Price * float64(orderTemp.Items[i].Quantity)
			found = true
			break
		}
	}

	// Si no existe, añadir nuevo item
	if !found {
		newItem := OrderItemTemp{
			ProductID:   product.ID,
			ProductName: product.Name,
			Price:       product.Price,
			Quantity:    quantity,
			Notes:       notes,
			Subtotal:    product.Price * float64(quantity),
		}
		orderTemp.Items = append(orderTemp.Items, newItem)
	}

	// Recalcular total
	total := 0.0
	for _, item := range orderTemp.Items {
		total += item.Subtotal
	}
	orderTemp.Total = total

	// Guardar orden actualizada en sesión
	updatedJSON, err := json.Marshal(orderTemp)
	if err != nil {
		log.Printf("AddItemToTempOrder - Error serializando orden actualizada: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error actualizando orden")
	}

	sess.Set("orderTemp", string(updatedJSON))

	// IMPORTANTE: Guardar la sesión
	if err := sess.Save(); err != nil {
		log.Printf("AddItemToTempOrder - Error guardando sesión: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar sesión")
	}

	log.Printf("AddItemToTempOrder - Item añadido a orden temporal, ahora tiene %d productos",
		len(orderTemp.Items))

	// Notificar éxito
	c.Set("HX-Trigger", `{"showToast": "Producto añadido"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     orderTemp.Items,
		"Total":     orderTemp.Total,
		"ItemCount": len(orderTemp.Items),
	}, "")
}

// RemoveItemFromTempOrder elimina un producto de la orden temporal
func RemoveItemFromTempOrder(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al leer orden temporal")
	}

	// Índice del item a eliminar
	itemIndex, err := strconv.Atoi(c.Params("index"))
	if err != nil || itemIndex < 0 || itemIndex >= len(orderTemp.Items) {
		return c.Status(fiber.StatusBadRequest).SendString("Índice de producto inválido")
	}

	// Eliminar item
	removedItem := orderTemp.Items[itemIndex]
	orderTemp.Items = append(orderTemp.Items[:itemIndex], orderTemp.Items[itemIndex+1:]...)

	// Recalcular total
	total := 0.0
	for _, item := range orderTemp.Items {
		total += item.Subtotal
	}
	orderTemp.Total = total

	// Guardar orden actualizada en sesión
	updatedJSON, _ := json.Marshal(orderTemp)
	sess.Set("orderTemp", string(updatedJSON))
	if err := sess.Save(); err != nil {
		log.Printf("Error al guardar sesión: %v", err)
	}

	// Notificar que se eliminó un producto
	c.Set("HX-Trigger", fmt.Sprintf(`{"showToast": "%s eliminado de la orden"}`, removedItem.ProductName))

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     orderTemp.Items,
		"Total":     orderTemp.Total,
		"ItemCount": len(orderTemp.Items),
	}, "")
}

// UpdateTempOrderItemQuantity actualiza la cantidad de un item
func UpdateTempOrderItemQuantity(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al leer orden temporal")
	}

	// Índice del item y acción
	itemIndex, err := strconv.Atoi(c.Params("index"))
	if err != nil || itemIndex < 0 || itemIndex >= len(orderTemp.Items) {
		return c.Status(fiber.StatusBadRequest).SendString("Índice de producto inválido")
	}

	action := c.Params("action")
	if action != "increase" && action != "decrease" {
		return c.Status(fiber.StatusBadRequest).SendString("Acción inválida")
	}

	// Modificar cantidad
	if action == "increase" {
		orderTemp.Items[itemIndex].Quantity++
	} else {
		if orderTemp.Items[itemIndex].Quantity > 1 {
			orderTemp.Items[itemIndex].Quantity--
		}
	}

	// Actualizar subtotal
	orderTemp.Items[itemIndex].Subtotal = orderTemp.Items[itemIndex].Price * float64(orderTemp.Items[itemIndex].Quantity)

	// Recalcular total
	total := 0.0
	for _, item := range orderTemp.Items {
		total += item.Subtotal
	}
	orderTemp.Total = total

	// Guardar orden actualizada en sesión
	updatedJSON, _ := json.Marshal(orderTemp)
	sess.Set("orderTemp", string(updatedJSON))
	if err := sess.Save(); err != nil {
		log.Printf("Error al guardar sesión: %v", err)
	}

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     orderTemp.Items,
		"Total":     orderTemp.Total,
		"ItemCount": len(orderTemp.Items),
	}, "")
}

// ClearTempOrder elimina todos los productos de la orden temporal
func ClearTempOrder(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal para conservar el número de mesa
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al leer orden temporal")
	}

	tableNum := orderTemp.TableNum

	// Crear nueva orden temporal vacía
	orderTemp = OrderTemp{
		TableNum: tableNum,
		Items:    make([]OrderItemTemp, 0),
		Total:    0,
	}

	// Guardar orden actualizada en sesión
	updatedJSON, _ := json.Marshal(orderTemp)
	sess.Set("orderTemp", string(updatedJSON))
	if err := sess.Save(); err != nil {
		log.Printf("Error al guardar sesión: %v", err)
	}

	// Notificar limpieza
	c.Set("HX-Trigger", `{"showToast": "Orden limpiada"}`)

	// Devolver HTML actualizado
	return c.Render("partials/temp_order_preview", fiber.Map{
		"Items":     []OrderItemTemp{},
		"Total":     0.0,
		"ItemCount": 0,
	}, "")
}

// ConfirmTempOrder crea una orden real a partir de la temporal
func ConfirmTempOrder(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Error al leer orden temporal",
		})
	}

	// Validar que haya productos
	if len(orderTemp.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "La orden debe tener al menos un producto",
		})
	}

	// Notas adicionales
	notes := c.FormValue("notes")

	// Verificar que la mesa exista y no esté ocupada
	var table Table
	result := db.Where("number = ?", orderTemp.TableNum).First(&table)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Mesa no encontrada",
		})
	}

	if table.Occupied {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Esta mesa ya está ocupada",
		})
	}

	// Crear la orden real
	tx := db.Begin()

	order := Order{
		TableNum:  orderTemp.TableNum,
		Status:    "pending",
		Notes:     notes,
		Total:     orderTemp.Total,
		CreatedAt: time.Now(),
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("Error al crear orden: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Error al crear la orden",
		})
	}

	// Crear los items
	for _, tempItem := range orderTemp.Items {
		orderItem := OrderItem{
			OrderID:   order.ID,
			ProductID: tempItem.ProductID,
			Quantity:  tempItem.Quantity,
			Notes:     tempItem.Notes,
			IsReady:   false,
		}

		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			log.Printf("Error al crear item: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Error al crear los productos de la orden",
			})
		}
	}

	// Marcar la mesa como ocupada
	table.Occupied = true
	table.OrderID = &order.ID
	if err := tx.Save(&table).Error; err != nil {
		tx.Rollback()
		log.Printf("Error al marcar mesa como ocupada: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Error al marcar la mesa como ocupada",
		})
	}

	// Confirmar transacción
	tx.Commit()

	// Limpiar sesión
	sess.Delete("orderTemp")
	if err := sess.Save(); err != nil {
		log.Printf("Error al limpiar sesión: %v", err)
	}

	// Si es una solicitud HTMX, enviar header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/order/"+strconv.Itoa(int(order.ID)))
		c.Set("HX-Trigger", `{"showToast": "Orden #`+strconv.Itoa(int(order.ID))+` creada correctamente"}`)
		return c.SendString("Redirigiendo...")
	}

	// Responder con el ID de la orden creada
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Orden creada correctamente",
		"orderId": order.ID,
	})
}

// GetTempOrderSummary obtiene el resumen de la orden temporal para el modal de confirmación
func GetTempOrderSummary(c *fiber.Ctx) error {
	// Obtener sesión
	sess, err := sessionStore.Get(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener sesión")
	}

	// Obtener orden temporal
	orderTempJSON := sess.Get("orderTemp")
	if orderTempJSON == nil {
		return c.Status(fiber.StatusBadRequest).SendString("No hay orden temporal en curso")
	}

	var orderTemp OrderTemp
	if err := json.Unmarshal([]byte(orderTempJSON.(string)), &orderTemp); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al leer orden temporal")
	}

	// Renderizar el resumen
	return c.Render("partials/confirm_order_summary", fiber.Map{
		"Items":    orderTemp.Items,
		"Total":    orderTemp.Total,
		"TableNum": orderTemp.TableNum,
	}, "")
}
