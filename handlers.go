package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// HomeHandler muestra la página principal
func HomeHandler(c *fiber.Ctx) error {
	return c.Render("index", fiber.Map{
		"Title": "Menú de Restaurante",
	})
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
			"CreatedAt": order.CreatedAt,
		})
	}

	return result
}

// DashboardHandler muestra el panel de control con estadísticas en tiempo real
func DashboardHandler(c *fiber.Ctx) error {
	stats := getDashboardStats()
	recentOrders := getRecentOrders(5)
	popularProducts := getPopularProducts(5)
	chartLabels, chartValues := getSalesChartData(7)

	return c.Render("dashboard", fiber.Map{
		"Title":           "Panel de Control",
		"ActivePage":      "dashboard",
		"Stats":           stats,
		"RecentOrders":    recentOrders,
		"PopularProducts": popularProducts,
		"ChartLabels":     chartLabels,
		"ChartValues":     chartValues,
	})
}

// getDashboardStats calcula estadísticas en tiempo real
func getDashboardStats() fiber.Map {
	var activeOrderCount int64
	db.Model(&Order{}).Where("status = ?", "pending").Count(&activeOrderCount)

	// Calcular ventas del día
	today := time.Now().Truncate(24 * time.Hour)
	var todaySales float64
	db.Model(&Order{}).
		Where("status = ? AND created_at >= ?", "completed", today).
		Select("COALESCE(SUM(total), 0)").
		Scan(&todaySales)

	// Categoría más popular
	var topCategory struct {
		Category string
		Count    int64
	}

	db.Raw(`SELECT p.category, COUNT(oi.id) as count 
            FROM products p 
            JOIN order_items oi ON p.id = oi.product_id 
            JOIN orders o ON oi.order_id = o.id
            WHERE o.created_at >= ?
            GROUP BY p.category 
            ORDER BY count DESC LIMIT 1`, time.Now().AddDate(0, 0, -30)).
		Scan(&topCategory)

	// Si no hay categorías, establecer valor predeterminado
	if topCategory.Category == "" {
		topCategory.Category = "N/A"
	}

	// Total de mesas ocupadas
	var occupiedTables int64
	db.Model(&Table{}).Where("occupied = ?", true).Count(&occupiedTables)

	// Total de productos
	var totalProducts int64
	db.Model(&Product{}).Count(&totalProducts)

	return fiber.Map{
		"ActiveOrders":   activeOrderCount,
		"TodaySales":     todaySales,
		"TopCategory":    topCategory.Category,
		"OccupiedTables": occupiedTables,
		"TotalProducts":  totalProducts,
	}
}

func getPopularProducts(limit int) []fiber.Map {
	type PopularProduct struct {
		ID         uint
		Name       string
		OrderCount int64
		Revenue    float64
	}

	var popularProducts []PopularProduct
	db.Raw(`SELECT p.id, p.name, 
               COUNT(oi.id) as order_count,
               SUM(p.price * oi.quantity) as revenue
            FROM products p 
            JOIN order_items oi ON p.id = oi.product_id 
            JOIN orders o ON oi.order_id = o.id
            WHERE o.status = 'completed'
            GROUP BY p.id, p.name 
            ORDER BY order_count DESC, revenue DESC
            LIMIT ?`, limit).
		Scan(&popularProducts)

	result := make([]fiber.Map, 0)
	for _, product := range popularProducts {
		result = append(result, fiber.Map{
			"ID":         product.ID,
			"Name":       product.Name,
			"OrderCount": product.OrderCount,
			"Revenue":    product.Revenue,
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
			Select("COALESCE(SUM(total), 0)").
			Scan(&dayTotal)

		labels[days-1-i] = dayStart.Format("02/01")
		values[days-1-i] = dayTotal
	}

	return labels, values
}

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
	}, "")
}

// GetOrderMetrics muestra métricas de tiempos de órdenes y productos
func GetOrderMetrics(c *fiber.Ctx) error {
	// Órdenes completadas en el último mes
	var orders []Order
	db.Preload("Items").Where("status = ? AND completed_at IS NOT NULL", "completed").Order("completed_at desc").Limit(50).Find(&orders)

	// Métricas globales
	totalOrders := len(orders)
	var sumTaking, sumCooking, sumDelivery, sumTotal float64
	var productTimes = make(map[string][]float64) // nombre producto -> tiempos

	for _, o := range orders {
		if o.SentToKitchenAt != nil && o.CreatedAt.Before(*o.SentToKitchenAt) {
			sumTaking += o.SentToKitchenAt.Sub(o.CreatedAt).Seconds()
		}
		if o.CookingCompletedAt != nil && o.SentToKitchenAt != nil && o.CookingCompletedAt.After(*o.SentToKitchenAt) {
			sumCooking += o.CookingCompletedAt.Sub(*o.SentToKitchenAt).Seconds()
		}
		if o.DeliveredAt != nil && o.CookingCompletedAt != nil && o.DeliveredAt.After(*o.CookingCompletedAt) {
			sumDelivery += o.DeliveredAt.Sub(*o.CookingCompletedAt).Seconds()
		}
		if o.CompletedAt != nil {
			sumTotal += o.CompletedAt.Sub(o.CreatedAt).Seconds()
		}
		for _, item := range o.Items {
			if item.CookingTime > 0 && item.Product.Name != "" {
				productTimes[item.Product.Name] = append(productTimes[item.Product.Name], float64(item.CookingTime))
			}
		}
	}

	// Promedios
	avgTaking := 0.0
	avgCooking := 0.0
	avgDelivery := 0.0
	avgTotal := 0.0
	if totalOrders > 0 {
		avgTaking = sumTaking / float64(totalOrders)
		avgCooking = sumCooking / float64(totalOrders)
		avgDelivery = sumDelivery / float64(totalOrders)
		avgTotal = sumTotal / float64(totalOrders)
	}

	// Promedio por producto
	var productAverages []struct {
		Name  string
		Avg   float64
		Count int
	}
	for name, times := range productTimes {
		sum := 0.0
		for _, t := range times {
			sum += t
		}
		productAverages = append(productAverages, struct {
			Name  string
			Avg   float64
			Count int
		}{name, sum / float64(len(times)), len(times)})
	}

	return c.Render("metrics", fiber.Map{
		"Title":           "Métricas",
		"ActivePage":      "metrics",
		"AvgTaking":       avgTaking,
		"AvgCooking":      avgCooking,
		"AvgDelivery":     avgDelivery,
		"AvgTotal":        avgTotal,
		"ProductAverages": productAverages,
		"Orders":          orders,
	})
}
