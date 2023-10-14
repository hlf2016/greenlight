package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"strconv"
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
	err := json.NewDecoder(r.Body).Decode(&dst)
	if err != nil {
		// 如果有错误发生 开始分类处理
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

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

		//	如果我们向 Decode() 传递的内容不是非零指针，就会返回 json.InvalidUnmarshalError 错误。我们会捕捉到这个错误并 panic，而不是向处理程序返回错误。
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}
	return nil
}
