package main

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

// 更新 routes() 方法，使其返回 http.Handler 而不是 httprouter.Router
// httprouter.Router实现了http.Handler接口 ServeHTTP
func (app *application) routes() http.Handler {
	router := httprouter.New()
	// 使用 http.HandlerFunc() 适配器将 notFoundResponse() 辅助程序转换为 http.Handler 程序，然后将其设置为 404 Not Found 响应的自定义错误处理程序。
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	// 同样，将 methodNotAllowedResponse() 转换为 http.Handler，并将其设置为 405 Method Not Allowed 响应的自定义错误处理程序。
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

	router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))
	router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))

	router.HandlerFunc(http.MethodPost, "/v1/users", app.registerUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateUserHandler)

	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationHandler)

	// 用 panic 恢复中间件包裹路由器。
	return app.recoverPanic(app.rateLimit(app.authenticate(router)))
}
