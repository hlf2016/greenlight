package main

import (
	"fmt"
	"net/http"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 创建一个defer 函数（在出现 panic 时，Go 会释放堆栈，并始终运行该函数）。
		defer func() {
			// 使用内置的 recover 功能检查是否发生 panic。
			if err := recover(); err != nil {
				// 如果出现 panic，则在响应上设置 "Connection: close"（连接：关闭）标头。
				// 这将作为一个触发器，使 Go 的 HTTP 服务器在发送响应后自动关闭当前连接。
				w.Header().Set("Connection", "close")
				// recover() 返回的值类型为 any，因此我们使用 fmt.Errorf() 将其规范化为错误，并调用 serverErrorResponse() 辅助程序。
				// 反过来，这将使用ERROR 级别的自定义日志记录器类型记录错误，并向客户端发送 500 内部服务器错误响应。
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
