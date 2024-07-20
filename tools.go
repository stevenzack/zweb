package zweb

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/tdewolff/minify/v2/minify"
)

func loadLangJson(file string) map[string]string {
	m := make(map[string]string)
	b, e := os.ReadFile(file)
	if e != nil {
		log.Panic(e)
		return nil
	}
	e = json.Unmarshal(b, &m)
	if e != nil {
		log.Panic(e)
		return nil
	}
	return m
}

func toUpperCamelCase(s string) string {
	if s == "" {
		return ""
	}
	v := s[0]
	if v >= 'a' && v <= 'z' {
		return strings.ToUpper(string(v)) + s[1:]
	}
	return s
}

func SubBefore(s string, sep, def string) string {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return s[:i]
		}
	}
	return def
}
func SubBeforeLast(s, sep, def string) string {
	for i := len(s) - len(sep); i > -1; i-- {
		if s[i:i+len(sep)] == sep {
			return s[:i]
		}
	}
	return def
}

func SubAfter(s, sep, def string) string {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return s[i+len(sep):]
		}
	}
	return def
}

func SubAfterLast(s, sep, def string) string {
	for i := len(s) - len(sep); i > -1; i-- {
		if s[i:i+len(sep)] == sep {
			return s[i+len(sep):]
		}
	}
	return def
}

func writeHtmlHeader(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

func downloadToWithMinifier(dst string, url string) error {
	res, e := http.Get(url)
	if e != nil {
		log.Println(e)
		return e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(res.Body)
	if e != nil {
		log.Println(e)
		return e
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("generate %s failed %d: %s", url, res.StatusCode, string(b))
	}

	os.MkdirAll(filepath.Dir(dst), 0755)
	e = os.WriteFile(dst, b, 0644)
	if e != nil {
		log.Println(e)
		return e
	}
	e = TryMinifyFile(dst)
	if e != nil {
		log.Println(e)
		return e
	}

	return nil
}

// minify
func TryMinifyFile(path string) error {
	ext := filepath.Ext(path)
	switch ext {
	case ".html", ".css", ".js":
	default:
		return nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		log.Println(e)
		return e
	}
	var s string
	switch filepath.Ext(path) {
	case ".html":
		s, e = minify.HTML(string(b))
	case ".css":
		s, e = minify.CSS(string(b))
	case ".js":
		s, e = minify.JS(string(b))
	}
	if e != nil {
		log.Println(e)
		return e
	}

	e = os.WriteFile(path, []byte(s), 0644)
	if e != nil {
		log.Println(e)
		return e
	}
	return nil
}

func parsePathParams(dir string, urlPath string) (string, map[string]any, error) {
	m := make(map[string]any)
	dst := filepath.Join(dir, strings.TrimPrefix(urlPath, "/"))
	if _, e := os.Stat(dst); e == nil {
		// already exists
		return "", m, nil
	}
	pwd := dir
	ss := strings.Split(urlPath, "/")

ROUTE_LOOP:
	for _, filename := range ss {
		if filename == "" {
			continue
		}
		info, e := os.Stat(filepath.Join(pwd, filename))
		if e != nil {
			fs, e := os.ReadDir(pwd)
			if e != nil {
				log.Panic(e)
				return "", nil, e
			}
			for _, f := range fs {
				if f.Name()[0] == '[' {
					key := SubBefore(f.Name()[1:], "]", "")
					suffix := SubAfter(f.Name(), "]", "")
					m[key] = strings.TrimSuffix(filename, suffix)
					pwd = filepath.Join(pwd, f.Name())
					continue ROUTE_LOOP
				}
			}
			// not found
			return "", nil, fmt.Errorf("route not found %s", urlPath)
		}

		if info.IsDir() {
			pwd = filepath.Join(pwd, filename)
			continue
		}
		break
	}
	return pwd, m, nil
}

func mdToHTML(mdFile string) template.HTML {
	md, e := os.ReadFile(mdFile)
	if e != nil {
		log.Println(e)
		return template.HTML(e.Error())
	}

	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	s := string(markdown.Render(doc, renderer))
	return template.HTML(s)
}

func loadMdList(dir string) []map[string]any {
	var list []map[string]any
	fs, e := os.ReadDir(dir)
	if e != nil {
		log.Println(e)
		return nil
	}
	for _, f := range fs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		b, e := os.ReadFile(filepath.Join(dir, f.Name()))
		if e != nil {
			log.Println(e)
			continue
		}

		m := make(map[string]any)
		m["filename"] = strings.TrimSuffix(f.Name(), ".md")

		s := SubBefore(string(b), "\n", "")
		m["title"] = strings.TrimPrefix(s, "# ")
		list = append(list, m)
	}
	return list
}
