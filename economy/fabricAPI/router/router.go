package router

import (
	"github.com/gin-gonic/gin"
	"pnt/economy/fabricAPI/controller/peer"
)

func Route(router *gin.Engine) {
	// 使用middleware
	//authorized := router.Group("/api/v1",middleware.AuthRequired)

	v1 := router.Group("/api/v1")
	{
		v1.POST("/peer/chaincode/:cca", peer.ChainCodeCmdHandler)
	}

}
