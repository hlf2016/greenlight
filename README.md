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


