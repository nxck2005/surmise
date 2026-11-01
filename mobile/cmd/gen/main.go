// Command gen writes the Kotlin side of the engine's wire format:
//
//	android/core/src/main/java/com/nxck/surmise/core/gen/Api.kt
//
// It reflects over the engine's method registry, so a Go request or response
// type and its Kotlin data class cannot drift apart: one data class per Go
// struct the methods reach, with the Go JSON names kept, and a CoreClient with
// one suspend fun per method. Run it from mobile/ after any change to an
// engine type (GOWORK=off go run ./cmd/gen); CI fails when the committed file
// differs.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/nxck2005/surmise/mobile/engine"
)

const defaultOut = "../android/core/src/main/java/com/nxck/surmise/core/gen/Api.kt"

func main() {
	log.SetFlags(0)
	out := flag.String("out", defaultOut, "the Kotlin file to write")
	flag.Parse()

	src, err := generate(engine.Methods(), engine.WireTypes())
	if err != nil {
		log.Fatalf("gen: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatal(err)
	}
}

var (
	timeType  = reflect.TypeFor[time.Time]()
	emptyType = reflect.TypeFor[engine.Empty]()
)

// gen collects the structs the methods reach, by Kotlin name.
type gen struct {
	structs map[string]reflect.Type
}

func generate(methods []engine.Method, extra []reflect.Type) ([]byte, error) {
	g := &gen{structs: map[string]reflect.Type{}}
	for _, m := range methods {
		for _, t := range []reflect.Type{m.Req, m.Resp} {
			if _, err := g.kotlinType(t); err != nil {
				return nil, fmt.Errorf("%s: %w", m.Name, err)
			}
		}
	}
	for _, t := range extra {
		if _, err := g.kotlinType(t); err != nil {
			return nil, err
		}
	}

	var b bytes.Buffer
	b.WriteString(header)

	names := make([]string, 0, len(g.structs))
	for n := range g.structs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := g.writeStruct(&b, n, g.structs[n]); err != nil {
			return nil, err
		}
	}

	b.WriteString(clientHead)
	for _, m := range methods {
		g.writeMethod(&b, m)
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}

// kotlinType is the Kotlin spelling of a Go type, recording every struct it
// reaches.
func (g *gen) kotlinType(t reflect.Type) (string, error) {
	switch {
	case t == timeType:
		return "String", nil
	case t == emptyType:
		return "Unit", nil
	}
	switch t.Kind() {
	case reflect.String:
		return "String", nil
	case reflect.Bool:
		return "Boolean", nil
	case reflect.Int, reflect.Int32:
		return "Int", nil
	case reflect.Int64:
		return "Long", nil
	case reflect.Float64:
		return "Double", nil
	case reflect.Pointer:
		inner, err := g.kotlinType(t.Elem())
		return inner + "?", err
	case reflect.Slice:
		inner, err := g.kotlinType(t.Elem())
		return "List<" + inner + ">", err
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return "", fmt.Errorf("map key %s: only string keys cross to Kotlin", t.Key())
		}
		inner, err := g.kotlinType(t.Elem())
		return "Map<String, " + inner + ">", err
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			return "", fmt.Errorf("anonymous struct %s: name it", t)
		}
		if seen, ok := g.structs[name]; ok {
			if seen != t {
				return "", fmt.Errorf("two structs named %s (%s and %s): rename one", name, seen.PkgPath(), t.PkgPath())
			}
			return name, nil
		}
		g.structs[name] = t
		for _, f := range fields(t) {
			if _, err := g.kotlinType(f.typ); err != nil {
				return "", fmt.Errorf("%s.%s: %w", name, f.goName, err)
			}
		}
		return name, nil
	}
	return "", fmt.Errorf("type %s (%s) has no Kotlin form", t, t.Kind())
}

type field struct {
	goName, wire string
	typ          reflect.Type
}

// fields lists a struct's JSON fields in declaration order, as encoding/json
// would write them.
func fields(t reflect.Type) []field {
	var out []field
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		wire := f.Name
		if tag, ok := f.Tag.Lookup("json"); ok {
			name, _, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if name != "" {
				wire = name
			}
		}
		out = append(out, field{goName: f.Name, wire: wire, typ: f.Type})
	}
	return out
}

// defaultFor is the value a field takes when the JSON leaves it out, which is
// what an omitempty Go field does with its zero value.
func (g *gen) defaultFor(t reflect.Type) string {
	if t == timeType {
		return `""`
	}
	switch t.Kind() {
	case reflect.String:
		return `""`
	case reflect.Bool:
		return "false"
	case reflect.Int, reflect.Int32:
		return "0"
	case reflect.Int64:
		return "0L"
	case reflect.Float64:
		return "0.0"
	case reflect.Pointer:
		return "null"
	case reflect.Slice:
		return "emptyList()"
	case reflect.Map:
		return "emptyMap()"
	case reflect.Struct:
		return t.Name() + "()"
	}
	return ""
}

func (g *gen) writeStruct(b *bytes.Buffer, name string, t reflect.Type) error {
	fs := fields(t)
	if len(fs) == 0 {
		return nil // Unit on the Kotlin side
	}
	fmt.Fprintf(b, "@Serializable\ndata class %s(\n", name)
	for _, f := range fs {
		kt, err := g.kotlinType(f.typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "    @SerialName(%q) val %s: %s = %s,\n", f.wire, property(f.wire), kt, g.defaultFor(f.typ))
	}
	b.WriteString(")\n\n")
	return nil
}

// property is a JSON name as a Kotlin property: snake_case becomes camelCase,
// and a Kotlin keyword is quoted.
func property(wire string) string {
	parts := strings.Split(wire, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	p := strings.Join(parts, "")
	switch p {
	case "as", "break", "class", "continue", "do", "else", "false", "for", "fun", "if", "in",
		"interface", "is", "null", "object", "package", "return", "super", "this", "throw",
		"true", "try", "typealias", "typeof", "val", "var", "when", "while":
		return "`" + p + "`"
	}
	return p
}

// funcName is a method name as a Kotlin function: board.submit → boardSubmit.
func funcName(method string) string {
	parts := strings.Split(method, ".")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func (g *gen) writeMethod(b *bytes.Buffer, m engine.Method) {
	resp, _ := g.kotlinType(m.Resp)
	decode := resp + ".serializer()"
	if resp == "Unit" {
		decode = "null"
	}
	fmt.Fprintf(b, "\n    /** %s */\n", m.Name)
	if m.Req == emptyType {
		fmt.Fprintf(b, "    suspend fun %s(): %s =\n        call(%q, \"{}\", %s)\n", funcName(m.Name), resp, m.Name, decode)
		return
	}
	req, _ := g.kotlinType(m.Req)
	fmt.Fprintf(b, "    suspend fun %s(request: %s): %s =\n        call(%q, json.encodeToString(%s.serializer(), request), %s)\n",
		funcName(m.Name), req, resp, m.Name, req, decode)
}

const header = `// Code generated by mobile/cmd/gen from the engine's method registry. DO NOT EDIT.
//
// Regenerate from mobile/ with: GOWORK=off go run ./cmd/gen

package com.nxck.surmise.core.gen

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

`

const clientHead = `/** A refusal from the engine: a code the app maps to its own words, never shown raw. */
class CoreException(val code: String, val detail: String) : Exception("$code: $detail")

/** The JSON settings the wire uses: a field left at its default is left out. */
val coreJson = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
}

/**
 * Reads an engine envelope: the "ok" value, or a CoreException for an
 * {"err": …} answer. A null serializer is a method that answers nothing.
 */
@Suppress("UNCHECKED_CAST")
fun <T> decodeEnvelope(text: String, serializer: KSerializer<T>?): T {
    val envelope = coreJson.parseToJsonElement(text).jsonObject
    envelope["err"]?.let { err ->
        val o = err.jsonObject
        throw CoreException(
            o["code"]?.jsonPrimitive?.content ?: "internal",
            o["detail"]?.jsonPrimitive?.content ?: "",
        )
    }
    if (serializer == null) return Unit as T
    return coreJson.decodeFromJsonElement(serializer, envelope["ok"] ?: JsonNull)
}

/**
 * The engine's methods, typed. raw is Engine.call: a method name and a JSON
 * request in, a JSON envelope out. Every call throws CoreException for an
 * {"err": …} answer.
 */
class CoreClient(private val raw: suspend (method: String, json: String) -> String) {
    private val json = coreJson

    private suspend fun <T> call(method: String, body: String, serializer: KSerializer<T>?): T =
        decodeEnvelope(raw(method, body), serializer)
`
