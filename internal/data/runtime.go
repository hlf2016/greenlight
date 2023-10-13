package data

import (
	"fmt"
	"strconv"
)

type RunTime int32

// MarshalJSON 在 Runtime 类型上实现 MarshalJSON() 方法，使其满足 json.Marshaler 接口。该方法将返回电影运行时的 JSON 编码值（在我们的例子中，它将返回格式为"<runtime> mins "的字符串）。
func (r RunTime) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d mins", r)
	// 在字符串上使用 strconv.Quote() 函数，用双引号将其包住。要成为有效的 JSON 字符串，必须用双引号将其包围。
	quotedJSONValue := strconv.Quote(jsonValue)
	return []byte(quotedJSONValue), nil
}
