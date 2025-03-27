package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// HomeHandler muestra la página principal
func HomeHandler(c *fiber.Ctx) error {
	return c.Render("index", fiber.Map{
		"Title": "Menú de Restaurante",
	})
}

// GetProducts obtiene todos los productos del menú
func GetProducts(c *fiber.Ctx) error {
	var products []Product
	result := db.Order("category, name").Find(&products)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener productos",
		})
	}

	// Si es una solicitud HTMX, renderiza parcial
	if c.Get("HX-Request") == "true" {
		return c.Render("partials/product_list", fiber.Map{
			"Products": products,
		})
	}

	return c.JSON(products)
}

// GetProductsByCategory obtiene productos filtrados por categoría
func GetProductsByCategory(c *fiber.Ctx) error {
	category := c.Params("category")

	var products []Product
	result := db.Where("category = ?", category).Order("name").Find(&products)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener productos",
		})
	}

	return c.Render("partials/product_list", fiber.Map{
		"Products": products,
	})
}

// CreateOrder crea una nueva orden
func CreateOrder(c *fiber.Ctx) error {
	tableNum, err := strconv.Atoi(c.FormValue("table_num"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Número de mesa inválido")
	}

	order := Order{
		TableNum: tableNum,
		Status:   "pending",
		Total:    0,
	}

	result := db.Create(&order)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear orden")
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
	db.Order("category, name").Find(&products)

	return c.Render("order", fiber.Map{
		"Title":    "Detalle de Orden #" + strconv.Itoa(id),
		"Order":    order,
		"Products": products,
	})
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
		quantity = 1
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

	// Crear el item de la orden
	orderItem := OrderItem{
		OrderID:   uint(orderID),
		ProductID: uint(productID),
		Quantity:  quantity,
		Notes:     notes,
	}

	if result := db.Create(&orderItem); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al añadir producto")
	}

	// Actualizar total de la orden
	order.Total += product.Price * float64(quantity)
	db.Save(&order)

	// Cargar la orden actualizada con sus items
	db.Preload("Items").Preload("Items.Product").First(&order, orderID)

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
	var item OrderItem
	if err := db.Preload("Product").First(&item, itemID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Item no encontrado")
	}

	// Eliminar el item
	db.Delete(&OrderItem{}, itemID)

	// Actualizar total de la orden
	var order Order
	db.First(&order, orderID)
	order.Total -= item.Product.Price * float64(item.Quantity)
	db.Save(&order)

	// Cargar la orden actualizada con sus items
	db.Preload("Items").Preload("Items.Product").First(&order, orderID)

	return c.Render("partials/order_items", fiber.Map{
		"Order": order,
	})
}

// CompleteOrder marca una orden como completada
func CompleteOrder(c *fiber.Ctx) error {
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

	return c.Redirect("/")
}
