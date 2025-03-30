package pkg

import "strings"

type ResponseBodyInterface interface {
	IsSuccess() bool
	IsFailed() bool
}

type ResponseBodyMetaInterface interface {
	GetMetaType() string
}

type ResponseBodyMetaPagination struct {
	Total int64 `json:"total"`
	Count int64 `json:"count"`
}

func (p *ResponseBodyMetaPagination) GetMetaType() string {
	return "pagination"
}

type ResponseBody struct {
	Result bool                                 `json:"result"`
	Data   interface{}                          `json:"data"`
	Meta   map[string]ResponseBodyMetaInterface `json:"meta"`
	Error  string                               `json:"error"`
}

func NewResponseBody(data interface{}) *ResponseBody {
	return &ResponseBody{Result: true, Data: data}
}

func NewResponseBodyError(error error) ResponseBody {
	return ResponseBody{Result: false, Error: convertErrorMessage(error.Error())}
}

func NewResponseBodyWithPagination(data interface{}, total int64, count int) *ResponseBody {
	meta := &ResponseBodyMetaPagination{Total: total, Count: int64(count)}
	rb := NewResponseBody(data)
	rb.Meta = map[string]ResponseBodyMetaInterface{meta.GetMetaType(): meta}
	return rb
}

func convertErrorMessage(msg string) string {
	var result string
	msgArr := strings.Split(msg, ":")
	result = strings.Trim(msgArr[len(msgArr)-1], " ")
	if strings.Contains(result, "SQLSTATE") {
		result = "Database error"
	}
	return result
}
