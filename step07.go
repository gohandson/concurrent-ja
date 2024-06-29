//go:build ignore

// Step07: goroutineの数を制限する
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var baseURL *url.URL

func init() {
	const baseURLStr = "http://localhost:8080/html/step07.html"
	_url, err := url.Parse(baseURLStr)
	if err != nil {
		panic(err)
	}
	baseURL = _url
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	srcs, err := fetchHTML(baseURL)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("1s経過")
			case <-done:
				return
			}
		}
	}()
	defer func() {
		ticker.Stop()
		close(done)
	}()

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(2) // 一度に送るリクエストの数を2にする
	for _, src := range srcs {
		eg.Go(func() error {
			fmt.Println("download start", src)
			defer fmt.Println("download done", src)

			// 5秒まってもダウンロードできない場合はエラーにする
			cause := errors.New("5秒以上かかったのでエラー")
			ctx, cancel := context.WithTimeoutCause(ctx, 5*time.Second, cause)
			defer cancel()

			if err := download(ctx, "imgs", src); err != nil {
				return err
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func download(ctx context.Context, dir, dlurlStr string) error {

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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlurl.String(), nil)
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
