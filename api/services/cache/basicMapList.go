package cache

import (
	"fmt"
	"github.com/errors"
	"pnt/db"
	"pnt/db/model"
	"pnt/log"
	"reflect"
	"strconv"
)

//GetBasicInfoCache get ALL
func AllBasicNodeCache() map[string]*model.BasicNode {
	return db.GetManager().BasicNodeDao().GetItem()
}

//GetBasicNodeCacheByNodeId get cache by nid
func GetBasicNodeCacheByNodeId(nodeId string) map[string]*model.BasicNode {
	return db.GetManager().BasicNodeDao().GetItem(nodeId)
}

//GetSelfNodeCache get self node
func GetSelfNodeCache() *model.BasicNode {
	node, err := db.GetManager().BasicNodeDao().GetSelfItem()
	if err != nil {
		log.GetLogHandler().Errorf("get self node cache error %s", err.Error())
		return nil
	}
	return node
}

func AddBasicNodeCache(node *model.BasicNode) error {
	db.GetManager().BasicNodeDao().AddItem(node)
	return nil
}

//DelBasicInfoCacheByNodeId
func DelBasicNodeCache(nodeId string) error {
	err := db.GetManager().BasicNodeDao().DelItem(nodeId)
	return err
}

//SetBasicNodeCache SetBasicNodeCache 属性
func SetBasicNodeCache(nodeId string, key string, fieldName string, val string) error {
	c := GetBasicNodeCacheByNodeId(nodeId)
	nodeCache, ok := c[nodeId]
	if !ok {
		return errors.New(fmt.Sprintf("not find node id %s", nodeId))
	}
	node := reflect.ValueOf(nodeCache).Elem()
	k := node.FieldByName(key)
	if !k.IsValid() {
		return errors.New(fmt.Sprintf("key %s not exist", key))
	}
	field := k.FieldByName(fieldName)
	if !k.IsValid() {
		return errors.New(fmt.Sprintf("field %s not exist", fieldName))
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Bool:
		v, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		field.SetBool(v)
	case reflect.Float64:
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		field.SetFloat(v)
	case reflect.Int, reflect.Int64:
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(v)
	}
	n := node.Interface().(model.BasicNode)
	if err := db.GetManager().BasicNodeDao().UpdateItem(&n); err != nil {
		return err
	}
	return nil
}
