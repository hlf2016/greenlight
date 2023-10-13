package data

import "time"

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
