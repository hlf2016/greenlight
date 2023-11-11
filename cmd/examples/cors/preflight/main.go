package main

import (
	"flag"
	"log"
	"net/http"
)

// 定义一个包含网页 HTML 的字符串常量。这包括一个 <h1> 标题标记，以及一些调用 POST v1/tokens/authentication 端点并将响应正文写入 <div id="output"></div> 标记内的 JavaScript。
const html = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
</head>
<body>
<h1>Preflight CORS</h1>
<div id="output"></div>
<script>
document.addEventListener('DOMContentLoaded', function() {
fetch("http://localhost:4000/v1/tokens/authentication", {
method: "POST",
headers: {
'Content-Type': 'application/json'
},
body: JSON.stringify({
email: 'bob@example.com',
password: 'pa55word'
})
}).then(
function (response) {
response.text().then(function (text) {
document.getElementById("output").innerHTML = text;
});
},
function(err) {
document.getElementById("output").innerHTML = err;
}
);
});
</script>
</body>
</html>`

func main() {
	addr := flag.String("addr", ":9000", "Server address")
	flag.Parse()

	log.Printf("starting server on %s", *addr)

	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))

	log.Fatal(err)
}
