package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"greenlight.311102.xyz/internal/validator"
	"time"
)

// ScopeActivation 定义 token scope 的常量。现在，我们只定义作用域 “ activation ”，但我们将在本书后面添加其他作用域。
var (
	ScopeActivation = "activation"
)

// Token 定义一个令牌结构，用于保存单个令牌的数据。其中包括令牌的明文和散列版本、相关用户 ID、过期时间和范围。
type Token struct {
	Plaintext string
	Hash      []byte
	UserID    int64
	Expiry    time.Time
	Scope     string
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	// 创建一个包含用户 ID、有效期和范围信息的令牌实例。注意到我们将提供的 ttl（生存时间）持续时间参数添加到当前时间以获得到期时间了吗？
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}
	// 初始化长度为 16 字节的零值字节切片
	randomBytes := make([]byte, 16)
	// 使用 crypto/rand 软件包中的 Read() 函数，用操作系统 CSPRNG 中的随机字节填充字节片。如果 CSPRNG 无法正常工作，将返回错误信息。
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}
	// 将字节片段编码为基 32 编码字符串，并将其赋值给令牌 plaintext 字段。
	// 这将是我们在用户的欢迎电子邮件中发送给用户的令牌字符串。
	// 它们看起来类似于Y3QMGX3PJ3WLRL2YRTQGQ6KRHU 注意，
	// 默认情况下，base-32 字符串可能会在末尾填充 = 字符。
	// 我们的标记不需要这种填充字符，因此我们在下面一行中使用 WithPadding(base32.NoPadding) 方法省略它们。
	// 需要指出的是，我们在这里创建的纯文本令牌字符串（如 Y3QMGX3PJ3WLRL2YRTQGQ6KRHU）并不是 16 个字符的长度，而是具有 16 字节随机性的底层熵。
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// 生成明文令牌字符串的 SHA-256 哈希值。这将是我们存储在数据库表的 `hash` 字段中的值。
	// 请注意，sha256.Sum256() 函数会返回一个长度为 32 的数组，因此，为了方便使用，我们在存储前使用 [:] 操作符将其转换为一个片段
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]
	return token, nil
}

type TokenModel struct {
	DB *sql.DB
}

// ValidateTokenPlaintext 检查明文标记是否已提供，长度是否正好为 26 字节。
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// New 方法是一个快捷方式，它可以创建一个新的Token Struct，然后将数据插入tokens表。
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}
	err = m.Insert(token)
	return token, err
}

// Insert 插入Token数据
func (m TokenModel) Insert(token *Token) error {
	query := `INSERT INTO tokens (hash, user_id, expiry, scope) VALUES ($1, $2, $3, $4)`
	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args)
	return err
}

// DeleteAllForUser 会删除特定用户和作用域的所有标记。
func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `DELETE FROM tokens WHERE scope=$1 AND user_id=$2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, scope, userID)
	return err
}
