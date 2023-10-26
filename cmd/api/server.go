package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func (app *application) serve() error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.config.port),
		Handler: app.routes(),
		// 遗憾的是，我们无法直接将 http.Server 设置为使用新的日志记录器类型。相反，你需要利用我们的日志记录器满足 io.Writer 接口这一事实（多亏了我们为它添加的 Write() 方法），并将 http.Server 设置为使用标准库中的常规 log.Logger 实例，该实例将我们自己的日志记录器作为目标目标写入。
		// 使用 log.New() 函数创建一个新的 Go log.Logger 实例，并将我们的自定义 Logger 作为第一个参数传递进去。""和 0 表示 log.Logger 实例不应使用前缀或任何标志。
		ErrorLog:     log.New(app.logger, "", 0),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})
	return srv.ListenAndServe()
}
