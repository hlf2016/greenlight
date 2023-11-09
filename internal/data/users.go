package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"greenlight.311102.xyz/internal/validator"
	"time"
)

// AnonymousUser 声明一个新的匿名用户变量。
// 因此，我们在这里创建了一个新的 AnonymousUser 变量，它包含一个指向 User 结构的指针，代表一个没有 ID、姓名、电子邮件或密码的未激活用户。
var AnonymousUser = &User{}

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

// 创建自定义密码类型，该类型是一个结构体，包含用户密码的明文和散列版本。
// 明文字段是指向字符串的指针，这样我们就能区分结构中根本不存在明文密码和明文密码为空字符串""的情况。
type password struct {
	plaintext *string
	hash      []byte
}

// IsAnonymous 检查用户实例是否为匿名用户。
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

// User 定义一个 User 结构来表示单个用户。
// 重要的是，请注意我们是如何使用 json:"-" 结构标记来防止将密码和版本字段编码为 JSON 时出现在任何输出中的。
// 还请注意，password 字段使用了下面定义的自定义密码类型。
type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	Version   int       `json:"-"`
}

// Set 方法会计算明文密码的 bcrypt 哈希值，并将哈希值和明文版本都存储在结构体中。
func (p *password) Set(plaintextPassword string) error {
	// 注意：创建 bcrypt 哈希值时，输入最多会被截断为 72 字节。因此，如果有人使用了很长的密码，这意味着在创建哈希值时，后面的字节将被忽略。
	// 为了避免用户产生任何困惑，我们只需在验证检查中硬性规定密码最大长度为 72 字节。如果不想设置最大长度，也可以预先对密码进行散列。
	// 返回的数据格式 $2b$[cost]$[22-character salt][31-character hash]
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plaintextPassword
	p.hash = hash
	return nil
}

// Matches 方法会检查所提供的明文密码是否与结构体中存储的散列密码匹配，如果匹配则返回 true，否则返回 false。
func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePlaintextPassword(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePlaintextPassword(v, *user.Password.plaintext)
	}

	// 如果密码散列为Nil，这将是由于代码库中的逻辑错误(可能是因为我们忘记了为用户设置密码)。在这里包含它是一项有用的健全性检查，但它对客户端提供的数据不是问题。因此，我们没有向验证映射中添加错误，而是引发了panic
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

type UserModel struct {
	DB *sql.DB
}

func (m UserModel) Insert(user *User) error {
	query := `
		INSERT INTO users (name, email, password_hash,activated)
		VALUES ($1, $2, $3, $4) 
		RETURNING id, created_at, version`

	args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 如果表中已经包含一条带有此电子邮件地址的记录，那么当我们尝试执行插入操作时，就会违反我们在上一章中设置的 UNIQUE "users_email_key "约束。
	// 我们将专门检查此错误，并返回自定义 ErrDuplicateEmail 错误。
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}
	return nil
}

// GetByEmail 根据用户的电子邮件地址从数据库中读取用户详细信息。
// 由于我们在电子邮件列上使用了 UNIQUE 约束，因此此 SQL 查询只会返回一条记录（或者一条记录也没有，在这种情况下，我们会返回 ErrRecordNotFound 错误）。
func (m UserModel) GetByEmail(email string) (*User, error) {
	query := `SELECT  id, created_at, name, email, password_hash, activated, version FROM users WHERE email=$1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user User

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

// Update 更新特定用户的详细信息。请注意，我们对版本字段进行了检查，以防止在请求周期中出现任何竞赛条件，就像更新电影时一样。
// 在执行更新时，我们还会检查是否违反了 "users_email_key "约束，就像最初插入用户记录时一样。
func (m UserModel) Update(user *User) error {
	query := `UPDATE users SET 
			 name=$1,email=$2,password_hash=$3,activated=$4, version=version+1 WHERE id=$5 AND version=$6 
			 RETURNING version`
	args := []any{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	// 计算客户端提供的明文令牌的 SHA-256 哈希值。请记住，返回的是长度为 32 的字节数组，而不是片段。
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
	SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
	FROM users
	INNER JOIN tokens
	ON users.id = tokens.user_id
	WHERE tokens.hash = $1
	AND tokens.scope = $2
	AND tokens.expiry > $3
	`

	// 创建一个包含查询参数的片段。
	// 请注意我们是如何使用 [:] 操作符来获取包含令牌散列的slice，而不是传递数组（pq 驱动程序不支持数组），而且我们还传递了当前时间作为检查令牌到期的值。
	args := []any{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 执行查询，将返回值扫描到 User 结构中。如果没有找到匹配记录，我们将返回 ErrRecordNotFound 错误。
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}
