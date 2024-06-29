package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
)

func fetchHTML(htmlURL *url.URL) ([]string, error) {
	resp, err := http.Get(htmlURL.String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch HTML %s: invalid status %d", htmlURL.String(), resp.StatusCode)
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var srcs []string
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if ok {
			srcs = append(srcs, src)
		}
	})

	return srcs, nil
}
