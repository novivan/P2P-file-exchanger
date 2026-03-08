package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type file struct {
	name string
	body []byte
}

func hello(c *gin.Context) {
	c.JSON(http.StatusOK, "Hello!")
}

func main() {
	fmt.Println("Starting our tracker")

	router := gin.Default()
	router.GET("/hello", hello)

	router.Run(":8080")
}
