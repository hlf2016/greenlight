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

## math/rand 和 crypto/rand 区别
Go 还有一个 math/rand 软件包，它提供了一个确定性伪随机数生成器（PRNG）。
> 重要的是，千万不要将 math/rand 包用于任何需要加密安全的用途，例如像我们这里这样生成令牌或秘密。

事实上，可以说最好使用 crypto/rand 作为标准做法。只有在特定情况下，即确定性 PRNG 是可以接受的，并且迫切需要更快的 math/rand 性能时，才会选择使用 math/rand。

##  身份验证和授权
> **Remember**: Authentication is about confirming who a user is, whereas authorization is
about checking whether that user is permitted to do something.

> 身份验证是关于确认用户是谁，而授权是关于检查该用户是否被允许执行某些操作

### 身份验证选项
- HTTP 基本认证
> 使用这种方法时，客户端会在每个请求中包含一个授权头，其中包含他们的凭据。凭据的格式为 username:password 和 base-64 编码。例如，要以 alice@example.com:pa55word 身份进行身份验证，客户端将发送以下头信息：
>Authorization: Basic YWxpY2VAZXhhbXBsZS5jb206cGE1NXdvcmQ=

在您的应用程序接口中，您可以使用 Go 的 `Request.BasicAuth()` 方法从该标头中提取凭据，并在继续处理请求之前验证它们是否正确。

- Token 验证
  - 有状态token
    > 在有状态令牌方法中，令牌的值是一个高熵加密安全随机字符串。这个令牌或其快速散列值与用户 ID 和令牌的到期时间一起存储在服务器端的数据库中。
    
    如 session 方式

  - 无状态token
    > 相比之下，无状态令牌将用户 ID 和过期时间编码在令牌本身中。令牌经过加密签名以防篡改，并（在某些情况下）进行加密以防内容被读取。
    
    如 jwt token
- API-key 身份验证
  > API 密钥身份验证背后的理念是，用户拥有与其账户相关联的非过期秘密 "密钥"。这个密钥应该是一个高熵加密安全随机字符串，密钥的快速散列（SHA256 或 SHA512）应该与相应的用户 ID 一起存储在数据库中。然后，用户每次向 API 请求时，都会在类似这样的标头中传递他们的密钥: Authorization: Key <key>

  从概念上讲，这与 `有状态Token` 方法相差无几，主要区别在于**密钥是永久密钥，而不是临时令牌**。
- OAuth 2.0 OpenID Connect
  > 另一种方法是利用 OAuth 2.0 进行身份验证。使用这种方法，用户的信息（及其密码）将由第三方身份提供商（如 Google 或 Facebook）而不是你自己来存储。

  https://github.com/coreos/go-oidc

## 读取和写入请求上下文
- 我们的应用程序处理的每个 http.Request 都嵌入了一个 context.Context，我们可以用它来存储请求生命周期内包含任意数据的键/值对。在本例中，我们要存储一个包含当前用户信息的 User 结构。
- 存储在请求上下文中的任何值的类型都是 any。这意味着从请求上下文中获取值后，需要在使用前将其恢复为原始类型。
- 为请求上下文键使用自己的自定义类型是一种很好的做法。这有助于防止代码与同样使用请求上下文存储信息的第三方软件包之间发生命名冲突。

> 当认证缺失或认证错误时，应使用 401 Unauthorized 响应；当用户已通过认证但不允许执行请求的操作时，应随后使用 403 Forbidden 响应。

## permissions 测试数据填充
```postgresql
-- Set the activated field for alice@example.com to true.
UPDATE users SET activated = true WHERE email = 'alice@example.com';
-- Give all users the 'movies:read' permission
INSERT INTO users_permissions
SELECT id, (SELECT id FROM permissions WHERE code = 'movies:read') FROM users;
-- Give faith@example.com the 'movies:write' permission
INSERT INTO users_permissions
VALUES (
(SELECT id FROM users WHERE email = 'faith@example.com'),
(SELECT id FROM permissions WHERE code = 'movies:write')
);
-- List all activated users and their permissions.
SELECT email, array_agg(permissions.code) as permissions
FROM permissions
INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
INNER JOIN users ON users_permissions.user_id = users.id
WHERE users.activated = true
GROUP BY email;
```
> **注意**：在最后的 SQL 查询中，我们使用了聚合函数 array_agg()和 GROUP BY 子句，以数组形式输出与每个电子邮件地址相关的权限。

## CORS 跨域
了解什么是 origins 非常重要，因为所有网络浏览器都会实施一种称为同源策略的安全机制。浏览器实施这一策略的方式存在一些细微差别，但大体上都是如此：
- 一个来源地的网页可以在其 HTML 中嵌入来自另一个来源地的某些类型的资源，包括图片、CSS 和 JavaScript 文件。例如，在网页中这样做是可以的：
  ```html
  <img src="http://anotherorigin.com/example.png" alt="example image">
  ```
- 一个来源的网页可以将数据发送到不同的来源。例如，网页中的一个HTML表单可以将数据提交到不同的来源。
- 但一个来源的网页不允许接收来自不同来源的数据

> 这里的关键是最后一个要点：同源策略可以防止另一个源网站（可能是恶意的）从你的网站读取（可能是机密的）信息。

> 需要强调的是，同源策略并不阻止跨源发送数据，尽管这也很危险。事实上，这就是 CSRF 攻击可能发生的原因，也是我们需要采取额外措施来防止它们的原因，比如使用 SameSite cookie 和 CSRF 标记。

### origin 为 null
> 切勿在安全列表中将 "null "值作为可信来源。这是因为攻击者可以通过从沙盒 iframe 发送请求来伪造请求标头 Origin: null。
[sandboxed iframe](https://stackoverflow.com/questions/44764338/origin-header-null-for-xhr-request-made-from-iframe-with-sandbox-attribute/44765536#44765536)

### 身份验证和 CORS
如果您的 API 端点需要凭证（cookie 或 HTTP 基本身份验证），您还应在响应中设置 Access-Control-Allow-Credentials: true 标头。如果不设置此标头，网络浏览器就会阻止 JavaScript 读取任何带有凭据的跨源响应。

重要的是，您绝不能将通配符 Access-Control-Allow-Origin:* 标头与 Access-Control-Allow-Credentials: true 结合使用，因为这将允许任何网站向您的 API 提出带凭证的跨源请求。

另外，重要的是，如果您希望在发送跨源请求时发送凭证，那么您需要在 JavaScript 中明确指定这一点。例如，在使用 fetch() 时，应将请求的凭据值设置为 "include"

```javascript
fetch("https://api.example.com", {credentials: 'include'}).then( ... );
```
或者，如果使用 XMLHTTPRequest，则应将 withCredentials 属性设置为 true。例如
```javascript
var xhr = new XMLHttpRequest();
xhr.open('GET', 'https://api.example.com');
xhr.withCredentials = true;
xhr.send(null);
```
### 预检CORS请求
当满足以下所有条件时，跨源请求被归类为 "简单 "请求：
- 请求 HTTP 方法是三种 CORS 安全方法之一：HEAD、GET 或 POST。
- 请求标头均为[禁止标头](https://developer.mozilla.org/en-US/docs/Glossary/Forbidden_header_name)或四种 CORS 安全标头之一：
  - Accept
  - Accept-Language
  - Content-Language
  - Content-Type
- Content-Type 标头（如果设置）的值为以下之一
  - application/x-www-form-urlencoded
  - multipart/form-data
  - text/plain

当跨源请求不符合这些条件时，网络浏览器会在真正请求之前触发一个初始 "预检 "请求。预检请求的目的是确定是否允许真正的跨源请求。

#### 缓存预检响应
如果需要，还可以在预检响应中添加 Access-Control-Max-Age 标头。
这表示浏览器可以缓存 Access-Control-Allow-Methods 和 Access-Control-Allow-Headers 头信息的秒数。
例如，如果要将这些值缓存 60 秒，可以在预检响应中设置以下标头：
```
Access-Control-Max-Age: 60
```
如果不设置 Access-Control-Max-Age 标头，当前版本的 Chrome/Chromium 和 Firefox 将默认缓存这些预检响应值 5 秒钟。
旧版本或其他浏览器可能有不同的默认值，或者根本不缓存这些值。

设置较长的 Access-Control-Max-Age 持续时间似乎是减少对 API 请求的一种有效方法，事实也确实如此！
但你也需要小心。并非所有浏览器都提供了清除预检缓存的方法，因此如果您发回了错误的标头，用户就会一直使用这些标头，直到缓存过期。

如果想完全禁用缓存，可以将值设为-1：
```
Access-Control-Max-Age: -1
```

同样重要的是要注意，浏览器可能会硬性规定标头缓存的最长时间。MDN 文档指出:
>- Firefox caps this at 24 hours(86400 seconds).
>- Chromium (prior to v76) caps at 10 minutes(600 seconds).
>- Chromium (starting in v76) caps at 2 hours(7200 seconds).

#### 预检请求通配符
如果您有一个复杂或快速变化的 API，那么为预检响应维护一个硬编码的方法和标头安全列表可能会很麻烦。
你可能会想：我只想允许跨源请求使用所有 HTTP 方法和头信息。

在这种情况下，Access-Control-Allow-Methods（访问控制允许的方法）和 Access-Control-Allow-Headers （访问控制允许的头信息）头信息都允许你使用 `*` 通配符，如图所示：
```
Access-Control-Allow-Methods: *
Access-Control-Allow-Headers: *
```

不过，使用时也有一些重要的注意事项：
- 目前只有 74% 的浏览器支持这些标头中的通配符。任何不支持通配符的浏览器都会阻止预检请求。
- Authorization 头不能通配符。取而代之的是，你需要在头信息中明确包含这一点，如 `Access-Control-Allow-Headers: Authorization, *` 
- 通配符不支持认证请求（带 Cookie 或 HTTP 基本认证的请求）。对于这些请求，字符将被视为字面字符串 "*"，而不是通配符。

## metrics 中间件另一种写法--嵌入式 http.ResponseWriter
如果你愿意，可以更改 metricsResponseWriter 结构，使其嵌入 http.ResponseWriter 而不是封装它。就像这样:
```go
type metricsResponseWriter struct {
  http.ResponseWriter
  statusCode int
  headerWritten bool
}
func (mw *metricsResponseWriter) WriteHeader(statusCode int) {
  mw.ResponseWriter.WriteHeader(statusCode)
  if !mw.headerWritten {
    mw.statusCode = statusCode
    mw.headerWritten = true
  }
}
func (mw *metricsResponseWriter) Write(b []byte) (int, error) {
  if !mw.headerWritten {
    mw.statusCode = http.StatusOK
    mw.headerWritten = true
  }
  return mw.ResponseWriter.Write(b)
}
func (mw *metricsResponseWriter) Unwrap() http.ResponseWriter {
    return mw.ResponseWriter
}
...
mw := &metricsResponseWriter{ResponseWriter: w}
next.ServeHTTP(mw, r)
```
这样做的最终结果与原始方法相同。不过，这样做的好处是，你不需要为 metricsResponseWriter 结构编写 Header() 方法（它会从嵌入式 http.ResponseWriter 自动升级）。至少在我看来，这样做的损失是不如使用封装字段来得清晰明确。无论哪种方法都可以，关键是看你喜欢哪一种。

## make && Makefile
请注意，makefile 规则中的每条命令必须以 `Tab` 开头，而**不是空格**。

> 需要指出的一点是，默认情况下，make 会在终端输出中 echo 命令。我们可以在上面的代码中看到，输出的第一行就是 echo 命令 go run ./cmd/api 。
> 如果需要，可以在命令前加上 @ 字符，以阻止命令被 echo。

### 环境变量的使用
当我们执行 make 规则时，make 启动时可用的每个环境变量都会转化为具有相同名称和值的 make 变量。
然后，我们可以在 makefile 中使用 ${VARIABLE_NAME} 语法访问这些变量。

### 传参数
访问命名参数值的语法与访问环境变量的语法完全相同。因此，在上面的例子中，我们可以通过 makefile 中的 ${name} 访问迁移文件名。
> makefile 中的变量名区分大小写，因此 foo、FOO 和 Foo 都指代不同的变量。make 文档建议，对于只在 makefile 中起内部作用的变量名，应使用小写字母，否则应使用大写变量名。

### 命名 target
随着 makefile 的不断增长，你可能需要开始为目标名称命名，以区分不同的规则，并帮助组织文件。
例如，在一个大的 makefile 中，与其将目标命名为 up，不如将其命名为 db/migrations/up，这样会更清晰。

我建议使用 `/` 字符作为命名空间分隔符，而不是使用句号、连字符或:字符。
事实上，在目标名称中应严格避免使用:字符，因为它会在使用目标先决条件时造成问题（我们稍后会介绍）。

> 使用该字符作为命名空间分隔符的一个好处是，在终端中键入目标名称时可以使用制表符完成。例如，键入 make db/migrations，然后按键盘上的 tab 键，就会列出命名空间下的其余目标。就像这样

```shell
$ make db/migrations/
new up
```

### 先决条件 target 和要求确认
```makefile
target: prerequisite-target-1 prerequisite-target-2 ...
command
command
...
```
