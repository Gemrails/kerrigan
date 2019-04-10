package kerrigan

import (
	"github.com/gin-gonic/gin"
	"os"
	"pnt/api/controller/common"
	"time"
)

func StopHandler(c *gin.Context) {
	go func() {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()
	common.ResponseOK(c)
}
