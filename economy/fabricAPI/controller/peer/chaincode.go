package peer

import (
	"github.com/gin-gonic/gin"
	"io"
	"pnt/economy/fabricAPI/controller/common"
	"pnt/economy/fabricAPI/pkg/e"
	"pnt/economy/fabricAPI/services/peer"
)

func ChainCodeCmdHandler(c *gin.Context) {
	params := make([]byte, 1024)
	n, err := c.Request.Body.Read(params)
	if err != io.EOF {
		common.ResponseErr(c, e.ErrorBindParams)
		return
	}
	data, err := peer.RunCmd(c.Param("cca"), string(params[0:n]))
	if err == nil {
		common.ResponseOK(c, data)
		return
	}
	common.ResponseErr(c, e.ERROR, err)
}
