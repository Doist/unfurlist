See [godoc](https://godoc.org/github.com/artyom/useragent) for details.

Package useragent provides http.RoundTripper wrapper to set User-Agent
header on each http request made.

Basic usage:

    client := &http.Client{
    	Transport: useragent.Set(http.DefaultTransport, "MyRobot/1.0"),
    }
    resp, err := client.Get("https://...")
