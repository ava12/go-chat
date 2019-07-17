package fserv

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Lister interface {
	List (w http.ResponseWriter, r *http.Request, dirs []string, files []os.FileInfo, hasParent bool)
}

type defaultLister struct {}

func emitHref (w http.ResponseWriter, prefix, name, display string, size int64) {
	var sizeText string
	prefix = html.EscapeString(prefix)
	name = html.EscapeString(name)
	display = html.EscapeString(display)
	if size < 0 {
		sizeText = "   &lt;DIR&gt;"
	} else {
		sizeText = fmt.Sprintf("%8d", size)
	}
	w.Write([]byte(fmt.Sprintf("%s  <a href=\"%s%s\">%s</a>\n", sizeText, prefix, name, display)))
}

func (defaultLister) List (w http.ResponseWriter, r *http.Request, dirs []string, files []os.FileInfo, hasParent bool) {
	prefix := r.RequestURI
	w.Write([]byte("<html><head><meta charset=\"UTF-8\"><style>pre{line-height:1.5em}</style></head><body><pre>\n"))
	var i int
	if hasParent {
		for i = len(prefix) - 2; i > 0; i -- {
			if prefix[i] == '/' {
				break
			}
		}
		emitHref(w, prefix[:i + 1], "", "..", -1)
	}
	for _, name := range dirs {
		emitHref(w, prefix, name, name, -1)
	}
	for _, info := range files {
		emitHref(w, prefix, info.Name(), info.Name(), info.Size())
	}
	w.Write([]byte("<hr></pre></body></html>\n"))
}

type handlerRec struct {
	lister Lister
	showDotted, listDirs bool
	urlPrefix, filePrefix string
}

func serveHttpError (w http.ResponseWriter, r *http.Request, code int, message string) {
	w.WriteHeader(code)
	log.Printf("%d %s (%s %s)", code, message, r.RequestURI, r.RemoteAddr)
}

func serveNotFound (w http.ResponseWriter, r *http.Request, message string) {
	serveHttpError(w, r, http.StatusNotFound, message)
}

func (h *handlerRec) ServeHTTP (w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, h.urlPrefix) {
		serveNotFound(w, r, "1")
		return
	}

	name := r.URL.Path[len(h.urlPrefix):]
	if !h.showDotted && name != "" && (name[0] == '.' || strings.Index(name, "/.") >= 0) {
		serveNotFound(w, r, "")
		return
	}

	fileName := filepath.Clean(h.filePrefix + name)
	info, e := os.Stat(fileName)
	if e != nil {
		serveNotFound(w, r, "")
		return
	}

	if !info.IsDir() {
		http.ServeFile(w, r, fileName)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/") {
		info, e = os.Stat(fileName + string(os.PathSeparator) + "index.html")
		if e == nil && !info.IsDir() {
			http.ServeFile(w, r, fileName + string(os.PathSeparator) + "index.html")
			return
		}
	}

	if !h.listDirs {
		serveNotFound(w, r, "")
		return
	}

	names, e := filepath.Glob(fileName + string(os.PathSeparator) + "*")
	if e != nil {
		serveNotFound(w, r, "")
		return
	}

	sort.Strings(names)
	dirs := make([]string, 0)
	files := make([]os.FileInfo, 0)
	for _, name := range names {
		info, e = os.Stat(name)
		if e != nil {
			continue
		}

		name = filepath.Base(name)
		if !h.showDotted && name[0] == '.' {
			continue
		}

		if info.IsDir() {
			dirs = append(dirs, info.Name())
		} else {
			files = append(files, info)
		}
	}

	hasParent := (name != "" && name[0] != '/' && name[0] != os.PathSeparator)
	h.lister.List(w, r, dirs, files, hasParent)
}

type Factory struct {
	Lister Lister
	ShowDotted bool
	ListDirs bool
}

func NewFactory () *Factory {
	return &Factory {defaultLister {}, false, false}
}

func (f *Factory) Make (urlPath, filePath string) http.Handler {
	if urlPath[len(urlPath) - 1] != '/' {
		urlPath += "/"
	}
	if filePath[len(filePath) - 1] != os.PathSeparator {
		filePath += string(os.PathSeparator)
	}

	return &handlerRec {f.Lister, f.ShowDotted, f.ListDirs, urlPath, filePath}
}
