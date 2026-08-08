package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

// frontend 包含由 Vite 构建的前端静态资源。
//
//go:embed all:web-dist
var frontend embed.FS

func defaultDataDir() string {
	return "/data"
}

func frontendHandler() (http.Handler, error) {
	content, err := fs.Sub(frontend, "web-dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(content)), nil
}

func main() {
	handler, err := frontendHandler()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", handler))
}
