package main

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

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
	})
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
	})
}

// OrdersHandler muestra todas las órdenes activas
func OrdersHandler(c *fiber.Ctx) error {
	var orders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Find(&orders)

	// Calcular ítems por orden
	for i := range orders {
		var count int64
		db.Model(&OrderItem{}).Where("order_id = ?", orders[i].ID).Count(&count)
		orders[i].Items = make([]OrderItem, int(count))
	}

	return c.Render("orders", fiber.Map{
		"Title":      "Órdenes Activas",
		"ActivePage": "orders",
		"Orders":     orders,
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
