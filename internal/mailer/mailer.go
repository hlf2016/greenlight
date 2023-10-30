package mailer

import (
	"bytes"
	"embed"
	"github.com/go-mail/mail/v2"
	"html/template"
	"time"
)

// 下面我们声明了一个类型为 embed.FS（嵌入式文件系统）的新变量，用于保存电子邮件模板。
// 该变量上方有一个格式为 `//go:embed <path>` 的注释指令，它向 Go 表明我们要将 ./templates 目录中的内容存储到 templateFS 嵌入式文件系统变量中。

//go:embed "templates"
var templateFS embed.FS

// Mailer 定义 Mailer 结构，其中包含 mail.Dialer 实例（用于连接到 SMTP 服务器）和邮件的发件人信息（您希望邮件来自的姓名和地址，如 "Alice Smith <alice@example.com>"）。
type Mailer struct {
	dialer *mail.Dialer
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	// 使用给定的 SMTP 服务器设置初始化一个新的 mail.Dialer 实例。我们还将其配置为在发送电子邮件时使用 5 秒超时。
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 3 * time.Second

	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

// Send 在 Mailer 类型上定义 Send() 方法。该方法的第一个参数是收件人电子邮件地址，第二个参数是包含模板的文件名，第三个参数是模板的任何动态数据。
func (m Mailer) Send(recipient, templateFile string, data any) error {
	// 使用 ParseFS() 方法从嵌入式文件系统中解析所需的模板文件。
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}
	// 执行命名为 "subject "的模板，传入动态数据并将结果存储在 bytes.Buffer 变量中。
	subject := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}
	// 按照同样的模式执行 "plainBody "模板，并将结果存储在 plainBody 变量中。
	plainBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	// 同样，"htmlBody "模板也是如此。
	htmlBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	// 使用 mail.NewMessage() 函数初始化一个新的 mail.Message 实例。
	// 然后使用 SetHeader() 方法设置邮件收件人、发件人和主题标题，使用 SetBody() 方法设置纯文本正文，使用 AddAlternative() 方法设置 HTML 正文。
	// 值得注意的是，AddAlternative() 应始终在 SetBody() 之后调用。
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	// 调用拨号器上的 DialAndSend() 方法，并传入要发送的信息。
	// 该方法会打开与 SMTP 服务器的连接，发送信息，然后关闭连接。如果出现超时，则会返回 "dial tcp: i/o timeout "错误信息。
	err = m.dialer.DialAndSend(msg)
	if err != nil {
		return err
	}
	return nil
}
