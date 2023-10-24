package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 使用 QueryRow() 方法在连接池上执行 SQL 查询，将 args 片段作为变量参数传递，并将系统生成的 id、created_at 和版本值扫描到 movie 结构中。
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

func (m MovieModel) Get(id int64) (*Movie, error) {
	// 我们使用的 PostgreSQL bigserial 类型的电影 ID 默认从 1 开始自动递增，因此我们知道没有电影的 ID 值会小于 1。
	// 为了避免不必要的数据库调用，我们采取了一个快捷方式，直接返回 ErrRecordNotFound 错误信息
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	var movie Movie
	// 更新查询，将 pg_sleep(10) 作为第一个值返回。
	// 模拟长时间查询
	/*query := `
	SELECT pg_sleep(10), id, created_at, title, year, run_time, genres, version
	FROM movies
	WHERE id=$1`*/
	query := `
		SELECT id, created_at, title, year, run_time, genres, version
		FROM movies
		WHERE id=$1`

	// 使用 context.WithTimeout() 函数创建一个 context.Context，其超时期限为 3 秒。
	// 请注意，我们使用空的 context.Background() 作为 "父 "上下文。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	// 重要的是，使用 defer 可以确保我们在 Get() 方法返回之前取消上下文。
	defer cancel()

	// 重要的是，更新 Scan() 参数，以便将 pg_sleep(10) 返回值扫描为 []byte 片段。
	// 使用 QueryRowContext() 方法执行查询，将带有截止日期的上下文作为第一个参数传递。
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		//&[]byte{}, // 测试用 对应上面的 pg_sleep() 注释掉
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
		WHERE id=$5 AND version = $6 RETURNING version`
	args := []any{
		movie.Title,
		movie.Year,
		movie.RunTime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m MovieModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `DELETE FROM movies WHERE id=$1`

	result, err := m.DB.Exec(query, id)

	if err != nil {
		return err
	}

	rowAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetAll 创建一个新的 GetAll() 方法，用于返回 movies Slice。虽然我们现在没有使用它们，但我们已将其设置为接受各种过滤器参数作为参数。
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
	// 支持全文搜索
	// to_tsvector('simple', title) 函数接收一个电影标题并将其拆分成词目。我们指定的是 simple 配置，这意味着词目只是标题中单词的小写版本。
	// 例如，电影标题 "The Breakfast Club（早餐俱乐部）"将被分割成词素 "breakfast""club""the"。
	// 其他 "non-simple" 配置可能会对词目应用额外的规则，如删除常用词或应用特定语言的词干。
	// plainto_tsquery('simple', $1) 函数接收搜索值，并将其转化为 PostgreSQL 全文搜索可以理解的格式化查询词。它对†搜索值进行规范化处理（再次使用简单配置），去掉所有特殊字符，并在单词之间插入和运算符 &。例如，搜索值 "The Club"的结果就是查询词 "the " & "club"。

	query := fmt.Sprintf(`
		SELECT count(*) OVER(), id, created_at, title, year, run_time, genres, version
		FROM movies
		WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
		AND (genres @> $2 OR $2 = '{}')
		ORDER BY %s %s, id ASC
		LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{title, pq.Array(genres), filters.limit(), filters.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}

	// 重要的是，推迟调用 rows.Close()，以确保在 GetAll() 返回之前关闭结果集。
	defer rows.Close()

	totalRecords := 0
	// 此处是将 movies 声明为 空的 *Movie slice 如果下面不存在数据 则最终响应结果为 []
	movies := []*Movie{}
	// 如果 采用下面这种方式 声明为 slice 的默认值 nil 下面不存在数据时 那么最终响应结果为 null
	// var movie []*Movie

	for rows.Next() {
		var movie Movie
		// 将该行的值扫描到 "Movie"结构中。请再次注意，我们在 genres 字段上使用了 pq.Array() 适配器。
		err := rows.Scan(
			&totalRecords,
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.RunTime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		movies = append(movies, &movie)
	}
	// 当 rows.Next() 循环结束后，调用 rows.Err() 来获取迭代过程中遇到的任何错误。
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return movies, metadata, nil
}
