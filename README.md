## json 编码

###  encoding/json
```go
js, err := json.Marshal(data)
```
| Go type                                            | ⇒  |                  JSON type |
|:---------------------------------------------------|:--:|---------------------------:|
| bool                                               | ⇒  |               JSON boolean |
| string                                             | ⇒  |                JSON string |
| int*, uint*, float*, rune                          | ⇒  |                JSON number |
| array, slice                                       | ⇒  |                JSON array  |
| struct, map                                        | ⇒  |                JSON object |
| nil pointers, interface values, slices, maps, etc. | ⇒  |                  JSON null |
| chan, func, complex*                               | ⇒  |              Not supported |
| time.Time                                          | ⇒  | RFC3339-format JSON string |
| []byte                                             | ⇒  | Base64-encoded JSON string |

💡提示
- Go time.Time值（实际上是一个幕后结构）将被编码为RFC 3339格式的JSON字符串，如 "2020-11-08T06:27:59+01:00"，而不是一个JSON对象。
- []字节片段将被编码为Base64编码的JSON字符串，而不是JSON数组。因此，例如，在json输出中，[]byte{‘h’，‘e’，‘L’，‘L’，‘o’}的字节片段将显示为“aGVsbG8=”。Base64编码使用填充和标准字符集。
- 任何指针值都将编码为所指向的值。
- map 中的项 按字母顺序排序，[]byte 采用 base64 编码
### json.Encoder
可以将对象编码为 JSON，并在一个步骤中将 JSON 写入输出流。
```go
err := json.NewEncoder(w).Encode(data)
```
#### 缺陷
当我们调用 json.NewEncoder(w).Encode(data) 时，JSON 将一次性创建并写入 http.ResponseWriter，这意味着没有机会根据 Encode() 方法是否返回错误来有条件地设置 HTTP 响应头。
### 性能差异
> json.Marshal() 所需的内存 (B/op) 比 json.Encoder 稍微多一点，而且还额外分配了一次堆内存 (allocs/op)。

### 在 JSON 对象中隐藏 struct 字段
#### - 
如果不想让某个结构字段出现在 JSON 输出中，可以使用-（连字符）指令。这对于包含与用户无关的内部系统信息或不想暴露的敏感信息（如密码的哈希值）的字段非常有用。
#### omitempty
如果且仅当结构字段值为空时，omitempty 指令才会在 JSON 输出中隐藏字段:
- 等于 false, 0, 或者 ""
- 空 array, slice 或者 map
- 一个 nil 指针 或者 一个 nil 接口值

:如果您想使用省略而不更改键名，则可以在 struct 标记中留空，如下所示： `json:",omitempty"`。注意，**逗号仍然是必需的**。

#### demo
```go
type Movie struct {
    ID int64 `json:"id"`
    CreatedAt time.Time `json:"-"` // Use the - directive
    Title string `json:"title"`
    Year int32 `json:"year,omitempty"` // Add the omitempty directive
    Runtime int32 `json:"runtime,omitempty"` // Add the omitempty directive
    Genres []string `json:"genres,omitempty"` // Add the omitempty directive
    Version int32 `json:"version"`
}
```
> 也可以通过简单地将结构字段设置为未导出来(也就是字段名首字母小写)防止它出现在 JSON 输出中。不过，使用 `json:"-"` struct 标记通常是更好的选择：它向 Go 和未来的代码阅读者明确表明，您不希望在 JSON 中包含该字段，而且还有助于防止将来有人在未意识到后果的情况下更改要导出的字段时出现问题。

结构体注释中的 `string` 指令，可以将字段在 json 输出中的类型转换为字符串，如想将上述结构体中的 RunTime 输出为 string 则可以将字段后的注释 从 `json:"runtime,omitempty"` 转换为 `json:"runtime,omitempty,string"`

请注意，**`string` 指令只适用于 int*、uint*、float 或 bool 类型的 struct 字段。对于其他类型的 struct 字段，该指令都不起作用。**

### Go 如何在幕后处理 JSON 编码
> 当 Go 将特定类型编码为 JSON 时，它会查看该类型是否有 MarshalJSON() 方法。如果有，Go 会调用该方法来确定如何编码

严格来说，当 Go 将特定类型编码为 JSON 时，它会查看该类型是否满足 json.Marshaler 接口，如下所示

```go
type Marshaler interface {
    MarshalJSON() ([]byte, error)
}
```

> 需要注意的一种特殊情况是，客户端在 JSON 请求中明确提供了一个值为 null 的字段。在这种情况下，我们的处理程序将忽略该字段，并将其视为未提供。

> 在理想情况下，这种类型的请求会返回某种验证错误。但是，除非您编写了自己的自定义 JSON 解析器，否则无法确定客户端在 JSON 中未提供键/值对与提供空值之间的区别。

> 在大多数情况下，只需在端点的客户端文档中解释这种特殊情况下的行为，并说明 "具有空值的 JSON 项目将被忽略并保持不变 "之类的内容即可。

## 数据库迁移
### 工具：golang-migrate
#### 创建 迁移文件
```shell
migrate create -seq -ext=.sql -dir=./migrations create_movies_table
```
- -seq标志表示我们希望对迁移文件使用顺序编号，如0001、0002、...（而不是默认的Unix时间戳）。
- -ext标志表示我们要给迁移文件添加 .sql 扩展名。
- -dir标志表示要将迁移文件保存在 ./migrations 目录中（如果该目录不存在，将自动创建）。
- create_movies_table 这个名称是一个描述性标签，我们要给迁移文件加上这个标签，以标明其内容。

#### 执行迁移文件
```shell
migrate -path=./migrations -database=$GREENLIGHT_DB_DSN up
```

#### 查看数据库当前所在的迁移版本
```shell
migrate -path=./migrations -database=$EXAMPLE_DSN version
```
#### 使用 goto 命令 up 或 down 迁移到特定版本
```shell
 migrate -path=./migrations -database=$EXAMPLE_DSN goto 1
```

#### 要回滚最近的迁移
```shell
migrate -path=./migrations -database =$EXAMPLE_DSN down 1
```

#### 回滚所有迁移
```shell
 migrate -path=./migrations -database=$EXAMPLE_DSN down
```
#### 迁移出现问题时 强行将数据库迁移到指定数据库版本
```shell
migrate -path=./migrations -database=$EXAMPLE_DSN force 1
```
#### 从亚马逊 S3 和 GitHub 资源库等远程源读取迁移文件
```shell
migrate -source="s3://<bucket>/<path>" -database=$EXAMPLE_DSN up
migrate -source="github://owner/repo/path#ref" -database=$EXAMPLE_DSN up
migrate -source="github://user:personal-access-token@owner/repo/path#ref" -database=$EXAMPLE_DSN up
```

## 数据库设计
### movies 
> 这可能会让你产生这样的疑问：既然电影 ID 从来都不是负数，为什么我们不在 Go 代码中使用无符号 uint64 类型来存储 ID，而要用 int64 类型呢？
- 第一个原因是 PostgreSQL 没有无符号整数。因此，由于 PostgreSQL 没有无符号整数，这意味着我们应该避免在 Go 代码中为读取/写入 PostgreSQL 的任何值使用 uint 类型。
- 还有一个更微妙的原因。Go 的数据库/sql 包实际上不支持任何大于 9223372036854775807（int64 的最大值）的整数值。uint64 的值有可能大于这个值，这反过来又会导致 Go 生成类似的运行时错误：
```shell
sql: converting argument $1 type: uint64 values with high bit set are not supported
```

#### 全文搜索
您可以在 PostgreSQL 中运行 \dF 元命令，获取所有可用配置的列表
使用其他的配置，如 english
```postgresql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (to_tsvector('english', title) @@ plainto_tsquery('english', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY id
```
#### 模糊匹配 使用 STRPOS 和 ILIKE
##### STRPOS
> PostgreSQL STRPOS() 函数允许您检查特定数据库字段中是否存在子串。
```postgresql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (STRPOS(LOWER(title), LOWER($1)) > 0 OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY id
```
缺点：
- 从客户的角度来看，这样做的缺点是可能会返回一些不直观的结果。例如，在我们的数据集中搜索 title=the 会同时返回 The Breakfast Club 和 Black Panther。
- 从服务器的角度来看，这也不是大型数据集的理想选择。因为没有有效的方法来索引标题字段以查看是否满足 STRPOS() condition 条件，这意味着每次运行查询时都可能需要进行全表扫描
##### ILIKE
> 另一个选项是 ILIKE 运算符，通过它可以查找与特定（不区分大小写）模式匹配的记录。
```postgresql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (title ILIKE $1 OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY id
```
- 从服务器角度看，这种方法更好，因为可以使用 pg_trgm 扩展和 GIN 索引在标题字段上创建索引 [post](https://niallburkley.com/blog/index-columns-for-like-in-postgres/)
- 从客户端来说，这种方法也比 STRPOS() 方法要好，因为他们可以通过在搜索词前缀/后缀添加 % 通配符（在 URL 查询字符串中需要转义为 %25）来控制匹配行为。例如，要搜索标题以 "the "开头的电影，客户可以发送查询字符串参数 title=the%25
#### 排序
> 如果我们不包含 ORDER BY 子句，那么 PostgreSQL 可能会以任何顺序返回电影，而且每次运行查询时，顺序可能会改变，也可能不会改变。

幸运的是，保证顺序非常简单，我们只需确保 ORDER BY 子句始终包含主键列（或其他具有唯一性约束的列）。因此，在我们的例子中，我们可以对 id 列进行二级排序，以确保顺序始终一致。就像这样:

```postgresql
SELECT id, created_at, title, year, runtime, genres, version
FROM movies
WHERE (STRPOS(LOWER(title), LOWER($1)) > 0 OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY year DESC, id ASC
```
## psql 查询
> psql 工具总是以十六进制编码字符串的形式显示字节值。因此，上面输出中的 password_hash 字段显示的是 bcrypt 哈希值的十六进制编码。如果需要，也可以运行下面的查询，将普通字符串版本追加到表中：
```postgresql
SELECT * , encode(password_hash, 'escape') FROM users .
```
### 电子邮件大小写敏感性
- 由于 RFC 2821 中的规范，电子邮件地址（username@domain）的域名部分不区分大小写。这意味着我们可以确信 alice@example.com 背后的真实用户与 alice@EXAMPLE.COM 是同一个人。
- 电子邮件地址中的用户名部分可能区分大小写，也可能不区分，这取决于电子邮件提供商。几乎所有主要的电子邮件提供商都将用户名视为不区分大小写，但也不是绝对保证。我们只能说，alice@example.com 这个地址背后的真实用户很可能（但不一定）与 ALICE@example.com 相同。

从安全的角度来看，我们应该**始终使用用户注册时提供的准确格式来存储电子邮件地址，并且只能使用该准确格式向用户发送电子邮件**。否则，就有可能将电子邮件发送给错误的真实用户。在使用电子邮件进行身份验证的工作流程（如密码重置工作流程）中，尤其要注意这一点。

不过，由于 alice@example.com 和 ALICE@example.com 很可能是同一个用户，因此我们通常应将电子邮件地址视为不区分大小写，以便进行比较。在我们的注册工作流程中，使用不区分大小写的比较方法可以防止用户因使用不同的大小写而意外（或故意）注册多个账户。

从用户体验的角度来看，在登录、激活或密码重置等工作流程中，如果我们不要求用户使用与注册时完全相同的电子邮件大小写提交请求，就会对用户更加宽容。

### 用户枚举攻击
在用户注册时所用的邮箱已经被占用的话，会返回 
```shell
curl -d "$BODY" localhost:4000/v1/users
{
  "error": {
    "email": "a user with this email address already exists"
  }
}
```
攻击者轻易的就可以知道某个指定的邮箱是否注册过账号，从而使用泄漏的用户密码库来进行撞库

防止枚举攻击通常需要做到两点：
- 无论用户是否存在，确保发送给客户端的响应始终完全相同。一般来说，这意味着要改变回复措辞，使其含糊不清，并在侧信道中通知用户任何问题（如向用户发送电子邮件，告知其已拥有账户）。
- 确保无论用户是否存在，发送响应所需的时间始终相同。在 Go 中，这通常意味着将工作卸载到后台程序中。

**避免响应结果的差异化，从而被找出规律**

## 使用嵌入式文件系统
- 您只能在包级别的全局变量上使用 go:embed 指令，而不能在函数或方法中使用。如果试图在函数或方法中使用该指令，编译时会出现 "go:embed cannot apply to var inside func "的错误。
- 使用 go:embed "<path>"指令创建嵌入式文件系统时，路径应相对于包含该指令的源代码文件。因此，在我们的例子中，go:embed "templates" 嵌入了 internal/mailer/templates 目录的内容。
- 嵌入式文件系统根目录是包含 go:embed 指令的目录。因此，在我们的例子中，要获取 user_welcome.tmpl 文件，我们需要从嵌入式文件系统中的 templates/user_welcome.tmpl 文件中获取。
- 路径不能包含.或.元素，也不能以.开头或结尾。 这基本上限制了你只能嵌入与带有 go:embed 指令的源代码位于同一目录（或子目录）中的文件。
- 如果路径是一个目录，那么目录中的所有文件都会被递归嵌入，名称以 . 或 _ 开头的文件除外。如果要包含这些文件，应在路径中使用通配符，如 go:embed "templates/*"
- 您可以在一条指令中指定多个目录和文件。例如： go:embed "images" "styles/css" "favicon.ico" .
- 路径分隔符应始终为正斜线 / ，即使在 Windows 机器上也是如此。

## 优雅关闭后台任务
我们可以使用 Go 的 sync.WaitGroup 功能来协调优雅关机和后台程序。
> 当你想等待一组 goroutines 完成它们的工作时，主要的辅助工具是 sync.WaitGroup 类型
> 
> 它的工作方式在概念上有点像 "计数器"。每次启动后台程序时，你都可以将计数器递增 1，当每个后台程序结束时，你再将计数器递减 1。 然后你就可以监控计数器，当计数器等于零时，你就知道所有后台程序都已结束。

```go
package main
import (
    "fmt"
    "sync"
)
func main() {
    // Declare a new WaitGroup.
    var wg sync.WaitGroup
    // Execute a loop 5 times.
    for i := 1; i <= 5; i++ {
        // Increment the WaitGroup counter by 1, BEFORE we launch the background routine.
        wg.Add(1)
        // Launch the background goroutine.
        go func() {
            // Defer a call to wg.Done() to indicate that the background goroutine has
            // completed when this function returns. Behind the scenes this decrements
            // the WaitGroup counter by 1 and is the same as writing wg.Add(-1).
            defer wg.Done()
            fmt.Println("hello from a goroutine")
        }()
    }
    // Wait() blocks until the WaitGroup counter is zero --- essentially blocking until all
    // goroutines have completed.
    wg.Wait()
    fmt.Println("all goroutines finished")
}
```
outputs
```shell
hello from a goroutine
hello from a goroutine
hello from a goroutine
hello from a goroutine
hello from a goroutine
all goroutines finished
```
> 这里需要强调的一点是，我们在启动后台程序之前立即用 wg.Add(1) 增加计数器。如果我们在后台程序中调用 wg.Add(1)，就会出现竞赛条件，因为 wg.Wait() 有可能在计数器递增之前被调用。