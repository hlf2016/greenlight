package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"greenlight.311102.xyz/internal/data"
	"greenlight.311102.xyz/internal/jsonlog"
	"greenlight.311102.xyz/internal/mailer"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	// 添加一个新的限制器结构，其中包含每秒请求数和突发值字段，以及一个布尔字段，我们可以用它来启用/禁用全部速率限制。
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *jsonlog.Logger // 将日志记录器字段改为 *jsonlog.Logger 类型，而不是 log.Logger。
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup // 在应用程序结构体中加入 sync.WaitGroup。sync.WaitGroup 类型的零值是一个有效的、可使用的、"计数器 "值为 0 的 sync.WaitGroup，因此我们在使用它之前不需要做任何其他初始化操作。
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 4000, "Api server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// 若报错 pq: SSL is not enabled on the server 需要在 dsn 禁用 ssl
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	// 将连接池设置从命令行标志读入配置结构。注意到我们使用的默认值了吗？
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	// 创建 limiter 配置 命令行标志，将设置值读入配置结构。注意到 "enabled" 设置的默认值是 true 吗？
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	// 使用 Mailtrap 设置作为默认值，将 SMTP 服务器配置设置读入 config 结构
	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "1e2b246389e036", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "e5bba5ecf0f339", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.alexedwards.net>", "SMTP sender")

	// 使用 flag.Func() 函数处理 -cors-trusted-origins 命令行标志。
	// 在此过程中，我们使用 strings.Fields() 函数根据空白字符将标志值分割成片段，并将其赋值给配置结构。
	// 重要的是，如果 -cors-trusted-origins 标记不存在、包含空字符串或仅包含空白字符，那么 strings.Fields() 将返回一个空的[]字符串片段。
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	// 如果 displayVersiondisplay值为 true，则打印出版本号并立即退出。
	// 没有值的布尔命令行标志被解释为值为 true。因此，使用 -version 运行应用程序与使用 -version=true 运行应用程序是一样的。
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	// 初始化一个新的日志记录器，将信息写入标准输出流，并以当前日期和时间为前缀。
	// logger := log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime)
	// 初始化一个新的 jsonlog.Logger 日志记录器，该日志记录器会将任何严重程度达到或超过 INFO 的信息写入标准输出流。
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	// 调用 openDB() 辅助函数（见下文）来创建连接池，并传入配置结构。如果返回错误，我们会记录并立即退出应用程序
	db, err := openDB(cfg)
	if err != nil {
		// 使用 PrintFatal() 方法写入包含 FATAL 级别错误的日志条目并退出。我们没有其他属性要包含在日志条目中，因此我们将 nil 作为第二个参数传递。
		logger.PrintFatal(err, nil)
	}

	// 延迟调用 db.Close()，以便在 main() 函数退出之前关闭连接池。
	defer db.Close()

	logger.PrintInfo("database connection pool established", nil)

	// 在 expvar 处理程序中发布一个新的 "版本 "变量，其中包含应用程序的版本号（目前为常量 "1.0.0"）。
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("db", expvar.Func(func() any {
		return db.Stats()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
	// 使用 data.NewModels() 函数初始化一个 Models 结构，并将连接池作为参数传递。
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

// openDB() 函数返回一个 sql.DB 连接池
func openDB(cfg config) (*sql.DB, error) {
	// 使用 sql.Open()，使用配置结构中的 DSN 创建一个空连接池。
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// 设置池中打开（使用中 + 空闲）连接的最大数量。请注意，如果传递的值小于或等于 0，则表示没有限制。
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	// 设置池中空闲连接的最大数量。同样，如果传递的值小于或等于 0，则表示没有限制。
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	// 使用 time.ParseDuration() 函数将空闲超时持续时间字符串转换为 time.Duration 类型。
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	// 设置最大空闲超时。
	db.SetConnMaxIdleTime(duration)

	if err != nil {
		return nil, err
	}
	//	创建具有 5 秒超时期限的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用 PingContext() 与数据库建立新连接，并将上文创建的上下文作为参数传入。如果连接无法在 5 秒期限内成功建立，则会返回错误信息。
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}
