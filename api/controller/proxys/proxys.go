package proxys

import (
	"github.com/gin-gonic/gin"
	"pnt/api/controller/common"
	"pnt/api/pkg/e"
	"pnt/api/services/proxys"
	"pnt/utils"
)

func HttpProxyHandler(c *gin.Context) {
	params := HttpProxyParams{}
	if err := c.ShouldBindJSON(&params); err != nil {
		common.ResponseErr(c, e.ErrorBindParams, err)
		return
	}
	if !utils.IsFileExist(params.Path) {
		common.ResponseErr(c, e.NoFileWasFound)
		return
	}
	proxys.HttpProxy(params.Path)
	common.ResponseOK(c)
}
