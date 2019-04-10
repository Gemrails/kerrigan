package proxys

import (
	"bufio"
	"io"
	"os"
	"pnt/api/pkg/utils/request"
	"pnt/log"
	"pnt/policy"
	"pnt/tunn"
	"strings"
)

func HttpProxy(path string) error {
	if err := os.Setenv(tunn.STRATEGY, string(policy.PROXYFULL)); err != nil {
		return err
	}
	defer os.Setenv(tunn.STRATEGY, "")

	f, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				go reqProxy(line)
				break
			} else {
				return err
			}
		}
		go reqProxy(line)
	}
	return nil
}

func reqProxy(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	if _, err := request.ProxyGet(url); err != nil {
		log.GetLogHandler().Errorf("request %s error: %s", url, err.Error())
	} else {
		log.GetLogHandler().Infof("request %s success", url)
	}
}
