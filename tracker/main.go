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
	c.JSON(http.StatusOK, "Hello! as you can see, it is working!")
}

func get_all_manifests(c *gin.Context) {
	c.JSON(http.StatusOK, "Now, there's only one manifest. And it's invalid. But it is a file at least. it's name is \"first_manifest\"")
}

func get_manifest_by_name(c *gin.Context) {
	name := c.Param("name")
	if name == "first_manifest" {
		c.File("file_storage/first_manifest.txt")
		return
	}
	c.JSON(http.StatusBadRequest, "there's no such manifest")
}

func main() {
	fmt.Println("Starting our tracker")

	router := gin.Default()
	router.GET("/hello", hello)
	router.GET("/get/all/manifests", get_all_manifests)

	router.GET("/get/manifest/by/name/:name", get_manifest_by_name)

	router.Run(":8080")
}
