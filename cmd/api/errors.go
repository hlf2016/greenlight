package main

import (
	"fmt"
	"net/http"
)

// logError() 方法是记录错误信息的通用助手。在本书的后面部分，我们将对该方法进行升级，以使用结构化日志，并记录有关请求的其他信息，包括 HTTP 方法和 URL。
func (app *application) logError(r *http.Request, err error) {
	app.logger.Print(err)
}

// errorResponse() 方法是一个通用辅助方法，用于向客户端发送带有给定状态代码的 JSON 格式错误消息。请注意，我们为消息参数使用的是任意类型，而不仅仅是字符串类型，因为这让我们可以更灵活地处理响应中包含的值。
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelope{"error": message}
	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// serverErrorResponse() 方法将在应用程序运行时遇到意外问题时使用。它会记录详细的错误信息，然后使用 errorResponse() 助手向客户端发送 500 Internal Server Error 状态代码和 JSON 响应（包含通用错误信息）。
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// notFoundResponse() 方法将用于向客户端发送 404 Not Found 状态代码和 JSON 响应。
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

// methodNotAllowedResponse() 方法将用于向客户端发送 405 Method Not Allowed 状态代码和 JSON 响应。
func (app *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to an edit conflict, please try again"
	app.errorResponse(w, r, http.StatusConflict, message)
}
