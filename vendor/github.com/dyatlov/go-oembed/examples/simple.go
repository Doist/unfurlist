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
			info, err := item.FetchOembedWithLocale(url, nil, "en-us")
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
