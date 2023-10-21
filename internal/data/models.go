package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

type Models struct {
	Movies interface {
		Insert(movie *Movie) error
		Get(id int64) (*Movie, error)
		Update(movie *Movie) error
		Delete(id int64) error
	}
}

func NewModels(db *sql.DB) Models {
	return Models{
		Movies: MovieModel{DB: db},
	}
}

// NewMockModels 创建一个辅助函数，返回一个只包含模拟模型的 Models 实例
func NewMockModels() Models {
	return Models{
		Movies: MovieModel{},
	}
}
