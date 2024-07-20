package zweb

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type (
	ZWeb struct {
		cfg        Config
		langEngine *LangEngine
		addr       string
	}
	Config struct {
		Dir                 string   //target directory, default src/
		OutDir              string   //output directory, default docs/
		LangDir             string   // language directory, default lang/
		TemplateExt         []string // template extension, default .html
		DisableLangAutoSync bool     //disable auto sync language files, default false
		DefaultLang         string   //default language, default en
	}
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func New(cfg ...Config) *ZWeb {
	z := &ZWeb{}
	if len(cfg) > 0 {
		z.cfg = cfg[0]
	}

	//init
	if z.cfg.Dir == "" {
		z.cfg.Dir = "src"
	}
	if z.cfg.OutDir == "" {
		z.cfg.OutDir = "docs"
	}
	z.langEngine = NewLangEngine(z.cfg)
	return z
}

// method isTemplateExt
func (z *ZWeb) isTemplateExt(ext string) bool {
	if len(z.cfg.TemplateExt) == 0 {
		z.cfg.TemplateExt = []string{".html"}
	}
	for _, e := range z.cfg.TemplateExt {
		if e == ext {
			return true
		}
	}
	return false
}

// method loadtemplates
func (z *ZWeb) loadTemplates(r *http.Request) (*template.Template, error) {
	var toParse []string
	e := filepath.WalkDir(z.cfg.Dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if z.isTemplateExt(filepath.Ext(path)) {
			toParse = append(toParse, path)
		}
		return nil
	})
	if e != nil {
		log.Println(e)
		return nil, e
	}

	t := template.New("zweb template")
	//funcs
	t = t.Funcs(template.FuncMap{
		"s":          z.langEngine.generateS(r),
		"absPathOf":  z.langEngine.generateAbsPathOf(r),
		"relPath":    z.langEngine.getRelPath,
		"mdToHtml":   mdToHTML,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"loadMdList": loadMdList,
		"trimSuffix": strings.TrimSuffix,
		"trimPrefix": strings.TrimPrefix,
	})
	for _, path := range toParse {
		rel, e := filepath.Rel(z.cfg.Dir, path)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		println(rel)
		bs, e := os.ReadFile(path)
		if e != nil {
			log.Println(e)
			return nil, e
		}
		t, e = t.New(rel).Parse(string(bs))
		if e != nil {
			log.Println(e)
			return nil, e
		}
	}

	return t, nil
}

// method Run
func (z *ZWeb) Run() error {
	return z.run(false)
}
func (z *ZWeb) run(randomPort bool) error {
	e := z.langEngine.Run()
	if e != nil {
		log.Println(e)
		return e
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path
		if strings.HasSuffix(name, "/") {
			name += "index.html"
		}
		name = name[1:]

		s := SubBefore(name, "/", "")
		if s != "" {
			if _, ok := z.langEngine.langs[s]; ok {
				name = strings.TrimPrefix(name, s)
				name = strings.TrimPrefix(name, "/")
			}
		}

		if z.isTemplateExt(filepath.Ext(name)) {
			t, e := z.loadTemplates(r)
			if e != nil {
				log.Println(e)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}

			dynamicFile, pathParams, e := parsePathParams(z.cfg.Dir, name)
			if e != nil {
				log.Println(e)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}
			if dynamicFile != "" {
				name, e = filepath.Rel(z.cfg.Dir, dynamicFile)
				if e != nil {
					log.Println(e)
					http.Error(w, e.Error(), http.StatusInternalServerError)
					return
				}
			}
			e = t.ExecuteTemplate(w, name, map[string]any{
				"Req":        r,
				"RelPath":    z.langEngine.getRelPath(r.URL.Path),
				"Lang":       z.langEngine.getLangOf(r),
				"PathParams": pathParams,
			})
			if e != nil {
				log.Println(e)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}
			z.langEngine.TrySync()
			return
		}
		http.ServeFile(w, r, filepath.Join(z.cfg.Dir, name))
	})

	port := 8080
	if randomPort {
		port = rand.Intn(1000) + 8080
	}
	z.addr = ":" + strconv.Itoa(port)
	fmt.Println("running on http://localhost" + z.addr)
	e = http.ListenAndServe(z.addr, nil)
	if e != nil {
		log.Panic(e)
		return e
	}
	return nil
}

// method export
func (z *ZWeb) Export() error {
	go z.run(true)
	os.RemoveAll(z.cfg.OutDir)
	time.Sleep(1 * time.Second)
	e := filepath.WalkDir(z.cfg.Dir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") || strings.Contains(path, "_data/") {
			return nil
		}
		rel, e := filepath.Rel(z.cfg.Dir, path)
		if e != nil {
			log.Println(e)
			return e
		}

		if strings.Contains(rel, "[") {
			dir := filepath.Dir(path)
			fs, e := os.ReadDir(filepath.Join(dir, "_data"))
			if e != nil {
				log.Println(e)
				return e
			}
			for _, f := range fs {
				name := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
				rel = filepath.Join(filepath.Dir(rel), name+filepath.Ext(rel))
				dst := filepath.Join(z.cfg.OutDir, rel)
				println(rel)

				e = downloadToWithMinifier(dst, "http://localhost"+z.addr+"/"+strings.TrimSuffix(rel, "index.html"))
				if e != nil {
					log.Println(e)
					return e
				}
			}
			return nil
		}

		println(rel)
		dst := filepath.Join(z.cfg.OutDir, rel)
		e = downloadToWithMinifier(dst, "http://localhost"+z.addr+"/"+strings.TrimSuffix(rel, "index.html"))
		if e != nil {
			log.Println(e)
			return e
		}

		// other languages
		if z.isTemplateExt(filepath.Ext(path)) {
			for lang := range z.langEngine.langs {
				dst = filepath.Join(z.cfg.OutDir, lang, rel)
				e = downloadToWithMinifier(dst, "http://localhost"+z.addr+"/"+lang+"/"+strings.TrimSuffix(rel, "index.html"))
				if e != nil {
					log.Println(e)
					return e
				}
			}
		}
		return nil
	})
	if e != nil {
		log.Println(e)
		return e
	}
	return nil
}
