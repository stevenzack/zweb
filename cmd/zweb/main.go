package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/stevenzack/zweb"
)

var (
	defaultLanguage     = flag.String("defaultLang", "en", "default language of the website")
	dir                 = flag.String("dir", "src", "source directory")
	outDir              = flag.String("out", "docs", "output directory")
	ext                 = flag.String("ext", "html", "template extension, multiple extensions separated by comma")
	disableLangAutoSync = flag.Bool("disableLangAutoSync", false, "disable auto sync language files")
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: zweb [run|export] [options]")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	c := zweb.Config{
		DefaultLang:         *defaultLanguage,
		Dir:                 *dir,
		OutDir:              *outDir,
		TemplateExt:         strings.Split(*ext, ","),
		DisableLangAutoSync: *disableLangAutoSync,
	}
	s := zweb.New(c)
	var e error
	switch flag.Arg(0) {
	case "run", "":
		fmt.Println("zweb run")
		e = s.Run()
	case "export":
		fmt.Println("zweb export")
		s.Export()
	}

	if e != nil {
		log.Fatal(e)
		return
	}
}
