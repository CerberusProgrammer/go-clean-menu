package main

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HistoryHandler muestra la página de historial de órdenes
func HistoryHandler(c *fiber.Ctx) error {
	// Por defecto mostrar las órdenes de hoy
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.AddDate(0, 0, 1)

	var completedOrders []Order
	db.Where("status = ? AND created_at BETWEEN ? AND ?", "completed", today, tomorrow).
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	// Calcular la cantidad de ítems para cada orden
	ordersData := prepareOrdersForDisplay(completedOrders)

	return c.Render("history", fiber.Map{
		"Title":      "Historial de Órdenes",
		"ActivePage": "history",
		"Orders":     ordersData,
		"StartDate":  today.Format("2006-01-02"),
		"EndDate":    tomorrow.Format("2006-01-02"),
		"FilterType": "today",
	})
}

// GetTodayHistory obtiene las órdenes de hoy
func GetTodayHistory(c *fiber.Ctx) error {
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.AddDate(0, 0, 1)

	var completedOrders []Order
	db.Where("status = ? AND created_at BETWEEN ? AND ?", "completed", today, tomorrow).
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	ordersData := prepareOrdersForDisplay(completedOrders)

	return c.Render("partials/order_history", fiber.Map{
		"Orders":     ordersData,
		"StartDate":  today.Format("2006-01-02"),
		"EndDate":    tomorrow.Format("2006-01-02"),
		"FilterType": "today",
	}, "")
}

// GetWeekHistory obtiene las órdenes de la semana actual
func GetWeekHistory(c *fiber.Ctx) error {
	now := time.Now()

	// Calcular el inicio de la semana (domingo o lunes, depende de la configuración regional)
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, now.Location())

	weekEnd := weekStart.AddDate(0, 0, 7)

	var completedOrders []Order
	db.Where("status = ? AND created_at BETWEEN ? AND ?", "completed", weekStart, weekEnd).
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	ordersData := prepareOrdersForDisplay(completedOrders)

	return c.Render("partials/order_history", fiber.Map{
		"Orders":     ordersData,
		"StartDate":  weekStart.Format("2006-01-02"),
		"EndDate":    weekEnd.Format("2006-01-02"),
		"FilterType": "week",
	}, "")
}

// GetMonthHistory obtiene las órdenes del mes actual
func GetMonthHistory(c *fiber.Ctx) error {
	now := time.Now()

	// Primer día del mes actual
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Primer día del mes siguiente
	nextMonth := monthStart.AddDate(0, 1, 0)

	var completedOrders []Order
	db.Where("status = ? AND created_at BETWEEN ? AND ?", "completed", monthStart, nextMonth).
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	ordersData := prepareOrdersForDisplay(completedOrders)

	return c.Render("partials/order_history", fiber.Map{
		"Orders":     ordersData,
		"StartDate":  monthStart.Format("2006-01-02"),
		"EndDate":    nextMonth.Format("2006-01-02"),
		"FilterType": "month",
	}, "")
}

// GetCustomHistory obtiene órdenes en un rango de fechas personalizado
func GetCustomHistory(c *fiber.Ctx) error {
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	layout := "2006-01-02"
	startDate, err1 := time.Parse(layout, startDateStr)
	endDate, err2 := time.Parse(layout, endDateStr)

	if err1 != nil || err2 != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Fechas inválidas")
	}

	// Ajustar la fecha final para incluir todo el día
	endDate = endDate.Add(24 * time.Hour)

	var completedOrders []Order
	db.Where("status = ? AND created_at BETWEEN ? AND ?", "completed", startDate, endDate).
		Order("created_at desc").
		Preload("Items").
		Find(&completedOrders)

	ordersData := prepareOrdersForDisplay(completedOrders)

	return c.Render("partials/order_history", fiber.Map{
		"Orders":     ordersData,
		"StartDate":  startDateStr,
		"EndDate":    endDateStr,
		"FilterType": "custom",
	}, "")
}

// Función auxiliar para preparar los datos de órdenes para mostrar
func prepareOrdersForDisplay(orders []Order) []fiber.Map {
	result := make([]fiber.Map, 0, len(orders))

	for _, order := range orders {
		// Calcular estadísticas de la orden
		itemCount := len(order.Items)

		orderData := fiber.Map{
			"ID":        order.ID,
			"TableNum":  order.TableNum,
			"Status":    order.Status,
			"Total":     order.Total,
			"ItemCount": itemCount,
			"CreatedAt": order.CreatedAt,
		}

		result = append(result, orderData)
	}

	return result
}

// GenerateOrderReport genera un informe PDF de una orden
func GenerateOrderReport(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	// Obtener la orden con todos sus detalles
	var order Order
	result := db.Preload("Items").Preload("Items.Product").First(&order, id)
	if result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Orden no encontrada")
	}

	// Aquí implementarías la generación real del PDF
	// Por ahora simulamos devolviendo un mensaje

	c.Set("HX-Trigger", `{"showToast": "Reporte generado"}`)

	return c.SendString("Reporte generado correctamente")
}
