package main

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Rating      float64 `json:"rating"`
	Unit        string  `json:"unit"`
	Village     string  `json:"village"`
	Seller      string  `json:"seller"`
	Available   bool    `json:"available"`
	Stock       int     `json:"stock"`
	Description string  `json:"description"`
}

type Village struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	District string `json:"district"`
	State   string `json:"state"`
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var villages = []Village{
	{ID: "v1", Name: "Kudlu", District: "Udupi", State: "Karnataka"},
	{ID: "v2", Name: "Anekal", District: "Bengaluru Rural", State: "Karnataka"},
	{ID: "v3", Name: "Nandigama", District: "Krishna", State: "Andhra Pradesh"},
}

var categories = []Category{
	{ID: "c1", Name: "Vegetables"},
	{ID: "c2", Name: "Fruits"},
	{ID: "c3", Name: "Grains"},
	{ID: "c4", Name: "Dairy"},
}

var products = []Product{
	{ID: "p1", Name: "Fresh Tomato", Category: "Vegetables", Price: 38, Rating: 4.8, Unit: "kg", Village: "Kudlu", Seller: "Green Valley Farm", Available: true, Stock: 22, Description: "Locally grown, bright red tomatoes suited for daily cooking."},
	{ID: "p2", Name: "Village Mango", Category: "Fruits", Price: 72, Rating: 4.9, Unit: "kg", Village: "Anekal", Seller: "Sunrise Orchard", Available: true, Stock: 18, Description: "Sweet, seasonal mangoes from nearby orchard clusters."},
	{ID: "p3", Name: "Rice Paddy", Category: "Grains", Price: 44, Rating: 4.6, Unit: "kg", Village: "Nandigama", Seller: "Riverbank Mills", Available: true, Stock: 30, Description: "Traditional grain harvested from local paddy plots."},
	{ID: "p4", Name: "Farm Eggs", Category: "Dairy", Price: 12, Rating: 4.7, Unit: "dozen", Village: "Kudlu", Seller: "Happy Hen Co-op", Available: true, Stock: 40, Description: "Free-range eggs collected daily from village poultry units."},
	{ID: "p5", Name: "Cucumber", Category: "Vegetables", Price: 26, Rating: 4.5, Unit: "kg", Village: "Anekal", Seller: "Hill View Greens", Available: false, Stock: 0, Description: "Fresh, crunchy cucumbers ready for quick household meals."},
}

func main() {
	gin.SetMode(gin.DebugMode)
	r := gin.Default()

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"success": true,
			"message": "VillageConnect API is running",
		})
	})

	r.GET("/api/villages", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "villages": villages})
	})

	r.GET("/api/categories", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "categories": categories})
	})

	r.GET("/api/products", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true, "products": products})
	})

	r.GET("/api/products/:id", func(c *gin.Context) {
		id := c.Param("id")
		for _, product := range products {
			if product.ID == id {
				c.JSON(200, gin.H{"success": true, "product": product})
				return
			}
		}
		c.JSON(404, gin.H{"success": false, "message": "Product not found"})
	})

	r.GET("/api/search/products", func(c *gin.Context) {
		query := strings.TrimSpace(strings.ToLower(c.Query("q")))
		if query == "" {
			c.JSON(200, gin.H{"success": true, "products": products})
			return
		}

		matches := []Product{}
		for _, product := range products {
			name := strings.ToLower(product.Name)
			description := strings.ToLower(product.Description)
			category := strings.ToLower(product.Category)
			village := strings.ToLower(product.Village)
			if strings.Contains(name, query) || strings.Contains(description, query) || strings.Contains(category, query) || strings.Contains(village, query) {
				matches = append(matches, product)
			}
		}
		c.JSON(200, gin.H{"success": true, "products": matches})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
