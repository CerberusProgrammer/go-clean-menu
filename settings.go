package main

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// SettingsHandler muestra la página de configuración
func SettingsHandler(c *fiber.Ctx) error {
	// Obtener configuración desde la base de datos
	var settings Settings
	result := db.First(&settings)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener configuración")
	}

	// Si no hay configuración, crear una predeterminada
	if result.Error == gorm.ErrRecordNotFound {
		settings = Settings{
			RestaurantName: "Resto",
			Address:        "123 Calle Principal",
			Phone:          "(555) 123-4567",
			Email:          "contacto@resto.com",
			DefaultPrinter: "thermal1",
			AutoPrint:      true,
			TableCount:     12,
			DarkMode:       false,
			AutoRefresh:    true,
			Language:       "es",
			TaxRate:        0.16,
			CurrencySymbol: "$",
		}
		db.Create(&settings)
	}

	// Obtener información de las mesas
	var tables []Table
	db.Order("number").Find(&tables)

	// Si no hay mesas o el número no coincide con el configurado, crear/actualizar
	if len(tables) != settings.TableCount {
		// Eliminar todas las mesas primero
		db.Exec("DELETE FROM tables")

		// Recrear las mesas
		tables = make([]Table, settings.TableCount)
		for i := 0; i < settings.TableCount; i++ {
			tables[i] = Table{
				Number:   i + 1,
				Capacity: 4,
				Occupied: false,
			}
			db.Create(&tables[i])
		}
	}

	// Obtener backups existentes
	var backups []Backup
	db.Order("created_at desc").Find(&backups)

	return c.Render("settings", fiber.Map{
		"Title":      "Configuración",
		"ActivePage": "settings",
		"Settings":   settings,
		"Tables":     tables,
		"Backups":    backups,
	})
}

// UpdateRestaurantSettings actualiza la información del restaurante
func UpdateRestaurantSettings(c *fiber.Ctx) error {
	var settings Settings
	db.First(&settings)

	settings.RestaurantName = c.FormValue("name")
	settings.Address = c.FormValue("address")
	settings.Phone = c.FormValue("phone")
	settings.Email = c.FormValue("email")

	if result := db.Save(&settings); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Información del restaurante actualizada"}`)
	return c.SendString("Configuración guardada")
}

// UpdatePrinterSettings actualiza la configuración de impresora
func UpdatePrinterSettings(c *fiber.Ctx) error {
	var settings Settings
	db.First(&settings)

	settings.DefaultPrinter = c.FormValue("default_printer")
	settings.AutoPrint = c.FormValue("auto_print") == "on"

	if result := db.Save(&settings); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Configuración de impresora actualizada"}`)
	return c.SendString("Configuración guardada")
}

func UpdateTableSettings(c *fiber.Ctx) error {
	tableCount, err := strconv.Atoi(c.FormValue("tableCount"))
	if err != nil || tableCount <= 0 {
		c.Set("HX-Trigger", `{"showToast": "Número de mesas inválido"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Número inválido")
	}

	// Actualizar la configuración
	var settings Settings
	db.First(&settings)
	settings.TableCount = tableCount
	db.Save(&settings)

	// Verificar mesas actuales
	var count int64
	db.Model(&Table{}).Count(&count)

	// Si hay discrepancia, recrear mesas
	if int(count) != tableCount {
		// Eliminar todas las mesas primero
		db.Exec("DELETE FROM tables")

		// Crear nuevas mesas
		for i := 1; i <= tableCount; i++ {
			db.Create(&Table{
				Number:   i,
				Capacity: 4,
				Occupied: false,
			})
		}
	}

	// Obtener la lista actualizada
	var tables []Table
	db.Order("number").Find(&tables)

	c.Set("HX-Trigger", `{"showToast": "Configuración de mesas actualizada"}`)
	return c.Render("partials/table_grid", fiber.Map{
		"Tables": tables,
	})
}

// UpdateAppSettings actualiza las preferencias de la aplicación
func UpdateAppSettings(c *fiber.Ctx) error {
	var settings Settings
	db.First(&settings)

	settings.DarkMode = c.FormValue("dark_mode") == "on"
	settings.AutoRefresh = c.FormValue("auto_refresh") == "on"
	settings.Language = c.FormValue("language")

	taxRateStr := c.FormValue("tax_rate")
	if taxRateStr != "" {
		if taxRate, err := strconv.ParseFloat(taxRateStr, 64); err == nil {
			settings.TaxRate = taxRate
		}
	}

	settings.CurrencySymbol = c.FormValue("currency_symbol")
	if settings.CurrencySymbol == "" {
		settings.CurrencySymbol = "$"
	}

	if result := db.Save(&settings); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Preferencias actualizadas"}`)
	return c.SendString("Configuración guardada")
}

// CreateBackup genera una copia de seguridad de la base de datos
func CreateBackup(c *fiber.Ctx) error {
	// Crear directorio de backups si no existe
	backupDir := "./backups"
	os.MkdirAll(backupDir, os.ModePerm)

	// Generar nombre de archivo con timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := "backup-" + timestamp + ".sql"
	filePath := filepath.Join(backupDir, filename)

	// En producción implementarías el respaldo real de PostgreSQL
	// Ejemplo: pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME > $filePath

	// Simular un archivo de respaldo
	dummyContent := "-- PostgreSQL database dump\n-- Database: go_clean_menu\n-- Generated at: " + timestamp
	err := os.WriteFile(filePath, []byte(dummyContent), 0644)
	if err != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al crear el respaldo"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear respaldo")
	}

	// Crear registro en base de datos
	backup := Backup{
		FileName: filename,
		FilePath: filePath,
		Size:     int64(len(dummyContent)),
	}
	db.Create(&backup)

	c.Set("HX-Trigger", `{"showToast": "Respaldo creado correctamente"}`)
	return c.SendString("Respaldo creado")
}

// GetBackupList obtiene la lista de respaldos
func GetBackupList(c *fiber.Ctx) error {
	var backups []Backup
	db.Order("created_at desc").Find(&backups)

	return c.Render("partials/backup_list", fiber.Map{
		"Backups": backups,
	})
}

// DownloadBackup permite descargar un archivo de respaldo
func DownloadBackup(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var backup Backup
	if result := db.First(&backup, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Respaldo no encontrado")
	}

	// En producción verificarías si el archivo existe realmente
	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).SendString("El archivo de respaldo no existe")
	}

	return c.Download(backup.FilePath, backup.FileName)
}
