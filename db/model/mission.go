package mission

import (
	"time"
)

const (
	MISSIONRUN = iota
	MISSIONBREAK
	MISSIONWRONG
	MISSIONCOMPLETE
)

type Mission struct {
	MissionID     string
	MissionType   string
	StartTime     time.Duration
	LastFreshTime time.Duration
	RelateNode    []string
	Status        int
	PluginID      string
	DNatAddr      []string
}

//NewBasicNode return basic node
func NewMissionTab() *Mission {
	r := make([]string)
	d := make([]string)
	return &Mission{
		RelateNode: r,
		DNatAddr:   d,
	}
}

//ModelName return model name
func (m *Mission) ModelName() string {
	return "node_mission"
}
