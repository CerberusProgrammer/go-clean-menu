package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// TablesHandler muestra la página de administración de mesas
func TablesHandler(c *fiber.Ctx) error {
	// Obtener todas las mesas
	var tables []Table
	db.Order("number").Find(&tables)

	// Obtener configuración para saber el número total de mesas configurado
	var settings Settings
	db.First(&settings)

	return c.Render("tables", fiber.Map{
		"Title":      "Administración de Mesas",
		"ActivePage": "tables",
		"Tables":     tables,
		"Settings":   settings,
	})
}

// CreateTable crea una nueva mesa
func CreateTable(c *fiber.Ctx) error {
	// Obtener número de mesa del formulario
	tableNum, err := strconv.Atoi(c.FormValue("table_num"))
	if err != nil {
		c.Set("HX-Trigger", `{"showToast": "Error: Número de mesa inválido"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Número de mesa inválido")
	}

	// Verificar si la mesa ya existe
	var existingTable Table
	result := db.Where("number = ?", tableNum).First(&existingTable)
	if result.Error == nil {
		c.Set("HX-Trigger", `{"showToast": "Error: Esta mesa ya existe"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Esta mesa ya existe")
	}

	// Crear la nueva mesa
	capacity, _ := strconv.Atoi(c.FormValue("capacity"))
	if capacity <= 0 {
		capacity = 4 // Capacidad predeterminada
	}

	table := Table{
		Number:   tableNum,
		Capacity: capacity,
		Occupied: false,
	}

	if result := db.Create(&table); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al crear la mesa"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear la mesa")
	}

	// Actualizar la configuración de mesas si es necesario
	var settings Settings
	db.First(&settings)
	if tableNum > settings.TableCount {
		settings.TableCount = tableNum
		db.Save(&settings)
	}

	// Obtener todas las mesas actualizadas
	var tables []Table
	db.Order("number").Find(&tables)

	c.Set("HX-Trigger", `{"showToast": "Mesa #`+strconv.Itoa(tableNum)+` creada correctamente"}`)
	return c.Render("partials/tables_grid", fiber.Map{
		"Tables": tables,
	}, "")
}

// DeleteTable elimina una mesa
func DeleteTable(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var table Table
	if result := db.First(&table, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Mesa no encontrada")
	}

	// Verificar si la mesa está ocupada
	if table.Occupied {
		c.Set("HX-Trigger", `{"showToast": "Error: No se puede eliminar una mesa ocupada"}`)
		return c.Status(fiber.StatusBadRequest).SendString("No se puede eliminar una mesa ocupada")
	}

	// Eliminar mesa
	db.Delete(&table)

	// Obtener mesas actualizadas
	var tables []Table
	db.Order("number").Find(&tables)

	c.Set("HX-Trigger", `{"showToast": "Mesa #`+strconv.Itoa(table.Number)+` eliminada correctamente"}`)
	return c.Render("partials/tables_grid", fiber.Map{
		"Tables": tables,
	}, "")
}

// ResetTables elimina todas las mesas y crea nuevas según la configuración
func ResetTables(c *fiber.Ctx) error {
	// Verificar si hay mesas ocupadas
	var occupiedCount int64
	db.Model(&Table{}).Where("occupied = ?", true).Count(&occupiedCount)
	if occupiedCount > 0 {
		c.Set("HX-Trigger", `{"showToast": "Error: Hay mesas ocupadas que no se pueden eliminar"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Hay mesas ocupadas")
	}

	// Obtener la configuración
	var settings Settings
	db.First(&settings)

	// Eliminar todas las mesas actuales
	db.Exec("DELETE FROM tables WHERE occupied = false")

	// Crear nuevas mesas
	for i := 1; i <= settings.TableCount; i++ {
		table := Table{
			Number:   i,
			Capacity: 4,
			Occupied: false,
		}
		db.Create(&table)
	}

	// Obtener mesas actualizadas
	var tables []Table
	db.Order("number").Find(&tables)

	c.Set("HX-Trigger", `{"showToast": "Mesas restablecidas correctamente"}`)
	return c.Render("partials/tables_grid", fiber.Map{
		"Tables": tables,
	}, "")
}
