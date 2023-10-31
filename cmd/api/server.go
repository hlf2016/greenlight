package main

import (
	"context"
	"errors"
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

	// 创建 shutdownError 频道。我们将用它来接收优雅关闭（）函数返回的任何错误。
	shutdownError := make(chan error)

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
		// 更新日志条目，将 "caught signal" 改为 "shutting down server"。
		app.logger.PrintInfo("shutting down server", map[string]string{
			"signal": s.String(),
		})

		// 创建一个超时 20 秒的上下文。
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// 在服务器上调用 Shutdown()，并传入我们刚刚创建的上下文。
		// 如果优雅关机成功，Shutdown() 将返回 nil，否则将返回错误（可能是因为关闭监听器出现问题，或者因为在 20 秒的上下文截止时间到来之前关机没有完成）。
		// 我们会将此返回值转发到 shutdownError 频道。

		// 以 0（成功）状态码退出应用程序。
		// os.Exit(0)
		// 将 shutdown 操作成功与否的信息放入 shutdownError channel 中
		// shutdownError <- srv.Shutdown(ctx)

		// 像以前一样在服务器上调用 Shutdown()，但现在只有在返回错误时才会发送到 shutdownError 频道。
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}
		// 记录一条信息，说明我们正在等待后台程序完成任务。
		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})

		// 调用 Wait() 进行阻塞，直到我们的 WaitGroup 计数器为零 -- 本质上就是阻塞，直到后台程序结束。然后，我们在 shutdownError 频道上返回 nil，表示关机顺利完成
		app.wg.Wait()
		shutdownError <- nil
	}()

	app.logger.PrintInfo("starting server", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})

	// 在服务器上调用 Shutdown() 会导致 ListenAndServe() 立即返回 http.ErrServerClosed 错误。
	// 因此，如果我们看到这个错误，其实是件好事，表明优雅关机已经开始。
	// 因此，我们专门为此进行了检查，只有在不是 http.ErrServerClosed 时才返回错误信息。
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// 否则，我们将在 shutdownError 频道上等待接收 Shutdown() 的返回值。如果返回值是错误，我们就知道优雅关机出现了问题，并返回错误信息。
	err = <-shutdownError
	if err != nil {
		return err
	}

	// 此时，我们知道优雅关机已成功完成，并记录了一条 "stopped server "信息。
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}
