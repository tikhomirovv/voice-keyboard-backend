package pkg

import "strings"

type ResponseBodyInterface interface {
	IsSuccess() bool
	IsFailed() bool
}

type ResponseBodyMetaInterface interface {
	GetMetaType() string
}

type ResponseBody struct {
	Result bool                                 `json:"result"`
	Data   any                                  `json:"data"`
	Meta   map[string]ResponseBodyMetaInterface `json:"meta"`
	Error  string                               `json:"error"`
}

func NewResponseBody(data any) *ResponseBody {
	return &ResponseBody{Result: true, Data: data}
}

func NewResponseBodyError(error error) ResponseBody {
	return ResponseBody{Result: false, Error: convertErrorMessage(error.Error())}
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
