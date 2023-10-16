package data

import (
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
