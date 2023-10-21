package data

import (
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"greenlight.311102.xyz/internal/validator"
	"time"
)

// Movie 这里需要指出的是，Movie 结构中的所有字段都是导出的（即以大写字母开头），
// 这对于 Go 的编码/Json 软件包来说是必不可少的。在将结构编码为 JSON 时，不会包含任何未导出的字段。
// 用 struct 标记注释 Movie 结构，以控制键在 JSON 编码输出中的显示方式。
type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	RunTime   RunTime   `json:"run_time,omitempty"` // 播放时长 分钟单位 // 使用 Runtime 类型而不是 int32。请注意，"omitempty "指令仍然有效：如果 Runtime 字段的底层值为 0，那么它将被视为空字段并被省略--而我们刚刚创建的 MarshalJSON() 方法根本不会被调用。
	Genres    []string  `json:"genres,omitempty"`   // 播放时长 分钟单位
	Version   int32     `json:"version"`            // 版本号从 1 开始，每次更新电影信息时都会递增
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")

	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")

	v.Check(movie.RunTime != 0, "run_time", "must be provided")
	v.Check(movie.RunTime > 0, "run_time", "must be a positive integer")

	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

type MovieModel struct {
	DB *sql.DB
}

func (m MovieModel) Insert(movie *Movie) error {
	query := `
		INSERT INTO movies (title, year, run_time, genres) 
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`
	// 创建一个 args 片段，其中包含影片结构中占位符参数的值。在我们的 SQL 查询旁边声明这个片段，有助于清楚地说明在查询中使用了哪些值。
	// 在幕后，pq.Array() 适配器接收我们的[]字符串片段，并将其转换为 pq.StringArray 类型。反过来，pq.StringArray 类型实现了必要的 driver.Valuer 和 sql.Scanner 接口，以便将我们的本地 []string 片段转换成 PostgreSQL 数据库可以理解的值，并存储在 text[] 数组列中。
	// 您也可以在 Go 代码中以同样的方式使用 pq.Array() 适配器函数，包括 []bool, []byte, []int32, []int64, []float32 和 []float64 Slice
	args := []any{movie.Title, movie.Year, movie.RunTime, pq.Array(movie.Genres)}
	// 使用 QueryRow() 方法在连接池上执行 SQL 查询，将 args 片段作为变量参数传递，并将系统生成的 id、created_at 和版本值扫描到 movie 结构中。
	return m.DB.QueryRow(query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

func (m MovieModel) Get(id int64) (*Movie, error) {
	// 我们使用的 PostgreSQL bigserial 类型的电影 ID 默认从 1 开始自动递增，因此我们知道没有电影的 ID 值会小于 1。
	// 为了避免不必要的数据库调用，我们采取了一个快捷方式，直接返回 ErrRecordNotFound 错误信息
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	var movie Movie
	query := `
		SELECT id, created_at, title, year, run_time, genres, version
		FROM movies
		WHERE id=$1`
	err := m.DB.QueryRow(query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.RunTime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	// 处理任何错误。如果没有找到匹配的影片，Scan() 将返回 sql.ErrNoRows 错误。我们会对此进行检查，并返回我们自定义的 ErrRecordNotFound 错误。
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &movie, nil
}

func (m MovieModel) Update(movie *Movie) error {
	query := `
		UPDATE movies SET title=$1, year=$2, run_time=$3, genres=$4, version=version + 1
		WHERE id=$5 RETURNING version`
	args := []any{
		&movie.Title,
		&movie.Year,
		&movie.RunTime,
		pq.Array(&movie.Genres),
		&movie.ID,
	}
	return m.DB.QueryRow(query, args...).Scan(&movie.Version)
}

func (m MovieModel) Delete(id int64) error {
	return nil
}
