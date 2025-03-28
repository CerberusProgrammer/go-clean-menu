package main

import (
	"strconv"
	"time"

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

// Modifica la función CreateOrder
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

	// Si es una solicitud HTMX, envía un header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/order/"+strconv.Itoa(int(order.ID)))
		return c.SendString("")
	}

	// Redirección normal para solicitudes no-HTMX
	return c.Redirect("/order/" + strconv.Itoa(int(order.ID)))
}

// Modifica la función CompleteOrder
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

	// Si es una solicitud HTMX, envía un header de redirección para HTMX
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/")
		return c.SendString("")
	}

	// Redirección normal para solicitudes no-HTMX
	return c.Redirect("/")
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

// ... código existente ...

// DashboardHandler muestra el panel de control
func DashboardHandler(c *fiber.Ctx) error {
	stats := getDashboardStats()
	recentOrders := getRecentOrders(5)
	popularProducts := getPopularProducts(5)
	chartLabels, chartValues := getSalesChartData(7)

	return c.Render("dashboard", fiber.Map{
		"Title":           "Panel de Control",
		"ActivePage":      "dashboard",
		"CurrentTime":     time.Now().Format("02/01/2006 15:04"),
		"Stats":           stats,
		"RecentOrders":    recentOrders,
		"PopularProducts": popularProducts,
		"ChartLabels":     chartLabels,
		"ChartValues":     chartValues,
	})
}

// Funciones auxiliares para el dashboard
func getDashboardStats() fiber.Map {
	var activeOrderCount int64
	db.Model(&Order{}).Where("status = ?", "pending").Count(&activeOrderCount)

	// Calcular ventas del día
	today := time.Now().Truncate(24 * time.Hour)
	var todaySales float64
	db.Model(&Order{}).
		Where("status = ? AND created_at >= ?", "completed", today).
		Select("SUM(total)").
		Scan(&todaySales)

	// Categoría más popular
	var topCategory string
	db.Raw(`SELECT p.category FROM order_items oi 
            JOIN products p ON oi.product_id = p.id 
            GROUP BY p.category 
            ORDER BY COUNT(*) DESC LIMIT 1`).
		Scan(&topCategory)

	if topCategory == "" {
		topCategory = "N/A"
	}

	return fiber.Map{
		"ActiveOrders": activeOrderCount,
		"TodaySales":   todaySales,
		"TopCategory":  topCategory,
	}
}

func getRecentOrders(limit int) []fiber.Map {
	var orders []Order
	db.Preload("Items").Order("created_at desc").Limit(limit).Find(&orders)

	result := make([]fiber.Map, 0)
	for _, order := range orders {
		result = append(result, fiber.Map{
			"ID":        order.ID,
			"TableNum":  order.TableNum,
			"Status":    order.Status,
			"Total":     order.Total,
			"ItemCount": len(order.Items),
		})
	}

	return result
}

func getPopularProducts(limit int) []fiber.Map {
	type ProductCount struct {
		ID         uint
		Name       string
		OrderCount int64
	}

	var popularProducts []ProductCount
	db.Raw(`SELECT p.id, p.name, COUNT(oi.id) as order_count 
            FROM products p 
            JOIN order_items oi ON p.id = oi.product_id 
            GROUP BY p.id, p.name 
            ORDER BY order_count DESC LIMIT ?`, limit).
		Scan(&popularProducts)

	result := make([]fiber.Map, 0)
	for _, product := range popularProducts {
		result = append(result, fiber.Map{
			"ID":         product.ID,
			"Name":       product.Name,
			"OrderCount": product.OrderCount,
		})
	}

	return result
}

func getSalesChartData(days int) ([]string, []float64) {
	labels := make([]string, days)
	values := make([]float64, days)

	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var dayTotal float64
		db.Model(&Order{}).
			Where("status = ? AND created_at BETWEEN ? AND ?", "completed", dayStart, dayEnd).
			Select("SUM(total)").
			Scan(&dayTotal)

		labels[days-1-i] = dayStart.Format("02/01")
		values[days-1-i] = dayTotal
	}

	return labels, values
}

// MenuHandler muestra la página de administración del menú
func MenuHandler(c *fiber.Ctx) error {
	var categories []string
	db.Model(&Product{}).Distinct().Pluck("category", &categories)

	return c.Render("menu", fiber.Map{
		"Title":       "Administración de Menú",
		"ActivePage":  "menu",
		"CurrentTime": time.Now().Format("02/01/2006 15:04"),
		"Categories":  categories,
	})
}

// HistoryHandler muestra la página de historial de órdenes
func HistoryHandler(c *fiber.Ctx) error {
	var completedOrders []Order
	db.Where("status = ?", "completed").
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	return c.Render("history", fiber.Map{
		"Title":       "Historial de Órdenes",
		"ActivePage":  "history",
		"CurrentTime": time.Now().Format("02/01/2006 15:04"),
		"Orders":      completedOrders,
	})
}

// KitchenHandler muestra la vista de cocina
func KitchenHandler(c *fiber.Ctx) error {
	var pendingOrders []Order
	db.Where("status = ?", "pending").
		Order("created_at asc").
		Preload("Items").
		Preload("Items.Product").
		Find(&pendingOrders)

	return c.Render("kitchen", fiber.Map{
		"Title":       "Vista de Cocina",
		"ActivePage":  "kitchen",
		"CurrentTime": time.Now().Format("02/01/2006 15:04"),
		"Orders":      pendingOrders,
	})
}

// SettingsHandler muestra la página de configuración
func SettingsHandler(c *fiber.Ctx) error {
	// Obtener configuración desde la base de datos o usar valores predeterminados
	settings := fiber.Map{
		"RestaurantName": "Resto",
		"Address":        "123 Calle Principal",
		"Phone":          "(555) 123-4567",
		"DefaultPrinter": "thermal1",
		"AutoPrint":      true,
		"TableCount":     12,
		"DarkMode":       false,
		"AutoRefresh":    true,
		"Language":       "es",
	}

	// Simular información de mesas
	tables := make([]fiber.Map, 0)
	for i := 1; i <= 12; i++ {
		tables = append(tables, fiber.Map{
			"Number":   i,
			"Occupied": i%3 == 0, // Solo para simulación
		})
	}

	return c.Render("settings", fiber.Map{
		"Title":       "Configuración",
		"ActivePage":  "settings",
		"CurrentTime": time.Now().Format("02/01/2006 15:04"),
		"Settings":    settings,
		"Tables":      tables,
	})
}
