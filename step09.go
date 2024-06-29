//go:build ignore

// Step09: conc/panicsを使う
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

	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
)

var baseURL *url.URL

func init() {
	const baseURLStr = "http://localhost:8080/html/step09.html"
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

func run(ctx context.Context) (rerr error) {
	defer func() {
		if p := recover(); p != nil {
			rerr = fmt.Errorf("panic:%v", p)
		}
	}()

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

	p := pool.New().WithContext(ctx).WithMaxGoroutines(2)
	for _, src := range srcs {
		p.Go(func(ctx context.Context) error {
			fmt.Println("download start", src)
			defer fmt.Println("download done", src)

			var rerr error
			recovered := panics.Try(func() {

				// 5秒まってもダウンロードできない場合はエラーにする
				cause := errors.New("5秒以上かかったのでエラー")
				ctx, cancel := context.WithTimeoutCause(ctx, 5*time.Second, cause)
				defer cancel()

				if err := download(ctx, "imgs", src); err != nil {
					rerr = err
				}
			})

			switch {
			case recovered != nil:
				return fmt.Errorf("recovered: %w", recovered.AsError())
			case rerr != nil:
				return rerr
			}

			return nil
		})
	}

	if err := p.Wait(); err != nil {
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
			if strings.Contains(query, "panic") {
				panic("panic " + dlurlStr)
			}
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
