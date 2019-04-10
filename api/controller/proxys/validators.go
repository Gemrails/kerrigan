package proxys

type HttpProxyParams struct {
	Path string `json:"path" binding:"required"`
}
