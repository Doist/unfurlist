//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Doist/unfurlist/internal/useragent"
)

var urls = []string{
	"http://techcrunch.com/2015/11/09/basic-income-createathon/",
	"https://news.ycombinator.com/",
	"https://twitter.com/amix3k/status/1399300280206909440",
	"http://news.chosun.com/site/data/html_dir/2009/09/24/2009092401755.html",
}

func main() {
	outfile := filepath.FromSlash("testdata/remote-dump.json")
	flag.StringVar(&outfile, "f", outfile, "`file` to save data to (will be overwritten)")
	flag.Parse()
	if outfile == "" {
		log.Fatal("-f is empty")
	}
	data := make(map[string]string)

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: useragent.Set(&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}, "unfurlist (https://github.com/Doist/unfurlist)"),
	}

	for _, v := range urls {
		u, err := url.Parse(v)
		if err != nil {
			log.Fatal(v, err)
		}
		req, err := http.NewRequest(http.MethodGet, v, nil)
		if err != nil {
			log.Fatal(v, err)
		}
		r, err := httpClient.Do(req)
		if err != nil {
			log.Fatal(v, err)
		}
		if r.StatusCode >= 400 {
			log.Fatal(v, r.Status)
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatal(v, err)
		}
		r.Body.Close()
		// store key without scheme
		data[u.Host+u.RequestURI()] = string(b)
	}
	if err := dump(data, outfile); err != nil {
		log.Fatal(err)
	}
}

func dump(data map[string]string, name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return err
	}
	return f.Close()
}
