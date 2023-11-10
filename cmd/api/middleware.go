package main

import (
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"greenlight.311102.xyz/internal/data"
	"greenlight.311102.xyz/internal/validator"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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
	// 定义一个client结构，用于保存每个客户端的速率限制器和最后查看时间。
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	// 声明一个 mutex 和一个 map，用于保存客户端的 IP 地址和速率限制器。
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// 启动后台程序，每分钟从客户端地图中删除一次旧条目。
	go func() {
		for {
			time.Sleep(time.Minute)
			// 开启互斥锁，防止在清理过程中进行任何速率限制器检查。
			mu.Lock()
			// 循环浏览所有客户端。如果在过去三分钟内没有看到它们，就从地图上删除相应的条目。
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			// 重要的是，清理完成后要解锁互斥。
			mu.Unlock()
		}
	}()
	// 初始化一个新的速率限制器，允许平均每秒 2 个请求，单个 "突发 "最多 4 个请求。
	// limiter := rate.NewLimiter(2, 4)

	// 我们返回的函数是一个闭包，它 "关闭 "了限制器变量。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.config.limiter.enabled {
			// 从请求中提取客户端的 IP 地址。
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

			// 上互斥锁，以防止这段代码被同时执行。
			mu.Lock()

			// 检查 IP 地址是否已存在于地图中。如果不存在，则初始化一个新的速率限制器，并将 IP 地址和限制器添加到映射中。
			if _, found := clients[ip]; !found {
				clients[ip] = &client{limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}
			}
			clients[ip].lastSeen = time.Now()

			// 调用 limiter.Allow() 查看请求是否允许，如果不允许，则调用 rateLimitExceededResponse() 辅助程序返回 429 太多请求响应（我们稍后将创建该辅助程序）。
			// 每当我们调用速率限制器上的 Allow() 方法时，就会从邮筒中消耗一个令牌。如果桶中没有剩余的令牌，Allow() 方法将返回 false，并触发向客户端发送 429 太多请求的响应。
			// 还需要注意的是，Allow() 方法后面的代码受互斥保护，可以安全并发使用。
			//if !limiter.Allow() {
			//	app.rateLimitExceededResponse(w, r)
			//	return
			//}

			// 调用当前 IP 地址的速率限制器上的 Allow() 方法。如果请求不被允许，则解锁互斥器 并发送 429 太多请求响应，就像之前一样。
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			// 最重要的是，在调用链中的下一个处理程序之前，要先解开互斥锁。
			// 请注意，我们不能使用 defer 来解锁互斥，因为这意味着在该中间件下游的所有处理程序都返回之前，互斥不会被解锁。
			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 将 "Vary：授权 "标头。这将向任何缓存表明，响应可能会根据请求中的 "授权"(Authorization) 标头的值而有所不同。
		w.Header().Add("Vary", "Authorization")
		// 从请求中读取授权标头的值。如果找不到授权标头，将返回空字符串""。
		authorizationHeader := r.Header.Get("Authorization")

		// 如果没有找到授权头，则使用我们刚刚制作的 contextSetUser() 辅助函数将匿名用户添加到请求上下文中。然后，我们调用链中的下一个处理程序并返回，不执行下面的任何代码。
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// 否则，我们希望授权头的值格式为 "Bearer <token>"。我们会尝试将其拆分成几个部分，如果头信息不符合预期格式，
		// 我们就会使用 invalidAuthenticationTokenResponse() 辅助程序（稍后将创建）返回 401 Unauthorized 响应。
		authParts := strings.Split(authorizationHeader, " ")
		if len(authParts) != 2 || authParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// 从标头部分提取实际的身份验证令牌。
		token := authParts[1]
		// 验证令牌，确保其格式合理。
		v := validator.New()
		// 如果令牌无效，则使用 invalidAuthenticationTokenResponse() 辅助程序发送响应，
		// 而不是我们通常使用的 failedValidationResponse() 辅助程序。
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// 读取与身份验证令牌关联的用户的详细信息，如果没有找到匹配记录，则再次调用 invalidAuthenticationTokenResponse() 辅助程序。
		// 重要：请注意，我们在这里使用 ScopeAuthentication 作为第一个参数。
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		// 调用 contextSetUser() 辅助函数将用户信息添加到请求上下文中。
		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

// 请注意，我们的 requireAuthenticatedUser requireActivatedUser() 中间件的签名与我们在本书中构建的其他中间件略有不同。它不是接受并返回一个 http.Handler，而是接受并返回一个 http.HandlerFunc。
// 这只是一个很小的改动，但它使得我们可以直接用这个中间件来封装我们的 v1/movie** 处理程序函数，而无需进行任何进一步的转换。
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)

	})
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
	// 在返回 fn 之前，用 requireAuthenticatedUser() 中间件对其进行封装。
	return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	return app.requireActivatedUser(fn)
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 告知客户端 响应根据Origin值不同会有变化
		w.Header().Set("Vary", "Origin")

		origin := r.Header.Get("Origin")

		// w.Header().Set("Access-Control-Allow-Origin", "*")

		if origin != "" {
			// 在受信任的来源列表中循环，检查请求的来源是否与其中之一完全匹配。如果没有受信任的来源地，则不会迭代循环
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					// 如果匹配，则设置一个以请求来源为值的 "Access-Control-Allow-Origin "响应头，然后跳出循环。
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
