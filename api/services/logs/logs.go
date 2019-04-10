package logs

import "pnt/log"

func GetLogChache(lines ...int) []string {
	var data []string
	logStack := log.GetLogStack()
	line := len(logStack.LogChan)	// 默认读取全部
	if len(lines) > 0 {
		if lines[0] < line {
			line = lines[0]
		}
	}

	for i := len(logStack.LogChan); i > 0; i-- {
		c := <-logStack.LogChan
		if line >= i {
			data = append(data, c.Msg)
		}
	}
	return data
}
