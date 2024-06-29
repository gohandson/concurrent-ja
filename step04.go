//go:build ignore

// Step04: sync.WaitGroupを使う
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var baseURL *url.URL

func init() {
	const baseURLStr = "http://localhost:8080/html/step04.html"
	_url, err := url.Parse(baseURLStr)
	if err != nil {
		panic(err)
	}
	baseURL = _url
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	srcs, err := fetchHTML(baseURL)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, src := range srcs {
		fmt.Println("download", src)
		wg.Add(1)
		go func() {
			download("imgs", src)
			fmt.Println("download done", src)
			wg.Done()
		}()
	}

	wg.Wait()

	return nil
}

func download(dir, dlurlStr string) error {

	dlurl := *baseURL
	if strings.HasPrefix(dlurlStr, "/") {
		path, query, ok := strings.Cut(dlurlStr, "?")
		dlurl.Path = path
		if ok {
			dlurl.RawQuery = query
		}
	} else {
		dlurl = *dlurl.JoinPath(dlurlStr)
	}

	req, err := http.NewRequest(http.MethodGet, dlurl.String(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	_, filename := path.Split(dlurl.Path)
	dlpath := filepath.Join(dir, filename)
	file, err := os.Create(dlpath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}

	return nil
}
