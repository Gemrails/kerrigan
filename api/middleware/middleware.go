package middleware

import (
	"github.com/gin-gonic/gin"
)

func RequestLogMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		//l := log.GetLogHandler()
		//body, _ := ioutil.ReadAll(context.Request.Body)
		//l.WithFields(logrus.Fields{"url": context.Request.URL, "body": string(body[:])}).Info()
		context.Next()
	}
}
