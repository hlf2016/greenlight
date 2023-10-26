package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	// 启动后台程序
	go func() {
		// 我们在这里需要使用缓冲通道，因为 signal.Notify() 在向 quit 通道发送信号时不会等待接收器可用。
		// 如果我们在这里使用的是普通（非缓冲）通道，那么在发送信号时，如果我们的 quit 通道尚未准备好接收信号，那么信号就可能被 "错过"。
		// 通过使用缓冲通道，我们避免了这个问题，并确保不会错过信号。
		// 创建一个可传输 os.Signal 值的 quit 通道。
		quit := make(chan os.Signal, 1)

		// 使用signal.Notify()监听传入的SIGINT和SIGTERM信号，并将它们转发到退出通道。
		// 任何其他信号将不会被signal.Notify()捕获，并将保留其默认行为。
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// 从 quit 通道读取信号。该代码将阻塞，直到收到信号。
		s := <-quit

		// 记录一条信息，说明信号已被捕获。请注意，我们还调用了信号的 String() 方法来获取信号名称，并将其包含在日志条目属性中。
		app.logger.PrintInfo("caught signal", map[string]string{
			"signal": s.String(),
		})

		// 以 0（成功）状态码退出应用程序。
		os.Exit(0)
	}()

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})
	return srv.ListenAndServe()
}
