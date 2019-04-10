package router

import (
	"github.com/gin-gonic/gin"
	"pnt/api/controller/kerrigan"
	"pnt/api/controller/logs"
	"pnt/api/controller/node"
	"pnt/api/controller/proxys"
	"pnt/api/middleware"
)

func Route(router *gin.Engine) {
	// 使用middleware
	//authorized := router.Group("/api/v1",middleware.AuthRequired)

	v1 := router.Group("/api/v1", middleware.RequestLogMiddleware())
	{
		v1.GET("/node", node.GetAllCacheHandler)
		v1.POST("/node", node.AddCacheHandler)
		v1.PUT("/node/:nodeId", node.SetCacheByNodeIdHandler)
		v1.GET("/node/:nodeId", node.GetCacheByNodeIdHandler)
		v1.GET("/self/node", node.GetSelfCacheHandler)
		v1.DELETE("/node/:nodeId", node.DelCacheListHandler)

		v1.GET("/log", logs.GetAllLogsHandler)
		v1.GET("/log/:line", logs.GetLogsListHandler)

		v1.POST("/proxy/request", proxys.HttpProxyHandler)

		v1.POST("/kerrigan/stop", kerrigan.StopHandler)
	}

}
