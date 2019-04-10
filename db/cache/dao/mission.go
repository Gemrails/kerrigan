package dao

import (
	"pnt/db/cache/funcs"
	"pnt/db/model"

	"github.com/Sirupsen/logrus"
)

//MissionImpl  mission interface
type MissionImpl struct {
	cache *funcs.LocalCache
	log   *logrus.Logger
}

//NewMissionImpl new basic node impl
func NewMissionImpl(l *logrus.Logger, f *funcs.LocalCache) *BandWidthProviderImpl {
	return &MissionImpl{
		cache: f,
		log:   l,
	}
}

//CreateTable to create table
func (m *MissionImpl) CreateTable(mo model.DBInterface) {
	missionP := mo.(*model.Mission)
	m.cache.CacheTable(missionP.ModelName())
	m.log.Infoln("Create table ", missionP.ModelName())
}

//AddItem add item, overall handle
func (m *MissionImpl) AddItem(mo model.DBInterface) {
	missionP := mo.(*model.Mission)
	if missionP.MissionID == "" {
		m.log.Debugln("No mission_id was found.")
		return
	}
	m.cache.CacheTable(missionP.ModelName()).NotFoundAdd(mission.MissionID, TIMEINTERVAL, missionP)
	m.log.Infof("Add mission item %s success", mission.MissionID)
}

//UpdateItem update item: del first, then add item, overall handle
func (m *MissionImpl) UpdateItem(mo model.DBInterface) error {
	missionP := mo.(*model.Mission)
	if missionP.MissionID == "" {
		m.log.Debugln("No mission_id was found.")
		return
	}
	_, err := m.cache.CacheTable(missionP.ModelName()).Delete(missionP.MissionID)
	if err != nil {
		m.log.Errorf("Update mission item %s error: %s", missionP.MissionID, err.Error())
		return err
	}
	m.AddItem(missionP)
	m.log.Infof("Update mission item %s success", missionP.MissionID)
	return nil
}

//GetItem get item
func (m *MissionImpl) GetItem(missionID ...string) (mb map[string]*model.BasicNod) {
	return nil, nil
}

//DelItem delete item
func (m *MissionImpl) DelItem(missionID string) error {
	_, err := m.cache.CacheTable(new(model.Mission).ModelName()).Delete(missionID)
	if err != nil {
		m.log.Errorf("Delete mission item %s error: %s", missionID, err.Error())
		return err
	}
	return nil
}

//GetMission return a mission by mission id
func (m *MissionImpl) GetMission(missionID string) (*model.Mission, error) {
	v, err := m.cache.CacheTable(new(model.Mission).ModelName()).Value(missionID)
	if err != nil {
		m.log.Errorln("Get mission item error: ", err.Error())
		return nil, err
	}
	return v.Data().(*model.Mission), nil
}
