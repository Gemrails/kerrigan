package test_model

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

//保存可支持协议的节点信息
var consistedProtocol map[string][]string

type levelJudge struct {
	Level      []byte
	TotalScore int
	NodeId     string
	IsWork     bool
}

type NodeSorted struct {
	NodeId string
	Speed  float64
	IsWork bool
}

//缓存通用接口
type cacheManager interface {
	Update()
	ADD()
	DEL()
	PUT()
}

//NodeSorted:按照Speed排序,重写方法
type SortedBySpeed []NodeSorted

type NodeSortedPool struct {
	SortedSlice []NodeSorted
	m           sync.Mutex
}

func (a SortedBySpeed) Len() int {
	return len(a)
}

func (a SortedBySpeed) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a SortedBySpeed) Less(i, j int) bool {
	return a[i].Speed > a[j].Speed
}

//types except http,https,tcp
func Update(nodeId, typeProtocal string) {
	consistedProtocol[typeProtocal] = append(consistedProtocol[typeProtocal], nodeId)
}

//func (a *NodeSorted) ADD(nodeId string){
func (a *NodeSortedPool) ADD(nodeId string, speed float64) {
	a.m.Lock()
	node := NodeSorted{nodeId, speed, true}
	a.SortedSlice = append(a.SortedSlice, node)
	a.m.Unlock()
	//added into consistedProtocol
	consistedProtocol["http"] = append(consistedProtocol["http"], nodeId)
	consistedProtocol["https"] = append(consistedProtocol["https"], nodeId)
	consistedProtocol["tcp"] = append(consistedProtocol["tcp"], nodeId)

	//TODO::if consist more types,maybe needed to modify interface
}

//improve entry criteria : schedule interface
//func (n *NodeSortedPool) SelectNode string{
func SelectNode(n *NodeSortedPool) string {
	rand.Seed(time.Now().Unix()) // set time seeds
	return n.SortedSlice[rand.Intn(len(n.SortedSlice))].NodeId
}

//func (n *NodeSortedPool) GetPercentage float64{
func GetPercentage(n *NodeSortedPool) float64 {
	len1 := len(n.SortedSlice)
	percentage := 0.0
	tmpPer := 40
	if len1 < 5 {
		percentage = 1.0
	} else {
		for {
			if tmpPer != 100 && n.SortedSlice[int(len1*tmpPer/100)].Speed > 15.0 {
				tmpPer += 10
				continue
			} else {
				percentage = float64(tmpPer) / 100
				break
			}

		}
	}
	return percentage
}

func (node *NodeSortedPool) DEL(nodeId string) {
	node.m.Lock()
	for k, _ := range node.SortedSlice {
		if node.SortedSlice[k].NodeId == nodeId {
			node.SortedSlice = append(node.SortedSlice[:k], node.SortedSlice[k+1:]...)
		}
	}
	node.m.Unlock()
}

//func (node *NodeSortedPool)SortCache{
func SortCache(node *NodeSortedPool) {
	node.m.Lock()
	len1 := len(node.SortedSlice)
	//for i:= 0;i<len(node.SortedSlice);i++{
	for i := 0; i < len1; i++ {
		if i == len1-1 && node.SortedSlice[i-1].Speed < 14.0 {
			node.SortedSlice = node.SortedSlice[0 : i-1]
			break
		} else if node.SortedSlice[i].Speed < 14.0 {
			node.SortedSlice = append(node.SortedSlice[:i], node.SortedSlice[i+1:]...)
		}
	}
	sort.Sort(SortedBySpeed(node.SortedSlice))
	fmt.Println(node)
	node.m.Unlock()
	//sort.Sort

}

func main() {
	var a NodeSortedPool
	consistedProtocol = make(map[string][]string, 16)
	a.ADD("12313", 12.0)
	a.ADD("1dfd3", 16.0)
	a.ADD("1fg13", 18.0)
	a.ADD("123fh", 14.0)
	a.ADD("123fh", 19.0)
	a.ADD("123fh", 17.0)
	a.ADD("123fh", 13.0)
	a.ADD("123fh", 20.0)
	a.ADD("123fh", 22.0)

	sort.Sort(SortedBySpeed(a.SortedSlice))
	fmt.Println(a.SortedSlice)
	/*
	   fmt.Println(consistedProtocol)
	   kk := SelectNode(&a)
	   fmt.Println(kk)
	*/

	nn := GetPercentage(&a)
	fmt.Println(nn)
	SortCache(&a)
	fmt.Println(a.SortedSlice)
}
