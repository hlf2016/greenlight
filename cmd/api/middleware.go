package main

import (
	"errors"
	"expvar"
	"fmt"
	"golang.org/x/time/rate"
	"greenlight.311102.xyz/internal/data"
	"greenlight.311102.xyz/internal/validator"
	"net"
	"net/http"
	"strconv"
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
		// 告知请求客户端 响应根据Origin值不同会有变化
		w.Header().Add("Vary", "Origin")
		// 告知请求客户端 响应根据Access-Control-Request-Method值不同会有变化
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		// w.Header().Set("Access-Control-Allow-Origin", "*")

		if origin != "" {
			// 在受信任的来源列表中循环，检查请求的来源是否与其中之一完全匹配。如果没有受信任的来源地，则不会迭代循环
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					// 如果匹配，则设置一个以请求来源为值的 "Access-Control-Allow-Origin "响应头，然后跳出循环。
					w.Header().Set("Access-Control-Allow-Origin", origin)
					// 检查请求是否具有 HTTP 方法 OPTIONS 并包含 "Access-Control-Request-Method"（访问控制请求方法）标头。如果是，我们就将其视为预检请求。
					// 响应预检请求时，无需在 Access-Control-Allow-Methods 头信息中包含 CORS 安全方法 HEAD、GET 或 POST。同样，也没有必要在 Access-Control-Allow-Headers 中包含禁止或 CORS 安全标头。
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						// 如前所述，设置必要的预检响应标头
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")

						// 如果允许在跨起源请求中使用 "授权"(Authorization) 标头，就像我们在上面的代码中所做的那样，那么重要的是不要设置通配符 "Access-Control-Allow-Origin: *"标头，也不要在未与受信任的起源列表进行核对的情况下反映起源标头。否则，您的服务就很容易受到针对该标头中传递的任何身份验证凭据的分布式暴力破解攻击。
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

						// 写入标头和 200 OK 状态，然后从中间件返回，不做进一步操作
						// 在响应预检请求时，我们会特意发送 HTTP 状态 200 OK，而不是 204 No Content，即使没有响应正文。这是因为某些浏览器版本可能不支持 204 No Content 响应，因此会阻止真正的请求。
						w.WriteHeader(http.StatusOK)
						return
					}
					break
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// 实现自己的 http.ResponseWriter

// metricsResponseWriter 封装了现有的 http.ResponseWriter，还包含一个用于记录响应状态代码的字段和一个布尔标志，用于指示是否已写入响应头。
// 重要的是，我们的 metricsResponseWriter 类型满足 http.ResponseWriter 接口的要求。它拥有具有相应签名的 Header()、WriteHeader() 和 Write() 方法，因此我们可以在处理程序中正常使用它。
type metricsResponseWriter struct {
	wrapped       http.ResponseWriter
	statusCode    int
	headerWritten bool
}

// Header 方法是对封装后的 http.ResponseWriter 的 Header() 方法的简单 "传递"。
func (mw *metricsResponseWriter) Header() http.Header {
	return mw.wrapped.Header()
}

// WriteHeader 同样，WriteHeader() 方法会 "穿过 "封装的 http.ResponseWriter 的 WriteHeader() 方法。但在返回后，我们还会记录响应状态代码（如果尚未记录的话）。
func (mw *metricsResponseWriter) WriteHeader(statusCode int) {
	mw.wrapped.WriteHeader(statusCode)
	// 此外，请注意我们在 WriteHeader() 方法中的 "直通 "调用之后才记录状态代码。
	// 这是因为该操作中的恐慌（可能是由于状态代码无效）可能意味着最终会向客户端发送不同的状态代码。
	if !mw.headerWritten {
		mw.statusCode = statusCode
		mw.headerWritten = true
	}
}

// Write 同样，Write() 方法会 "传递 "到封装的 http.ResponseWriter 的 Write() 方法。
// 如果没有单独成功调用 WriteHeader()，我们知道 Go 将默认使用响应状态 200 OK，所以我们记录了以下内容
func (mw *metricsResponseWriter) Write(b []byte) (int, error) {
	if !mw.headerWritten {
		mw.statusCode = http.StatusOK
		mw.headerWritten = true
	}
	return mw.wrapped.Write(b)
}

// Unwrap  我们还需要一个 Unwrap() 方法，用于返回现有的封装 http.ResponseWriter
func (mw *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return mw.wrapped
}

func (app *application) metrics(next http.Handler) http.Handler {
	// 在首次构建中间件链时初始化新的 expvar 变量。
	var (
		totalRequestsReceived           = expvar.NewInt("total_requests_received")
		totalResponsesSent              = expvar.NewInt("total_responses_sent")
		totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
		totalActiveRequests             = expvar.NewInt("total_active_requests")

		// 声明一个新的 expvar 映射，用于保存每个 HTTP 状态代码的响应计数。
		totalResponsesSentByStatus = expvar.NewMap("total_responses_sent_by_status")
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 记录我们开始处理申请的时间。
		start := time.Now()
		// 使用 Add() 方法将收到的请求数增加 1。
		totalRequestsReceived.Add(1)

		totalActiveRequests.Set(totalRequestsReceived.Value() - totalResponsesSent.Value())

		// 创建一个新的 metricsResponseWriter，它封装了metrics中间件收到的原始 http.ResponseWriter 值。
		mw := &metricsResponseWriter{wrapped: w}
		// 使用新的 metricsResponseWriter 作为 http.ResponseWriter 值，调用链中的下一个处理程序。x
		next.ServeHTTP(mw, r)

		// 在返回中间件链的途中，将发送的响应数递增 1
		totalResponsesSent.Add(1)

		// 此时，响应状态代码应存储在 mw.statusCode 字段中。
		// 请注意，expvar 映射是以字符串为键的，因此我们需要使用 strconv.Itoa() 函数将状态代码（整数）转换为字符串。
		// 然后，我们在新的 totalResponsesSentByStatus 映射上使用 Add() 方法，将给定状态代码的计数递增 1。
		totalResponsesSentByStatus.Add(strconv.Itoa(mw.statusCode), 1)

		// 计算我们开始处理请求后的微秒数，然后将总处理时间按此数递增。
		duration := time.Since(start).Microseconds()
		totalProcessingTimeMicroseconds.Add(duration)

		// 在 /v1/metrics 下看到的 json 数据中 totalResponsesSent 总比 totalRequestsReceived 至少小 1
		// 这是因为 totalRequestsReceived.Add 是在 返回 json 之前被访问到  而 totalResponsesSent.Add 则总是在 json 返回之后才被访问到
	})
}
