//go:build ignore

package main

import (
	"fmt"
	"net/url"
)

func main() {
	baseURL, _ := url.Parse("https://en.wikipedia.org/wiki/Cat")
	imgURL, _ := baseURL.Parse("//upload.wikimedia.org/wikipedia/commons/thumb/d/df/Hurricane.jpg")
	fmt.Printf("String: %s\n", imgURL.String())
}
