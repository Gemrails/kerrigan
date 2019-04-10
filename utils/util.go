package utils

import (
	"container/list"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"pnt/db/model"
	"time"
)

func getMd5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

//UniqueID UniqueID
func UniqueID() string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return getMd5(base64.URLEncoding.EncodeToString(b))
}

//GetExternalIP outer ip
func GetExternalIP() (string, error) {
	resp, err := http.PostForm("http://ip.taobao.com/service/getIpInfo2.php", url.Values{"ip": {"myip"}})
	if resp.StatusCode != http.StatusOK || err != nil {
		time.Sleep(2 * time.Second)
		return GetExternalIP()
	}
	var tabooID struct {
		Code int
		Data map[string]string
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "127.0.0.1", err
	}

	if err := json.Unmarshal(content, &tabooID); err != nil {
		return "127.0.0.1", err
	}
	if tabooID.Code == 0 {
		return tabooID.Data["ip"], nil
	}
	return "127.0.0.1", nil
}

//GetMemory Mac无法使用syscall
//TODO: Get memery
// func GetMemory() uint64 {
// 	sysInfo := new(syscall.Sysinfo_t)
// 	err := syscall.SysinLfo(sysInfo)
// 	var total uint64 = 0
// 	if err == nil {
// 		total = sysInfo.Totalram * uint32(syscall.Getpagesize())
// 	}
// 	return total
// }

//ProdLocalIDWithMac ProdLocalIDWithMac
func ProdLocalIDWithMac() string {

	return ""
}

//TimeDuration TimeDuration
func TimeDuration(timeouts time.Duration) {
	for {
		select {
		case <-time.After(time.Second * timeouts):
			fmt.Printf("Finished listening.\n")
			return
		default:
			fmt.Printf("Listening...\n")
			//read from UDPConn here
		}
	}
}

//ListContains to judge if a value in list or not
func ListContains(l *list.List, value interface{}) (bool, *list.Element) {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value == value {
			return true, e
		}
	}
	return false, nil
}

//ListInsert to judge
func ListInsert(l *list.List, value interface{}) {
	//0 before, 1 after
	//judge first last item
	if l.Len() == 0 || l.Front().Value.(*model.BandWidthItem).GetBandwidth() < value.(*model.BandWidthItem).GetBandwidth() {
		l.PushFront(value)
	} else if l.Back().Value.(*model.BandWidthItem).GetBandwidth() > value.(*model.BandWidthItem).GetBandwidth() {
		l.PushBack(value)
	} else {
		for e := l.Front(); e != nil; e = e.Next() {
			if e.Value.(*model.BandWidthItem).GetBandwidth() < value.(*model.BandWidthItem).GetBandwidth() ||
				e.Value.(*model.BandWidthItem).GetBandwidth() == value.(*model.BandWidthItem).GetBandwidth() {
				l.InsertBefore(value, e)
			}
		}
	}
	//TODO: list lens setting
	ListLensFix(l, 500)
}

//ListLensFix fix list length
func ListLensFix(l *list.List, lth int) {
	for l.Len() > lth {
		l.Remove(l.Back())
	}
}

//IsFileExist file is exist
func IsFileExist(filePath string) bool {
	if filePath == "" {
		return false
	}
	_, err := os.Stat(filePath)
	return err == nil || os.IsExist(err)
}

func WriteToFile(path string, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		fmt.Printf("yaml Marshal err %s", err.Error())
		return err
	}
	return ioutil.WriteFile(path, data, 0777)
}
