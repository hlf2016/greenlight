package data

import (
	"greenlight.311102.xyz/internal/validator"
	"math"
	"strings"
)

type Filters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	// 检查排序参数是否与安全列表中的值匹配。
	v.Check(validator.PermittedValue(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

// 检查客户提供的 "排序 "字段是否与安全列表中的某个条目相匹配，如果相匹配，则从 "sort" 字段中提取列名，删除前导连字符（如果存在）。
func (f Filters) sortColumn() string {
	for _, safeValue := range f.SortSafeList {
		if safeValue == f.Sort {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}
	panic("unsafe sort parameter:" + f.Sort)
}

// 根据排序字段的前缀字符，返回排序方向（"ASC "或 "DESC"）。
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

func (f Filters) limit() int {
	return f.PageSize
}

func (f Filters) offset() int {
	// 注意这里可能出现 整数溢出的情况  但是好在我们在 ValidateFilters 已经进行了限制  可规避这种风险
	return (f.Page - 1) * f.PageSize
}

// Metadata omitempty 空属性 省略
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// calculateMetadata() 函数根据记录总数、当前页面和页面大小值计算相应的分页元数据值。
// 请注意，最后一页的值是使用 math.Ceil() 函数计算的，该函数将浮点数四舍五入为最接近的整数。
// 因此，举例来说，如果总共有 12 条记录，页面大小为 5，那么最后一页的值就是 math.Ceil(12/5) = 3。
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
