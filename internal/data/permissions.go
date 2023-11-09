package data

import (
	"context"
	"database/sql"
	"time"
)

// Permissions 定义一个权限片段，我们将用它来保存单个用户的权限代码（如 "movies:read "和 "movies:write"）。单个用户的 "movies:read "和 "movies:write "权限代码
type Permissions []string

func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}
	return false
}

type PermissionModel struct {
	DB *sql.DB
}

// GetAllForUser 方法返回 Permissions 片中特定用户的所有权限代码。该方法中的代码应该非常熟悉--它使用的是我们以前在 SQL 查询中检索多条数据行时见过的标准模式。
func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
	query := `
	SELECT permissions.code 
	FROM permissions
	INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id
	INNER JOIN users ON users_permissions.user_id = users.id
	WHERE users.id=$1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions
	for rows.Next() {
		var permission string

		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}
