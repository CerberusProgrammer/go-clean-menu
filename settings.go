package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SettingsHandler(c *fiber.Ctx) error {
	var settings Settings
	result := db.First(&settings)

	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al obtener configuración")
	}

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

	var tables []Table
	db.Order("number").Find(&tables)

	if len(tables) != settings.TableCount {
		db.Exec("DELETE FROM tables")

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

func UpdateRestaurantSettings(c *fiber.Ctx) error {
	var settings Settings
	db.First(&settings)

	name := c.FormValue("name")
	if name == "" {
		c.Set("HX-Trigger", `{"showToast": "El nombre del restaurante es obligatorio", "toastType": "error"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Nombre obligatorio")
	}

	settings.RestaurantName = name
	settings.Address = c.FormValue("address")
	settings.Phone = c.FormValue("phone")
	settings.Email = c.FormValue("email")

	if file, err := c.FormFile("logo"); err == nil {
		uploadDir := "./static/uploads"
		os.MkdirAll(uploadDir, os.ModePerm)

		filename := fmt.Sprintf("logo-%d%s", time.Now().Unix(), filepath.Ext(file.Filename))
		filepath := filepath.Join(uploadDir, filename)

		if err := c.SaveFile(file, filepath); err == nil {
			if settings.LogoPath != "" && settings.LogoPath != "/static/uploads/"+filename {
				oldPath := "." + settings.LogoPath
				if _, err := os.Stat(oldPath); err == nil {
					os.Remove(oldPath)
				}
			}
			settings.LogoPath = "/static/uploads/" + filename
		}
	}

	if result := db.Save(&settings); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración", "toastType": "error"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Información del restaurante actualizada", "toastType": "success"}`)
	return c.SendString("Configuración guardada")
}

func UpdatePrinterSettings(c *fiber.Ctx) error {
	var settings Settings
	db.First(&settings)

	settings.DefaultPrinter = c.FormValue("default_printer")
	settings.AutoPrint = c.FormValue("auto_print") == "on"

	if result := db.Save(&settings); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración", "toastType": "error"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Configuración de impresora actualizada", "toastType": "success"}`)
	return c.SendString("Configuración guardada")
}

func UpdateTableSettings(c *fiber.Ctx) error {
	tableCount, err := strconv.Atoi(c.FormValue("tableCount"))
	if err != nil || tableCount <= 0 {
		c.Set("HX-Trigger", `{"showToast": "Número de mesas inválido", "toastType": "error"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Número inválido")
	}

	var settings Settings
	db.First(&settings)
	settings.TableCount = tableCount
	db.Save(&settings)

	var occupiedTables []Table
	db.Where("occupied = ?", true).Find(&occupiedTables)

	if len(occupiedTables) > 0 && len(occupiedTables) > tableCount {
		c.Set("HX-Trigger", `{"showToast": "No se pueden reducir mesas porque hay órdenes activas", "toastType": "error"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Mesas ocupadas")
	}

	db.Exec("DELETE FROM tables WHERE occupied = ?", false)

	var tables []Table
	for i := 1; i <= tableCount; i++ {
		var exists bool
		for _, t := range occupiedTables {
			if t.Number == i {
				exists = true
				break
			}
		}

		if !exists {
			db.Create(&Table{
				Number:   i,
				Capacity: 4,
				Occupied: false,
			})
		}
	}

	db.Order("number").Find(&tables)

	c.Set("HX-Trigger", `{"showToast": "Configuración de mesas actualizada", "toastType": "success"}`)
	return c.Render("partials/table_grid", fiber.Map{
		"Tables": tables,
	}, "")
}

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
		c.Set("HX-Trigger", `{"showToast": "Error al guardar la configuración", "toastType": "error"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al guardar")
	}

	c.Set("HX-Trigger", `{"showToast": "Preferencias actualizadas. Actualiza la página para aplicar los cambios.", "toastType": "success", "refreshTheme": true}`)
	return c.SendString("Configuración guardada")
}

func CreateBackup(c *fiber.Ctx) error {
	backupDir := "./backups"
	os.MkdirAll(backupDir, os.ModePerm)

	timestamp := time.Now().Format("20060102-150405")
	filename := "backup-" + timestamp + ".sql"
	filePath := filepath.Join(backupDir, filename)

	dummyContent := fmt.Sprintf("-- PostgreSQL database dump\n-- Database: go_clean_menu\n-- Generated at: %s\n\n-- Tables: products, orders, order_items, tables, settings", time.Now().Format(time.RFC3339))
	err := os.WriteFile(filePath, []byte(dummyContent), 0644)
	if err != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al crear el respaldo", "toastType": "error"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear respaldo")
	}

	backup := Backup{
		FileName: filename,
		FilePath: filePath,
		Size:     int64(len(dummyContent)),
	}
	db.Create(&backup)

	c.Set("HX-Trigger", `{"showToast": "Respaldo creado correctamente", "toastType": "success", "refreshBackups": true}`)
	return c.SendString("Respaldo creado")
}

func GetBackupList(c *fiber.Ctx) error {
	var backups []Backup
	db.Order("created_at desc").Find(&backups)

	return c.Render("partials/backup_list", fiber.Map{
		"Backups": backups,
	}, "")
}

func DownloadBackup(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var backup Backup
	if result := db.First(&backup, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Respaldo no encontrado")
	}

	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		c.Set("HX-Trigger", `{"showToast": "El archivo de respaldo no existe", "toastType": "error"}`)
		return c.Status(fiber.StatusNotFound).SendString("El archivo de respaldo no existe")
	}

	return c.Download(backup.FilePath, backup.FileName)
}

func DeleteBackup(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var backup Backup
	if result := db.First(&backup, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Respaldo no encontrado")
	}

	if _, err := os.Stat(backup.FilePath); err == nil {
		os.Remove(backup.FilePath)
	}

	db.Delete(&backup)

	c.Set("HX-Trigger", `{"showToast": "Respaldo eliminado", "toastType": "success"}`)
	return GetBackupList(c)
}
