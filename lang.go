package zweb

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type LangEngine struct {
	LangDir         string                       // language directory, default lang/
	DefaultLang     string                       //default language, default en
	DisableAutoSync bool                         //disable auto sync language files
	langs           map[string]map[string]string // language map, e.g. {"en": {"hello": "Hello"}}
	needSync        bool                         //need sync language files
}

func NewLangEngine(cfg Config) *LangEngine {
	z := &LangEngine{
		LangDir:         cfg.LangDir,
		DisableAutoSync: cfg.DisableLangAutoSync,
		DefaultLang:     cfg.DefaultLang,
		langs:           make(map[string]map[string]string),
	}
	if z.LangDir == "" {
		z.LangDir = "lang"
	}
	if z.DefaultLang == "" {
		z.DefaultLang = "en"
	}
	return z
}
func (l *LangEngine) getDefaultLang() string {
	if l.DefaultLang == "" {
		return "en"
	}
	return l.DefaultLang
}

// method loadLangs
func (l *LangEngine) loadLangs() error {
	info, e := os.Stat(l.LangDir)
	if e != nil || !info.IsDir() {
		return nil
	}

	fs, e := os.ReadDir(l.LangDir)
	if e != nil {
		log.Println(e)
		return e
	}
	for _, f := range fs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(f.Name(), ".json")

		content, e := os.ReadFile(filepath.Join(l.LangDir, f.Name()))
		if e != nil {
			log.Println(e)
			return e
		}

		m := make(map[string]string)
		e = json.Unmarshal(content, &m)
		if e != nil {
			log.Println(e)
			return e
		}

		l.langs[lang] = m
	}
	return nil
}

// method run
func (l *LangEngine) Run() error {
	e := l.loadLangs()
	if e != nil {
		log.Println(e)
		return e
	}

	return nil
}

// method s
func (l *LangEngine) generateS(r *http.Request) func(key string) string {
	l.loadLangs()
	lang := l.getDefaultLang()
	s := SubAfter(r.URL.Path, "/", r.URL.Path)
	s = SubBefore(s, "/", "")
	if s != "" {
		if _, ok := l.langs[s]; ok {
			lang = s
		}
	}

	return func(key string) string {
		if l.langs[lang] != nil {
			if v, ok := l.langs[lang][key]; ok {
				return v
			}
		}
		if !l.DisableAutoSync {
			if l.langs[lang] == nil {
				l.langs[lang] = make(map[string]string)
			}
			l.langs[lang][key] = toUpperCamelCase(key)
			l.needSync = true
		}
		if lang != l.getDefaultLang() && l.langs[l.getDefaultLang()] != nil {
			v, ok := l.langs[l.getDefaultLang()][key]
			if ok {
				l.langs[lang][key] = v
				return v
			}
		}
		return toUpperCamelCase(key)
	}
}

// method TrySync
func (l *LangEngine) TrySync() error {
	if l.needSync {
		for k, m := range l.langs {
			b, e := json.MarshalIndent(m, "", "\t")
			if e != nil {
				log.Println(e)
				return e
			}

			os.MkdirAll(l.LangDir, 0755)
			e = os.WriteFile(filepath.Join(l.LangDir, k+".json"), b, 0644)
			if e != nil {
				log.Println(e)
				return e
			}
		}
	}
	return nil
}

// method get relative path of req.URL
func (l *LangEngine) getRelPath(path string) string {
	s := SubAfter(path, "/", path)
	s = SubBefore(s, "/", "")
	if s != "" {
		if _, ok := l.langs[s]; ok {
			return strings.TrimPrefix(path, "/"+s)
		}
	}

	return path
}

// method get relative path of req.URL
func (l *LangEngine) getLangOf(r *http.Request) string {
	s := SubAfter(r.URL.Path, "/", r.URL.Path)
	s = SubBefore(s, "/", "")
	if s != "" {
		if _, ok := l.langs[s]; ok {
			return s
		}
	}

	return l.getDefaultLang()
}

// method absPath
func (l *LangEngine) generateAbsPathOf(r *http.Request) func(string) string {
	s := SubAfter(r.URL.Path, "/", r.URL.Path)
	s = SubBefore(s, "/", "")
	if s != "" {
		if _, ok := l.langs[s]; ok {
			return func(key string) string {
				return "/" + s + key
			}
		}
	}
	return func(s string) string {
		return s
	}
}
