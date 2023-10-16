package validator

import "regexp"

var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// Validator 定义一个新的验证器类型，其中包含一个验证错误映射 map。
type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// PermittedValue 通用函数，如果特定值在列表中，则返回 true。
func PermittedValue[T comparable](value T, permitValues ...T) bool {
	for i := range permitValues {
		if value == permitValues[i] {
			return true
		}
	}
	return false
}

// Matches 如果字符串值与特定 regexp 模式匹配，则 Matches 返回 true。
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// Unique 通用函数，如果片段中的所有值都是唯一的，则返回 true。
func Unique[T comparable](values []T) bool {
	uniqueValues := make(map[T]bool)
	for _, value := range values {
		uniqueValues[value] = true
	}
	return len(values) == len(uniqueValues)
}
