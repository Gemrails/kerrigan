package e

var MsgFlags = map[int]string{
	SUCCESS:         "OK",
	ERROR:           "ERROR",
	ErrorBindParams: "error bind params",
	NoFileWasFound:  "No file was found",
}

func GetMsg(code int) string {
	return MsgFlags[code]
}
