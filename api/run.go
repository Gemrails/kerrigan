package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"pnt/api/router"
	"pnt/conf"
)

func Run(c *conf.Config) {
	app := gin.Default()
	router.Route(app)
	gin.SetMode(c.APIConfig.Mode)
	addr := fmt.Sprintf(":%d", c.APIConfig.Port)
	log.Printf("Api Server start listen addr %s", addr)
	app.Run(addr)

}
