package httpflags

import (
	"fmt"
	"net/http/httptest"
)

func ExampleParse() {
	args := &struct {
		Name  string `flag:"name"`
		Age   int    `flag:"age"`
		Extra bool   `flag:"extra"`
	}{
		// default values:
		Age:   42,
		Extra: true,
	}
	req := httptest.NewRequest("GET", "/?name=John%20Doe&extra=false", nil)
	if err := Parse(args, req); err != nil {
		fmt.Println(err)
	}
	fmt.Printf("updated args: %+v\n", args)

	req = httptest.NewRequest("GET", "/?badField=boom", nil)
	fmt.Println("request with wrong field:", Parse(args, req))
	// Output:
	// updated args: &{Name:John Doe Age:42 Extra:false}
	// request with wrong field: flag provided but not defined: -badField
}
