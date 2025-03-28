package main

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// MenuHandler muestra la página de administración del menú
func MenuHandler(c *fiber.Ctx) error {
	// Obtener todas las categorías
	var categories []string
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	// Estadísticas del menú
	var productCount int64
	db.Model(&Product{}).Count(&productCount)

	var categoryCount int64
	categoryCount = int64(len(categories))

	// Obtener productos más vendidos
	type TopProduct struct {
		ID       uint
		Name     string
		Category string
		Count    int64
	}
	var topProducts []TopProduct
	db.Raw(`SELECT p.id, p.name, p.category, COUNT(oi.id) as count 
           FROM products p 
           JOIN order_items oi ON p.id = oi.product_id 
           GROUP BY p.id, p.name, p.category 
           ORDER BY count DESC LIMIT 5`).
		Scan(&topProducts)

	return c.Render("menu", fiber.Map{
		"Title":         "Administración de Menú",
		"ActivePage":    "menu",
		"Categories":    categories,
		"ProductCount":  productCount,
		"CategoryCount": categoryCount,
		"TopProducts":   topProducts,
	})
}

// GetProducts obtiene todos los productos o filtrados por categoría
func GetProducts(c *fiber.Ctx) error {
	var products []Product
	query := db.Order("name")

	// Filtrar por categoría si existe el parámetro
	category := c.Query("category")
	if category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}

	// Filtrar por búsqueda si existe
	search := c.Query("search")
	if search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Filtrar por disponibilidad
	availability := c.Query("availability")
	if availability == "available" {
		query = query.Where("is_available = ?", true)
	} else if availability == "unavailable" {
		query = query.Where("is_available = ?", false)
	}

	// Ordenar resultados
	sortBy := c.Query("sort", "name")
	sortOrder := c.Query("order", "asc")

	if sortOrder == "desc" {
		query = query.Order(sortBy + " DESC")
	} else {
		query = query.Order(sortBy)
	}

	query.Find(&products)

	// Obtener todas las categorías para los filtros
	var categories []string
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	return c.Render("partials/product_list", fiber.Map{
		"Products":   products,
		"Categories": categories,
		"Filters": fiber.Map{
			"Category":     category,
			"Search":       search,
			"Availability": availability,
			"SortBy":       sortBy,
			"SortOrder":    sortOrder,
		},
	})
}

// GetCategoryForm muestra el formulario para añadir una categoría
func GetCategoryForm(c *fiber.Ctx) error {
	var categories []string
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	return c.Render("partials/category_form", fiber.Map{
		"Categories": categories,
	})
}

// CreateCategory crea una nueva categoría
func CreateCategory(c *fiber.Ctx) error {
	categoryName := strings.TrimSpace(c.FormValue("name"))
	if categoryName == "" {
		c.Set("HX-Trigger", `{"showToast": "El nombre de la categoría no puede estar vacío"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Nombre de categoría requerido")
	}

	// Verificar si la categoría ya existe
	var count int64
	db.Model(&Product{}).Where("category = ?", categoryName).Count(&count)

	if count > 0 {
		c.Set("HX-Trigger", `{"showToast": "Esta categoría ya existe"}`)
		return c.Status(fiber.StatusBadRequest).SendString("La categoría ya existe")
	}

	// Crear un producto vacío con la nueva categoría para registrarla
	dummyProduct := Product{
		Name:        "Categoría: " + categoryName,
		Description: "Categoría temporal - puede eliminar este producto",
		Category:    categoryName,
		Price:       0.01,
		IsAvailable: false,
	}

	if result := db.Create(&dummyProduct); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al crear la categoría"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear categoría")
	}

	// Obtener todas las categorías para actualizar la UI
	var categories []string
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	c.Set("HX-Trigger", `{"showToast": "Categoría '`+categoryName+`' creada con éxito", "refreshCategories": true}`)
	return c.Render("partials/category_list", fiber.Map{
		"Categories": categories,
	})
}

func GetProductForm(c *fiber.Ctx) error {
	var categories []string
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	return c.Render("partials/product_form", fiber.Map{
		"Categories": categories,
		"IsNew":      true,
	})
}

// CreateProduct crea un nuevo producto
func CreateProduct(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))
	category := strings.TrimSpace(c.FormValue("category"))
	priceStr := strings.TrimSpace(c.FormValue("price"))

	if name == "" || category == "" || priceStr == "" {
		c.Set("HX-Trigger", `{"showToast": "Todos los campos obligatorios deben completarse"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Todos los campos obligatorios son requeridos")
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price < 0 {
		c.Set("HX-Trigger", `{"showToast": "El precio debe ser un número válido mayor o igual a cero"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Precio inválido")
	}

	// Crear producto
	product := Product{
		Name:        name,
		Description: description,
		Category:    category,
		Price:       price,
		IsAvailable: c.FormValue("is_available") == "on",
	}

	if result := db.Create(&product); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al crear el producto"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al crear producto")
	}

	c.Set("HX-Trigger", `{"showToast": "Producto '`+name+`' creado exitosamente", "refreshProducts": true}`)

	// Redirigir a la lista de productos actualizada
	return GetProducts(c)
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
	db.Model(&Product{}).Distinct().Order("category").Pluck("category", &categories)

	return c.Render("partials/product_form", fiber.Map{
		"Product":    product,
		"Categories": categories,
		"IsNew":      false,
	})
}

// UpdateProduct actualiza un producto existente
func UpdateProduct(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		c.Set("HX-Trigger", `{"showToast": "ID de producto inválido"}`)
		return c.Status(fiber.StatusBadRequest).SendString("ID inválido")
	}

	var product Product
	if result := db.First(&product, id); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Producto no encontrado"}`)
		return c.Status(fiber.StatusNotFound).SendString("Producto no encontrado")
	}

	// Actualizar campos
	product.Name = strings.TrimSpace(c.FormValue("name"))
	product.Description = strings.TrimSpace(c.FormValue("description"))
	product.Category = strings.TrimSpace(c.FormValue("category"))

	price, err := strconv.ParseFloat(c.FormValue("price"), 64)
	if err == nil && price >= 0 {
		product.Price = price
	}

	product.IsAvailable = c.FormValue("is_available") == "on"

	// Guardar cambios
	if result := db.Save(&product); result.Error != nil {
		c.Set("HX-Trigger", `{"showToast": "Error al actualizar el producto"}`)
		return c.Status(fiber.StatusInternalServerError).SendString("Error al actualizar producto")
	}

	c.Set("HX-Trigger", `{"showToast": "Producto '`+product.Name+`' actualizado correctamente", "closeModal": true, "refreshProducts": true}`)

	// Redirigir a la lista de productos actualizada
	return GetProducts(c)
}

// DeleteProduct elimina un producto
func DeleteProduct(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		c.Set("HX-Trigger", `{"showToast": "ID de producto inválido"}`)
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

	// Redirigir a la lista de productos actualizada
	return GetProducts(c)
}

// BulkAction realiza acciones masivas sobre productos seleccionados
func BulkAction(c *fiber.Ctx) error {
	action := c.FormValue("action")
	if action == "" {
		c.Set("HX-Trigger", `{"showToast": "Acción no especificada"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Acción no especificada")
	}

	// Obtener IDs de productos seleccionados
	productIDs := c.FormValue("product_ids")
	if productIDs == "" {
		c.Set("HX-Trigger", `{"showToast": "No se seleccionaron productos"}`)
		return c.Status(fiber.StatusBadRequest).SendString("No se seleccionaron productos")
	}

	// Convertir la cadena de IDs a slice
	ids := strings.Split(productIDs, ",")

	// Realizar la acción correspondiente
	switch action {
	case "enable":
		db.Model(&Product{}).Where("id IN ?", ids).Update("is_available", true)
		c.Set("HX-Trigger", `{"showToast": "Productos habilitados correctamente"}`)
	case "disable":
		db.Model(&Product{}).Where("id IN ?", ids).Update("is_available", false)
		c.Set("HX-Trigger", `{"showToast": "Productos deshabilitados correctamente"}`)
	case "delete":
		// Verificar si algún producto está usado en órdenes
		var count int64
		db.Model(&OrderItem{}).Where("product_id IN ?", ids).Count(&count)

		if count > 0 {
			// Solo marcar como no disponible
			db.Model(&Product{}).Where("id IN ?", ids).Update("is_available", false)
			c.Set("HX-Trigger", `{"showToast": "Algunos productos están en uso y han sido marcados como no disponibles"}`)
		} else {
			// Eliminar si no están en uso
			db.Delete(&Product{}, "id IN ?", ids)
			c.Set("HX-Trigger", `{"showToast": "Productos eliminados correctamente"}`)
		}
	default:
		c.Set("HX-Trigger", `{"showToast": "Acción no reconocida"}`)
		return c.Status(fiber.StatusBadRequest).SendString("Acción no reconocida")
	}

	// Redirigir a la lista de productos actualizada
	return GetProducts(c)
}
