package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
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
		dsn string
	}
}

type application struct {
	config config
	logger *log.Logger
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 4000, "Api server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	//  pq: SSL is not enabled on the server 需要在 dsn 禁用 ssl
	flag.StringVar(&cfg.db.dsn, "db-dsn", "postgres://greenlight:25804769@localhost/greenlight?sslmode=disable", "PostgreSQL DSN")
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

	app := &application{
		config: cfg,
		logger: logger,
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
