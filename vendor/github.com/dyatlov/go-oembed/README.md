GO Oembed
===

Go Oembed provides methods to retrieve oembed data from known providers.
Provider list can be fetched from this repository or from http://oembed.com/providers.json

To download and install this package run:

`go get github.com/dyatlov/go-oembed/oembed`

Source docs: http://godoc.org/github.com/dyatlov/go-oembed

An example use:

```go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dyatlov/go-oembed/oembed"
)

func main() {
	data, err := ioutil.ReadFile("../providers.json")

	if err != nil {
		panic(err)
	}

	oe := oembed.NewOembed()
	oe.ParseProviders(bytes.NewReader(data))

	for {

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter url: ")
		url, _ := reader.ReadString('\n')

		url = strings.Trim(url, "\r\n")

		if url == "" {
			break
		}

		item := oe.FindItem(url)

		if item != nil {
			info, err := item.FetchOembed(url, nil)
			if err != nil {
				fmt.Printf("An error occured: %s\n", err.Error())
			} else {
				if info.Status >= 300 {
					fmt.Printf("Response status code is: %d\n", info.Status)
				} else {
					fmt.Printf("Oembed info:\n%s\n", info)
				}
			}
		} else {
			fmt.Println("nothing found :(")
		}

	}
}
```
