// Step10: リトライする
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
	"strconv"
	"strings"
	"time"

	"github.com/lestrrat-go/backoff/v2"
	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
)

var baseURL *url.URL

func init() {
	const baseURLStr = "http://localhost:8080/html/step10.html"
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

	dl := &Downloader{
		Dir: "imgs",
	}

	// .WithFirstError()とするとerrgroupと同じ挙動になる
	p := pool.New().WithContext(ctx).WithMaxGoroutines(2)
	for _, src := range srcs {
		p.Go(func(ctx context.Context) error {
			fmt.Println("download start", src)
			defer fmt.Println("download done", src)

			var rerr error
			recovered := panics.Try(func() {
				if err := dl.Do(ctx, src); err != nil {
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

type Downloader struct {
	Dir string
}

func (d *Downloader) Do(ctx context.Context, dlurlStr string) error {

	p := backoff.Exponential(
		backoff.WithMaxRetries(3),
		backoff.WithMinInterval(time.Second),
		backoff.WithMaxInterval(time.Minute),
		backoff.WithJitterFactor(0.05),
	)

	cause := errors.New("5秒以上かかったのでエラー")
	dlurl := d.toURL(dlurlStr)

	b := p.Start(ctx)
	for backoff.Continue(b) {
		err := d.doWithTimeout(ctx, dlurl, cause)
		switch {
		case err == nil: // 成功
			return nil
		case errors.Is(err, cause): // リトライ
			fmt.Println("retry")
			continue
		case err != nil:
			return err
		}
	}

	return fmt.Errorf("ダウンロードに失敗しました: %s", dlurlStr)
}

func (d *Downloader) toURL(dlurlStr string) *url.URL {
	dlurl := *baseURL
	if strings.HasPrefix(dlurlStr, "/") {
		path, query, ok := strings.Cut(dlurlStr, "?")
		dlurl.Path = path
		if ok {
			values, err := url.ParseQuery(query)
			if err != nil {
				return &dlurl
			}

			if values.Has("panic") {
				panic("panic " + dlurlStr)
			}

			dlurl.RawQuery = query
		}
		fmt.Println(dlurl.String())
		return &dlurl
	}

	return dlurl.JoinPath(dlurlStr)
}

func (d *Downloader) countdownNG(dlurl *url.URL) {
	values := dlurl.Query()
	if strNG := values.Get("ng"); strNG != "" {
		ng, err := strconv.Atoi(strNG)
		if err == nil {
			ng--
			if ng >= 0 {
				values.Set("ng", strconv.Itoa(ng))
			} else {
				values.Del("ng")
			}
			dlurl.RawQuery = values.Encode()
		}
	}
}

// 5秒まってもダウンロードできない場合はエラーにする
func (d *Downloader) doWithTimeout(ctx context.Context, dlurl *url.URL, cause error) error {
	ctx, cancel := context.WithTimeoutCause(ctx, 5*time.Second, cause)
	defer cancel()

	return d.do(ctx, dlurl)
}

func (d *Downloader) do(ctx context.Context, dlurl *url.URL) error {

	d.countdownNG(dlurl)
	fmt.Println("countdown:", dlurl)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlurl.String(), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if cause := context.Cause(ctx); cause != nil {
		return cause
	} else if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("error with " + http.StatusText(resp.StatusCode))
	}

	defer resp.Body.Close()

	_, filename := path.Split(dlurl.Path)
	dlpath := filepath.Join(d.Dir, filename)
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
