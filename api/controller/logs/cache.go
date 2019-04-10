package logs

import (
	"github.com/gin-gonic/gin"
	"pnt/api/controller/common"
	"pnt/api/pkg/e"
	"pnt/api/services/logs"
	"strconv"
)

func GetAllLogsHandler(c *gin.Context) {
	common.ResponseOK(c, logs.GetLogChache())
}

func GetLogsListHandler(c *gin.Context) {
	line, err := strconv.Atoi(c.Param("line"))
	if err != nil {
		common.ResponseErr(c, e.ErrorBindParams)
		return
	}
	common.ResponseOK(c, logs.GetLogChache(line))
}
