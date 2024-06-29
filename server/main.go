package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

//go:embed html/*.html
var htmlDir embed.FS

//go:embed tenntenn.png
var pngimg []byte

func main() {
	http.HandleFunc("/img/ok/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL.String())
		r.ParseForm()
		delay, err := parseDelay(r.FormValue("d"))
		if err != nil {
			slog.Error("parse delay", "err", err)
			code := http.StatusInternalServerError
			http.Error(w, http.StatusText(code), code)
			return
		}
		fmt.Println("delay", delay)
		time.Sleep(delay)

		count, err := parseCount(r.FormValue("ng"))
		if err != nil {
			slog.Error("parse count", "err", err)
			code := http.StatusInternalServerError
			http.Error(w, http.StatusText(code), code)
			return
		}

		if count > 0 {
			if delay == 0 {
				delay = 60 * time.Second
			}
			time.Sleep(delay)
		}

		io.Copy(w, bytes.NewReader(pngimg))
	})
	http.HandleFunc("/img/ng/", func(w http.ResponseWriter, r *http.Request) {
		slog.Error("NG", "url", r.URL)
		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
	})
	http.Handle("/", http.FileServerFS(htmlDir))
	http.ListenAndServe(":8080", nil)
}

func parseDelay(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	delay, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}

	return delay, nil
}

func parseCount(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	count, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	return count, nil
}
