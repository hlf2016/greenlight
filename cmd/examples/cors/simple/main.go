package main

import (
	"flag"
	"log"
	"net/http"
)

// 定义一个包含网页 HTML 的字符串常量。
// 这包括一个 <h1> 标题标记和一些 JavaScript，这些 JavaScript 会从我们的 GET v1/healthcheck 端点获取 JSON 并将其写入 <div id="output"></div> 元素内。
const html = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
</head>
<body>
<h1>Simple CORS</h1>
<div id="output"></div>
<script>
document.addEventListener('DOMContentLoaded', function() {
fetch("http://localhost:4000/v1/healthcheck").then(
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
	addr := flag.String("addr", ":9000", "Server Addr")
	flag.Parse()

	log.Printf("starting server on %s", *addr)

	err := http.ListenAndServe(*addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	log.Fatal(err)
}
