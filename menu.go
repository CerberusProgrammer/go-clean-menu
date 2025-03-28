package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// MenuHandler muestra la página de administración del menú
func MenuHandler(c *fiber.Ctx) error {
	// Obtener todas las categorías
	var categories []string
	db.Model(&Product{}).Distinct().Pluck("category", &categories)

	// Obtener todos los productos
	var products []Product
	db.Order("category, name").Find(&products)

	return c.Render("menu", fiber.Map{
		"Title":      "Administración de Menú",
		"ActivePage": "menu",
		"Categories": categories,
		"Products":   products,
	})
}

// GetCategoryForm muestra el formulario para añadir una categoría
func GetCategoryForm(c *fiber.Ctx) error {
	return c.Render("partials/category_form", fiber.Map{})
}

// CreateCategory crea una nueva categoría
func CreateCategory(c *fiber.Ctx) error {
	categoryName := c.FormValue("name")
	if categoryName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Nombre de categoría requerido")
	}

	// Verificar si la categoría ya existe
	var count int64
	db.Model(&Product{}).Where("category = ?", categoryName).Count(&count)

	if count > 0 {
		return c.Status(fiber.StatusBadRequest).SendString("La categoría ya existe")
	}

	// Crear nueva categoría
	result := db.Exec("INSERT INTO categories (name, created_at, updated_at) VALUES (?, NOW(), NOW())", categoryName)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear categoría")
	}

	// Obtener todas las categorías para actualizar la UI
	var categories []string
	db.Model(&Product{}).Distinct().Pluck("category", &categories)

	return c.Render("partials/category_form", fiber.Map{
		"Categories": categories,
		"Success":    true,
		"Message":    "Categoría creada con éxito",
	})
}

// GetProductEditForm retorna el formulario para editar un producto
func GetProductEditForm(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var product Product
	if result := db.First(&product, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	var categories []string
	db.Model(&Product{}).Distinct().Pluck("category", &categories)

	return c.Render("partials/product_edit_form", fiber.Map{
		"Product":    product,
		"Categories": categories,
	})
}

// UpdateProduct actualiza un producto existente
func UpdateProduct(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var product Product
	if result := db.First(&product, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Actualizar campos
	product.Name = c.FormValue("name")
	product.Description = c.FormValue("description")
	product.Category = c.FormValue("category")

	price, err := strconv.ParseFloat(c.FormValue("price"), 64)
	if err == nil {
		product.Price = price
	}

	product.IsAvailable = c.FormValue("is_available") == "on"

	// Guardar cambios
	if result := db.Save(&product); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar producto")
	}

	// Obtener productos actualizados
	var products []Product
	db.Order("category, name").Find(&products)

	c.Set("HX-Trigger", `{"showToast": "Producto actualizado correctamente"}`)
	return c.Render("partials/product_list", fiber.Map{
		"Products": products,
	})
}

// DeleteProduct elimina un producto
func DeleteProduct(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	// Verificar si el producto está usado en órdenes
	var count int64
	db.Model(&OrderItem{}).Where("product_id = ?", id).Count(&count)

	if count > 0 {
		// Solo marcar como no disponible sin eliminar
		db.Model(&Product{}).Where("id = ?", id).Update("is_available", false)
		c.Set("HX-Trigger", `{"showToast": "El producto está en uso y ha sido marcado como no disponible"}`)
	} else {
		// Eliminar si no está en uso
		db.Delete(&Product{}, id)
		c.Set("HX-Trigger", `{"showToast": "Producto eliminado correctamente"}`)
	}

	// Obtener productos actualizados
	var products []Product
	db.Order("category, name").Find(&products)

	return c.Render("partials/product_list", fiber.Map{
		"Products": products,
	})
}
