package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"greenlight.311102.xyz/internal/data"
	"log"
	"net/http"
	"os"
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
}

type application struct {
	config config
	logger *log.Logger
	models data.Models
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 4000, "Api server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// 若报错 pq: SSL is not enabled on the server 需要在 dsn 禁用 ssl
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

	// 将连接池设置从命令行标志读入配置结构。注意到我们使用的默认值了吗？
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	flag.Parse()

	// 初始化一个新的日志记录器，将信息写入标准输出流，并以当前日期和时间为前缀。
	logger := log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime)

	// 调用 openDB() 辅助函数（见下文）来创建连接池，并传入配置结构。如果返回错误，我们会记录并立即退出应用程序
	db, err := openDB(cfg)
	if err != nil {
		logger.Fatal(err)
	}

	// 延迟调用 db.Close()，以便在 main() 函数退出之前关闭连接池。
	defer db.Close()

	logger.Printf("database connection pool established")

	// 使用 data.NewModels() 函数初始化一个 Models 结构，并将连接池作为参数传递。
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("Starting %s server on %d", cfg.env, cfg.port)
	err = srv.ListenAndServe()
	logger.Fatal(err)
}

// openDB() 函数返回一个 sql.DB 连接池
func openDB(cfg config) (*sql.DB, error) {
	// 使用 sql.Open()，使用配置结构中的 DSN 创建一个空连接池。
	db, err := sql.Open("postgres", cfg.db.dsn)

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
