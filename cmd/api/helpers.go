package main

import (
	"errors"
	"github.com/julienschmidt/httprouter"
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
