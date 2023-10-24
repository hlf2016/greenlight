package main

import (
	"errors"
	"fmt"
	"greenlight.311102.xyz/internal/data"
	"greenlight.311102.xyz/internal/validator"
	"net/http"
	"strconv"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		RunTime data.RunTime `json:"run_time"`
		Genres  []string     `json:"genres"`
	}
	// json.Unmarshal() 比 json.Decoder 多用 80% 内存 且更慢一些
	// err := json.NewDecoder(r.Body).Decode(&input)

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		RunTime: input.RunTime,
		Genres:  input.Genres,
	}

	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Movies.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// 在发送 HTTP 响应时，我们希望包含一个 Location 标头，让客户端知道他们可以在哪个 URL 找到新创建的资源。
	// 我们先创建一个空的 http.Header map，然后使用 Set() 方法添加一个新的 Location 标头，在 URL 中插入系统生成的新电影 ID。
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	// 编写一个 JSON 响应，状态代码为 201 "已创建"，在响应正文中包含影片数据和位置标头。
	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// 如果请求包含 X-Expected-Version 标头，则要验证数据库中的电影版本是否与标头中指定的预期版本一致。
	if r.Header.Get("X-Expected-Version") != "" {
		// FormatInt 返回 i 以给定基数表示的字符串，2 <= 基数 <= 36。对于大于等于 10 的数字值，结果使用小写字母 "a "至 "z "表示。
		if strconv.FormatInt(int64(movie.Version), 32) != r.Header.Get("X-Expected-Version") {
			app.editConflictResponse(w, r)
			return
		}
	}

	// 为了实现 按需修改 而不是 次次 都完全替换 即 将 put 替换成 patch
	// 因为普通值传递类型 当json解析时候 字段不存在传值时直接将其赋值未该值类型的 0 值  无法区分 缺少字段直接报错 还是未传值 对字段不进行修改
	// 所以将值传递类型 改为存储指针 对 genres 切片 类型则无需处理 若修改提交上来的 json 数据中缺少某个 字段 则为 nil
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		RunTime *data.RunTime `json:"run_time"`
		Genres  []string      `json:"genres"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Title != nil {
		movie.Title = *input.Title
	}

	if input.Year != nil {
		movie.Year = *input.Year
	}

	if input.RunTime != nil {
		movie.RunTime = *input.RunTime
	}

	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	// 验证更新的电影记录，如果任何检查失败，则向客户端发送 422 不可处理实体响应。
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	err = app.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	// 为了与其他处理程序保持一致，我们将定义一个输入结构来保存来自请求查询字符串的预期值。
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	v := validator.New()

	qs := r.URL.Query()

	// 使用我们的助手提取标题和基因查询字符串值，如果客户端没有提供，则分别返回默认的空字符串和空片段值
	input.Title = app.readString(qs, "title", "")
	input.Genres = app.readCSV(qs, "genres", []string{})
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	// 提取排序查询字符串值，如果客户端未提供，则返回到 "id"（这意味着将根据影片 ID 升序排序）。
	input.Filters.Sort = app.readString(qs, "sort", "id")

	input.Filters.SortSafeList = []string{"id", "title", "year", "run_time", "-id", "-title", "-year", "-run_time"}

	// 检查验证器实例是否有任何错误，必要时使用 failedValidationResponse() 助手向客户端发送响应。
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	// 注意，%+v 会输出字段名和相应的值，而 "\n" 则是一个换行符，使输出更易读。
	// fmt.Fprintf(w, "%+v\n", input)
}
