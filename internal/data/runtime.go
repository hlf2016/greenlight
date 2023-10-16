package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidRunTimeFormat 定义一个错误，当我们无法成功解析或转换 JSON 字符串时，UnmarshalJSON() 方法可以返回该错误。
var ErrInvalidRunTimeFormat = errors.New("invalid runtime format")

type RunTime int32

// MarshalJSON 在 Runtime 类型上实现 MarshalJSON() 方法，使其满足 json.Marshaler 接口。该方法将返回电影运行时的 JSON 编码值（在我们的例子中，它将返回格式为"<runtime> mins "的字符串）。
func (r RunTime) MarshalJSON() ([]byte, error) {
	jsonValue := fmt.Sprintf("%d mins", r)
	// 在字符串上使用 strconv.Quote() 函数，用双引号将其包住。要成为有效的 JSON 字符串，必须用双引号将其包围。
	quotedJSONValue := strconv.Quote(jsonValue)
	return []byte(quotedJSONValue), nil
}

// UnmarshalJSON 在 RunTime 类型上实现 UnmarshalJSON() 方法，使其满足 json.Unmarshaler 接口。重要：由于 UnmarshalJSON() 需要修改接收器（我们的 RunTime 类型），因此我们必须使用指针接收器才能正常工作。否则，我们只能修改一个副本（该副本会在此方法返回时被丢弃）。
// Go 在解码 JSON 时，会检查目标类型是否满足 json.Unmarshaler 接口。如果满足接口，Go 将调用 UnmarshalJSON() 方法来确定如何将提供的 JSON 解码为目标类型。这基本上是 json.Marshaler 接口的反向操作，我们之前使用该接口定制了 JSON 编码行为。
func (r *RunTime) UnmarshalJSON(jsonValue []byte) error {
	// 我们预计传入的 JSON 值将是一个格式为"<runtime> mins "的字符串，我们首先要做的就是从这个字符串中移除周围的双引号。如果无法取消引号，我们就会返回 ErrInvalidRuntimeFormat 错误。
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRunTimeFormat
	}
	// 分割字符串，分离出包含数字的部分。
	parts := strings.Split(unquotedJSONValue, " ")
	// 对字符串的各部分进行理智检查，确保其符合预期格式。如果不是，我们将再次返回 ErrInvalidRuntimeFormat 错误。
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRunTimeFormat
	}
	// 否则，将包含数字的字符串解析为 int32。同样，如果解析失败，将返回 ErrInvalidRuntimeFormat 错误。
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRunTimeFormat
	}
	// 将 int32 转换为 RunTime 类型，并将其赋值给接收器。
	// 请注意，我们使用运算符来替换接收器（接收器是指向运行时类型的指针），以便设置指针的底层值。
	// 直接替换 *r 这个内存地址对应的值为 RunTime(i)
	*r = RunTime(i)
	return nil
}
