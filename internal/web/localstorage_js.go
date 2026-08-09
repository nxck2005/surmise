//go:build js && wasm

package web

import (
	"fmt"

	"syscall/js"

	"github.com/nxck2005/surmise/internal/store"
)

// LocalStorage is a store.KV over the browser's localStorage.
//
// Every call is wrapped, because localStorage throws rather than returning an
// error and an unrecovered panic in Go on WebAssembly kills the program. Two
// throws are ordinary rather than exceptional: Safari refuses access at all in
// private mode, and any browser raises QuotaExceededError once the origin's
// allowance is full. Neither should cost the player their game, so the first is
// answered by falling back to memory (see Storage) and the second by returning
// an error that reaches the UI's error line.
func LocalStorage() (kv store.KV, err error) {
	defer func() {
		if r := recover(); r != nil {
			kv, err = nil, fmt.Errorf("web: localStorage is unavailable: %v", r)
		}
	}()

	v := js.Global().Get("localStorage")
	if !v.Truthy() {
		return nil, fmt.Errorf("web: this browser has no localStorage")
	}
	// Touching it is the only way to find out whether it is usable: Safari's
	// private mode has the object and throws on use.
	v.Call("getItem", "")
	return localStorage{v}, nil
}

// Storage is the KV the browser build should use: localStorage when the browser
// allows it, and memory when it does not. A session that forgets everything on
// reload is a much better answer than a blank page.
func Storage() (store.KV, error) {
	kv, err := LocalStorage()
	if err != nil {
		return store.NewMemoryKV(), err
	}
	return kv, nil
}

type localStorage struct{ v js.Value }

func (l localStorage) Get(key string) (value string, ok bool) {
	defer func() {
		if recover() != nil {
			value, ok = "", false
		}
	}()
	got := l.v.Call("getItem", key)
	if !got.Truthy() {
		// A missing key is null, which is not an error.
		return "", false
	}
	return got.String(), true
}

func (l localStorage) Set(key, value string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Almost always QuotaExceededError. The caller reports it; nothing
			// here may panic its way out.
			err = fmt.Errorf("web: could not save: %v", r)
		}
	}()
	l.v.Call("setItem", key, value)
	return nil
}

func (l localStorage) Delete(key string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("web: could not delete: %v", r)
		}
	}()
	l.v.Call("removeItem", key)
	return nil
}

func (l localStorage) Keys() (keys []string) {
	defer func() {
		if recover() != nil {
			keys = nil
		}
	}()
	n := l.v.Get("length").Int()
	keys = make([]string, 0, n)
	for i := range n {
		k := l.v.Call("key", i)
		if k.Truthy() {
			keys = append(keys, k.String())
		}
	}
	return keys
}
