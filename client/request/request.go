package request

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"pnt/conf"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

const TimeOut = 10 * time.Second

func getHost() string {
	var domain = "http://127.0.0.1:"
	if c, err := conf.GetKerriConfig(); err == nil {
		return domain + strconv.Itoa(c.APIConfig.Port)
	}
	return domain + "48899"
}

//Get Get Request Api
func Get(url string) (*Response, error) {
	data := &Response{}
	res, err := http.Get(getHost() + url)
	if err != nil || res.StatusCode != http.StatusOK {
		return data, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	json.Unmarshal(body, data)
	return data, nil
}

//Post Post Request Api
func Post(url string, params string) (*Response, error) {
	data := &Response{}
	res, err := http.Post(getHost()+url, "application/json", strings.NewReader(params))
	if err != nil || res.StatusCode != http.StatusOK {
		return data, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	json.Unmarshal(body, data)
	return data, nil

}

//Put Put Request Api
func Put(url string, params string) (*Response, error) {
	data := &Response{}
	req, err := http.NewRequest(http.MethodPut, getHost()+url, strings.NewReader(params))
	if err != nil {
		return data, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: TimeOut}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		return data, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	json.Unmarshal(body, data)
	return data, nil

}

//Delete Delete Request Api
func Delete(url string) (*Response, error) {
	req, err := http.NewRequest(http.MethodDelete, getHost()+url, nil)
	data := &Response{}
	if err != nil {
		return data, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: TimeOut}
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK {
		return data, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return data, err
	}
	json.Unmarshal(body, data)
	return data, nil
}
