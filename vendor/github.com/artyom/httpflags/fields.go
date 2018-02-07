// Package httpflags provides a conveniet way of filling struct fields from http
// request form values. Exposed struct fields should have special `flag` tag
// attached:
//
//	args := struct {
//		Name    string `flag:"name"`
//		Age     uint   `flag:"age"`
//		Married bool   // this won't be exposed
//	}{
// 		// default values
// 		Name: "John Doe",
// 		Age:  34,
//	}
//
// After declaring flags and their default values as above, call
// httpflags.Parse() inside http.Handler to fill struct fields from http request
// form values:
//
// 	func myHandler(w http.ResponseWriter, r *http.Request) {
//		args := struct {
// 			...
// 		}{}
// 		if err := httpflags.Parse(&args, r) ; err != nil {
//			http.Error(w, "Bad request", http.StatusBadRequest)
//			return
// 		}
//		// use args fields here
//
// Parse() calls ParseForm method of http.Request automatically, so it would
// understand data both from the URL field's query parameters and the POST or
// PUT form.
//
// Package httpflags supports all basic types supported by xxxVar functions from
// standard library flag package: int, int64, uint, uint64, float64, bool,
// string, time.Duration as well as types implementing flag.Value interface.
// Parse would panic on non-empty `flag` tag on unsupported type field.
package httpflags

import (
	"flag"
	"io/ioutil"
	"net/http"

	"github.com/artyom/autoflags"
)

// Parse fills dst struct with values extracted from r.Form. dst should be
// a non-nil pointer to struct having its exported attributes tagged with 'flag'
// tag — see autoflags package documentation. r.ParseForm is called
// automatically. Only the first value of each key from r.Form is used.
func Parse(dst interface{}, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return err
	}
	fs := new(flag.FlagSet)
	fs.SetOutput(ioutil.Discard)
	autoflags.DefineFlagSet(fs, dst)
	args := make([]string, 0, len(r.Form))
	for k := range r.Form {
		args = append(args, "-"+k+"="+r.Form.Get(k))
	}
	return fs.Parse(args)
}
