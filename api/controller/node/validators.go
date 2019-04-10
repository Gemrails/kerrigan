package node

type SetCacheParams struct {
	Key       string `json:"key" binding:"required"`
	FieldName string `json:"field_name" binding:"required"`
	Value     string `json:"value" binding:"required"`
}
