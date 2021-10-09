package ssr_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-duo/bud/bfs"
	"github.com/go-duo/bud/internal/npm"
	"github.com/go-duo/bud/internal/vfs"
	v8 "github.com/go-duo/bud/js/v8"
	"github.com/go-duo/bud/ssr"
	"github.com/go-duo/bud/svelte"
	"github.com/go-duo/bud/view"
	"github.com/matryer/is"
)

func TestSvelteHello(t *testing.T) {
	is := is.New(t)
	cwd, err := os.Getwd()
	is.NoErr(err)
	dir := filepath.Join(cwd, "_tmp")
	is.NoErr(os.RemoveAll(dir))
	defer func() {
		if !t.Failed() {
			is.NoErr(os.RemoveAll(dir))
		}
	}()
	vm := v8.New()
	memfs := vfs.Memory{
		"view/index.svelte": &fstest.MapFile{
			Data: []byte(`<h1>hi world</h1>`),
		},
	}
	is.NoErr(vfs.WriteAll(".", dir, memfs))
	dirfs := os.DirFS(dir)
	svelte := svelte.New(&svelte.Input{
		VM:  vm,
		Dev: true,
	})
	bf := bfs.New(dirfs)
	bf.Add(map[string]bfs.Generator{
		"bud/view/_ssr.js": ssr.Generator(dirfs, svelte, dir),
	})
	// Install svelte
	err = npm.Install(dir, "svelte@3.42.3")
	is.NoErr(err)
	// Read the wrapped version of index.svelte with node_modules rewritten
	code, err := fs.ReadFile(bf, "bud/view/_ssr.js")
	is.NoErr(err)
	is.True(strings.Contains(string(code), `create_ssr_component(`))
	is.True(strings.Contains(string(code), `<h1>hi world</h1>`))
	is.True(strings.Contains(string(code), `views["/"] = `))
	result, err := vm.Eval("render.js", string(code)+`; bud.render("/", {})`)
	is.NoErr(err)
	var res view.Response
	err = json.Unmarshal([]byte(result), &res)
	is.NoErr(err)
	is.Equal(res.Status, 200)
	is.Equal(len(res.Headers), 1)
	is.Equal(res.Headers["Content-Type"], "text/html")
	is.True(strings.Contains(res.Body, `<script id="bud_props" type="text/template" defer>{}</script>`))
	is.True(strings.Contains(res.Body, `<script type="module" src="/bud/view/_index.svelte" defer></script>`))
	is.True(strings.Contains(res.Body, `<div id="bud_target">`))
	is.True(strings.Contains(res.Body, `<h1>hi world</h1>`))
}
func TestSvelteAwait(t *testing.T) {
	is := is.New(t)
	cwd, err := os.Getwd()
	is.NoErr(err)
	dir := filepath.Join(cwd, "_tmp")
	is.NoErr(os.RemoveAll(dir))
	defer func() {
		if !t.Failed() {
			is.NoErr(os.RemoveAll(dir))
		}
	}()
	vm := v8.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("all good"))
	}))
	defer server.Close()
	memfs := vfs.Memory{
		"view/index.svelte": &fstest.MapFile{
			Data: []byte(`
			<script>
				let promise = fetch("` + server.URL + `").then(res => res.text())
			</script>

			<div>
				{#await promise}
					Loading...
				{:then value}
					response: {value}
				{/await}
			</div>
			`),
		},
	}
	is.NoErr(vfs.WriteAll(".", dir, memfs))
	dirfs := os.DirFS(dir)
	svelte := svelte.New(&svelte.Input{
		VM:  vm,
		Dev: true,
	})
	bf := bfs.New(dirfs)
	bf.Add(map[string]bfs.Generator{
		"bud/view/_ssr.js": ssr.Generator(dirfs, svelte, dir),
	})
	// Install svelte
	err = npm.Install(dir, "svelte@3.42.3")
	is.NoErr(err)
	// Read the wrapped version of index.svelte with node_modules rewritten
	code, err := fs.ReadFile(bf, "bud/view/_ssr.js")
	is.NoErr(err)
	result, err := vm.Eval("render.js", string(code)+`; bud.render("/", {})`)
	is.NoErr(err)
	var res view.Response
	err = json.Unmarshal([]byte(result), &res)
	is.NoErr(err)
	is.Equal(res.Status, 200)
	is.Equal(len(res.Headers), 1)
	is.Equal(res.Headers["Content-Type"], "text/html")
	is.True(strings.Contains(res.Body, `<script id="bud_props" type="text/template" defer>{}</script>`))
	is.True(strings.Contains(res.Body, `<script type="module" src="/bud/view/_index.svelte" defer></script>`))
	is.True(strings.Contains(res.Body, `<div id="bud_target">`))
	is.True(strings.Contains(res.Body, `Loading...`))
}