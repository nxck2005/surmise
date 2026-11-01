package engine

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
)

// Empty is the request or response of a method that carries nothing.
type Empty struct{}

// Method is one registered call, with the Go types of its request and
// response so the generator can write the Kotlin side from them.
type Method struct {
	Name string
	Req  reflect.Type
	Resp reflect.Type
	call func(*Engine, []byte) (any, error)
}

var methods = map[string]Method{}

// register adds a typed handler. The request is decoded strictly: an unknown
// field is a bad request, so a Kotlin side that drifted from the Go one fails
// loudly in its first test instead of being quietly half-read.
func register[Req, Resp any](name string, fn func(*Engine, Req) (Resp, error)) {
	if _, dup := methods[name]; dup {
		panic("engine: method registered twice: " + name)
	}
	methods[name] = Method{
		Name: name,
		Req:  reflect.TypeFor[Req](),
		Resp: reflect.TypeFor[Resp](),
		call: func(e *Engine, raw []byte) (any, error) {
			var req Req
			if len(bytes.TrimSpace(raw)) > 0 {
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&req); err != nil {
					return nil, &Error{Code: CodeBadRequest, Detail: err.Error()}
				}
			}
			return fn(e, req)
		},
	}
}

// Methods lists every registered method, sorted by name.
func Methods() []Method {
	out := make([]Method, 0, len(methods))
	for _, m := range methods {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
