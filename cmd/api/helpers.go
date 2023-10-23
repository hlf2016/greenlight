package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"greenlight.311102.xyz/internal/validator"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (app *application) readIDParam(r *http.Request) (int64, error) {
	// 当 httprouter 解析请求时，任何插值的 URL 参数都将存储在请求上下文中。我们可以使用 ParamsFromContext() 函数检索包含这些参数名称和值的片段。
	params := httprouter.ParamsFromContext(r.Context())

	// 然后，我们可以使用 ByName() 方法从片段中获取 "id "参数的值。在我们的项目中，所有影片都有一个唯一的正整数 ID，但 ByName() 返回的值始终是字符串。
	// 因此，我们尝试将其转换为以 10 为底的整数（位大小为 64）。如果参数无法转换或小于 1，我们就知道 ID 无效，因此我们使用 http.NotFound() 函数返回 404 Not Found 响应。
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// js, err := json.Marshal(data)

	// 使用 json.MarshalIndent() 函数将 空格 添加到编码后的 JSON 中。在这里，我们对每个元素都不使用行前缀（""）和制表符缩进（"\t"）
	// 方便在 命令行 请求时 查看响应json结构明确 同时 比 json.Marshal 性能要差 30%
	// 在幕后，json.MarshalIndent() 通过正常调用 json.Marshal()，然后通过独立的 json.Indent() 函数运行 JSON 来添加空白。还有一个反向函数 json.Compact()，可以用来删除 JSON 中的空白。
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	// 添加换行符，以便于在终端应用程序中查看。
	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	//  使用 http.MaxBytesReader() 将请求正文的大小限制为 1MB。
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	// 初始化 json.Decoder 并在解码前调用 DisallowUnknownFields() 方法。这意味着，如果现在来自客户端的 JSON 包含任何无法映射到目标目的地的字段，解码器将返回错误信息，而不是直接忽略该字段。
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// 解码请求正文 dst 本身传入的就是指针
	err := dec.Decode(dst)
	if err != nil {
		// 如果有错误发生 开始分类处理
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		// 使用 errors.As() 函数检查错误是否属于 json.SyntaxError 类型。如果是，则返回包含问题位置的普通英文错误信息。
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		//	在某些情况下，如果 JSON 中出现语法错误，Decode() 还可能返回 io.ErrUnexpectedEOF 错误。因此，我们使用 errors.Is() 来检查这种情况，并返回一条通用的错误信息。关于这个问题，https://github.com/golang/go/issues/25956。
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		// 同样，捕捉任何 json.UnmarshalTypeError 错误。当 JSON 值的类型不适合目标目的地时，就会出现这种错误。如果错误与特定字段有关，我们就会在错误信息中包含该字段，以方便客户端调试。
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		//	如果请求正文为空，Decode() 将返回 io.EOF 错误。我们使用 errors.Is() 来检查这种情况，并返回一条纯英文的错误信息。
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// 如果 JSON 包含一个无法映射到目标目的地的字段，那么 Decode() 将返回格式为 "json: unknown field "<name>"（json：未知字段"<name>"）的错误信息。我们将对此进行检查，从错误信息中提取字段名称，并将其插入我们的自定义错误信息中。请注意，https://github.com/golang/go/issues/29035 上还有一个关于将来将其转化为独立错误类型的开放问题。
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains uknown key %s", fieldName)

		// 使用 errors.As() 函数检查错误是否属于 http.MaxBytesError 类型。如果是，则表示请求体超过了 1MB 的大小限制，我们将返回一条明确的错误信息。
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		//	如果我们向 Decode() 传递的内容不是非零指针，就会返回 json.InvalidUnmarshalError 错误。我们会捕捉到这个错误并 panic，而不是向处理程序返回错误。
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}
	// 再次调用 Decode()，将指向空匿名结构体的指针作为目标。如果请求正文只包含一个 JSON 值，就会返回一个 io.EOF 错误。因此，如果我们得到其他信息，我们就会知道请求体中还有其他数据，并返回我们自己的自定义错误信息。
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

// readString() helper 会从查询字符串中返回一个字符串值，如果找不到匹配的键，则返回所提供的默认值。
func (app *application) readString(qs url.Values, key, defaultValue string) string {
	// 从查询字符串中提取给定键的值。如果不存在键，将返回空字符串""。
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	return s
}

// readCSV() helper 从查询字符串中读取字符串值，然后以逗号字符为分割点。如果找不到匹配的键，则返回所提供的默认值。
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)
	if csv == "" {
		return defaultValue
	}
	return strings.Split(csv, ",")
}

// readInt() helper 从查询字符串中读取字符串值，并在返回前将其转换为整数。如果找不到匹配的键，则返回所提供的默认值。如果无法将值转换为整数，则会在提供的 Validator 实例中记录一条错误信息。
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	// 尝试将数值转换为 int。如果失败，则在验证器实例中添加错误信息，并返回默认值
	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}
	return i
}
