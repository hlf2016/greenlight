package main

import (
	"errors"
	"greenlight.311102.xyz/internal/data"
	"greenlight.311102.xyz/internal/validator"
	"net/http"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	// 注意 var input struct{} 和 type input struct{} 区别
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// 将请求正文中的数据复制到新的 User 结构中。还要注意的是，我们将 Activated 字段设置为 false，这在严格意义上并非必要，因为 Activated 字段默认值为零，即 false。不过，明确设置这个字段有助于让阅读代码的人清楚我们的意图。
	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// 单独开一个 goroutine 来进行邮件发送 减少程序阻塞
	go func() {
		// 调用邮件发送器的 Send() 方法，传入用户的电子邮件地址、模板文件名称和包含新用户数据的 User 结构。
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", user)
		if err != nil {
			// 重要的是，如果发送电子邮件时出现错误，我们将使用 app.logger.PrintError() 助手来处理，而不是像以前那样使用 app.serverErrorResponse() 助手。
			// 我们使用 app.logger.PrintError() 助手来管理后台程序中的任何错误。这是因为当我们遇到错误时，客户端很可能已经通过我们的 writeJSON() 辅助程序发送了 202 Accepted 响应。请注意，我们不想使用 app.serverErrorResponse() 辅助函数来处理后台程序中的任何错误，因为这会导致我们尝试编写第二个 HTTP 响应，并在运行时从 http.Server 收到 "http: superfluous response.WriteHeader call"（http：多余的 response.WriteHeader 调用）错误。
			// 在后台程序中运行的代码会对用户和应用程序变量形成封闭。需要注意的是，这些 "封闭 "变量的作用域与后台程序无关，这意味着你对它们所做的任何更改都会反映在代码库的其他部分。
			// 在我们的例子中，我们没有以任何方式更改这些变量的值，因此此行为不会给我们带来任何问题。但重要的是要记住这一点。
			app.logger.PrintError(err, nil)
		}

	}()

	// 请注意，我们也会将其改为向客户端发送 202 Accepted 状态代码。此状态代码表示请求已被接受处理，但处理尚未完成
	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
