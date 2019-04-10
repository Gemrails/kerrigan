package cache

import (
	"github.com/Sirupsen/logrus"
	"pnt/conf"
	cacheDao "pnt/db/cache/dao"
	"pnt/db/cache/funcs"
	"pnt/db/dao"
	"pnt/log"
)

//Manager cache manager
type Manager struct {
	conf  *conf.Config
	log   *logrus.Logger
	cache *funcs.LocalCache
}

//CreateCacheManager create cache manager
func CreateCacheManager(c *conf.Config) (*Manager, error) {
	m := new(Manager)
	m.conf = c
	m.log = log.GetLogHandler()
	m.cache = funcs.GetCacheInstance()
	m.log.Infoln("create cache manager success.")
	return m, nil
}

//LogManager log manager
func (m *Manager) LogManager() *logrus.Logger {
	return m.log
}

//CacheManager cache manager
func (m *Manager) CacheManager() *funcs.LocalCache {
	return m.cache
}

//BasicNodeDao return basic node dao
func (m *Manager) BasicNodeDao() dao.BasicNodeDao {
	return cacheDao.NewBasicNodeImpl(m.log, m.cache)
}

//RepeaterNodeDao return repeater dao
func (m *Manager) RepeaterNodeDao() dao.RepeaterNodeDao {
	return cacheDao.NewRepeaterNodeImpl(m.log, m.cache)
}

//BandWidthProviderDao return bandwidth dao
func (m *Manager) BandWidthProviderDao() dao.BandWidthProviderDao {
	return cacheDao.NewBandWidthProviderImpl(m.log, m.cache)
}

//KcpConnect
func (m *Manager) KcpConnectDao() dao.KcpConnectDao {
	return cacheDao.NewKcpConnectImpl(m.log, m.cache)
}
