package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

var engine *gin.Engine
var pixels map[string]string

func main() {
	engine = gin.Default()
	pixels = make(map[string]string)
	engine.LoadHTMLGlob("./templates/*")
	engine.SetTrustedProxies(nil)
	engine.GET("/", func(context *gin.Context) {
		context.HTML(http.StatusOK, "index.html", gin.H{})
	})
	engine.GET("/pixels", func(context *gin.Context) {
		context.JSON(http.StatusOK, pixels)
		fmt.Println(pixels)
	})
	engine.POST("/updatePixel", func(context *gin.Context) {
		var data PixelData
		context.ShouldBindJSON(&data)
		pixels[data.ID] = data.Color
		fmt.Println("Socket:", data)
	})
	err := engine.Run(":5050")
	if err != nil {
		panic("Engine failed to Start")
		return
	}
}

type PixelData struct {
	ID    string `json:"id"`
	Color string `json:"color"`
}
