package main

import (
	"flag"
	"fmt"
	"github.com/mattn/go-iconv"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var encodings = []string{
	"iso-2022-jp-3",
	"iso-2022-jp",
	"euc-jisx0213",
	"euc-jp",
	"utf-8",
	"ucs-bom",
	"euc-jp",
	"eucjp-ms",
	"cp932",
}

type grepper struct {
	pattern string
	re      *regexp.Regexp
	oc      *iconv.Iconv
}

func (v *grepper) VisitDir(dir string, f *os.FileInfo) bool {
	if dir == "." {
		return true
	}
	dirmask, _ := filepath.Split(v.pattern)
	dir = filepath.ToSlash(dir)

	mi := strings.Split(dirmask, "/")
	if len(mi) == 2 && mi[0] == "**" {
		if m, e := filepath.Match(dirmask, dir); e != nil || m == false {
			return true
		}
	}
	for i, d := range strings.Split(dir, "/") {
		if len(mi) <= i {
			break
		}
		if m, e := filepath.Match(mi[i], d); e != nil || m == false {
			return false
		}
	}

	return true
}

func (v *grepper) VisitFile(path string, f *os.FileInfo) {
	dirmask, filemask := filepath.Split(v.pattern)
	dir, file := filepath.Split(path)

	dirmask = filepath.ToSlash(dirmask)
	dir = filepath.ToSlash(dir)

	dm, e := filepath.Match(dirmask, dir)
	if e != nil {
		return
	}
	fm, e := filepath.Match(filemask, file)
	if e != nil {
		return
	}
	if dm && fm {
		v.Grep(filepath.ToSlash(path))
	}
}

func (v *grepper) Grep(input interface{}) {
	var f []byte
	var path = ""
	var ok bool
	var stdin *os.File
	var err os.Error

	if path, ok = input.(string); ok {
		f, err = ioutil.ReadFile(path)
		if err != nil {
			return
		}
	} else if stdin, ok = input.(*os.File); ok {
		f, err = ioutil.ReadAll(stdin)
		if err != nil {
			return
		}
		path = "stdin"
	}
	for _, enc := range encodings {
		ic, err := iconv.Open("utf-8", enc)
		if err != nil {
			continue
		}
		did := false
		for n, line := range strings.Split(string(f), "\n") {
			t, err := ic.Conv(line)
			if err != nil {
				break
			}
			bs := v.re.FindAll([]byte(t), -1)
			if len(bs) == 0 {
				continue
			}
			o, err := v.oc.Conv(t)
			if err != nil {
				o = line
			}
			fmt.Printf("%s:%d:%s\n", path, n+1, o)
			did = true
		}
		ic.Close()
		if did {
			break
		}
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: gogrep [pattern] [file...]\n")
		flag.PrintDefaults()
		os.Exit(-1)
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
	}
	re, err := regexp.Compile(flag.Arg(0))
	if err != nil {
		println(err.String())
		os.Exit(-1)
	}
	oc, err := iconv.Open("char", "utf-8")
	if err != nil {
		oc, err = iconv.Open("utf-8", "utf-8")
	}
	defer func() {
		if oc != nil {
			oc.Close()
		}
	}()

	if flag.NArg() == 1 {
		g := &grepper{"", re, oc}
		g.Grep(os.Stdin)
	} else {
		for _, arg := range flag.Args()[1:] {
			g := &grepper{filepath.ToSlash(arg), re, oc}

			root := ""
			for _, i := range strings.Split(g.pattern, "/") {
				if strings.Index(i, "*") != -1 {
					break
				}
				if syscall.OS == "windows" && len(i) == 2 && filepath.VolumeName(i) != "" {
					root = i + "/"
				} else {
					root = filepath.Join(root, i)
				}
			}
			if arg != root {
				if root == "" {
					root = "."
				} else {
					root += "/"
				}
			}

			filepath.Walk(root, g, nil)
		}
	}
}
