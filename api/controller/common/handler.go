package common

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"pnt/api/pkg/e"
)

//HandleErr
func ResponseErr(c *gin.Context, code int, msg ...error) {
	if len(msg) > 0 {
		c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg[0].Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": code, "msg": e.GetMsg(code)})
	}

}

//HandleOk
func ResponseOK(c *gin.Context, data ...interface{}) {
	if len(data) > 0 {
		c.JSON(http.StatusOK, gin.H{"code": e.SUCCESS, "data": data[0], "msg": e.GetMsg(e.SUCCESS)})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": e.SUCCESS, "data": "", "msg": e.GetMsg(e.SUCCESS)})
	}
}
