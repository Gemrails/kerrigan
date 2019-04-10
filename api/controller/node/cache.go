package node

import (
	"github.com/gin-gonic/gin"
	"pnt/api/controller/common"
	"pnt/api/pkg/e"
	"pnt/api/services/cache"
	"pnt/db/model"
)

//GetAllCacheHandler 获取全部节点信息
func GetAllCacheHandler(c *gin.Context) {
	common.ResponseOK(c, cache.AllBasicNodeCache())
}

//GetCacheByNodeIdHandler  获取指定节点信息
func GetCacheByNodeIdHandler(c *gin.Context) {
	common.ResponseOK(c, cache.GetBasicNodeCacheByNodeId(c.Param("nodeId")))
}

//GetSelfCacheHandler get self node
func GetSelfCacheHandler(c *gin.Context) {
	common.ResponseOK(c, cache.GetSelfNodeCache())
}

//AddCacheHandler 添加节点信息
func AddCacheHandler(c *gin.Context) {
	data := model.BasicNode{}
	if err := c.ShouldBindJSON(&data); err != nil {
		common.ResponseErr(c, e.ERROR, err)
		return
	}
	if err := cache.AddBasicNodeCache(&data); err != nil {
		common.ResponseErr(c, e.ERROR, err)
		return
	}
	common.ResponseOK(c)
}

//SetCacheByNodeIdHandler 更新节点信息
func SetCacheByNodeIdHandler(c *gin.Context) {
	data := SetCacheParams{}
	if err := c.ShouldBindJSON(&data); err != nil {
		common.ResponseErr(c, e.ErrorBindParams, err)
		return
	}
	if err := cache.SetBasicNodeCache(c.Param("nodeId"), data.Key, data.FieldName, data.Value); err != nil {
		common.ResponseErr(c, e.ERROR, err)
		return
	}

	common.ResponseOK(c)
}

//DeleteCacheListHandler 删除节点信息
func DelCacheListHandler(c *gin.Context) {
	if err := cache.DelBasicNodeCache(c.Param("nodeId")); err != nil {
		common.ResponseErr(c, e.ERROR, err)
		return
	}
	common.ResponseOK(c)
}
