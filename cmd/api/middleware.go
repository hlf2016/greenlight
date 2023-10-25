package main

import (
	"fmt"
	"golang.org/x/time/rate"
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

func (app *application) rateLimit(next http.Handler) http.Handler {
	// 初始化一个新的速率限制器，允许平均每秒 2 个请求，单个 "突发 "最多 4 个请求。
	limiter := rate.NewLimiter(2, 4)
	// 我们返回的函数是一个闭包，它 "关闭 "了限制器变量。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 调用 limiter.Allow() 查看请求是否允许，如果不允许，则调用 rateLimitExceededResponse() 辅助程序返回 429 太多请求响应（我们稍后将创建该辅助程序）。
		// 每当我们调用速率限制器上的 Allow() 方法时，就会从邮筒中消耗一个令牌。如果桶中没有剩余的令牌，Allow() 方法将返回 false，并触发向客户端发送 429 太多请求的响应。
		// 还需要注意的是，Allow() 方法后面的代码受互斥保护，可以安全并发使用。
		if !limiter.Allow() {
			app.rateLimitExceededResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
