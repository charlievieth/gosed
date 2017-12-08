package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/tools/imports"
)

type Replace struct {
	From, To string
}

type Walker struct {
	Reps         []Replace
	Modified     []string
	IncludeFakes bool
}

// type WalkFunc func(path string, info os.FileInfo, err error) error

func (w *Walker) skipDir(name string, fi os.FileInfo) error {
	if name == ".git" || name == "vendor" ||
		(!w.IncludeFakes && strings.Contains(name, "fake")) {
		return filepath.SkipDir
	}
	return nil
}

func (w *Walker) MatchFile(path string) bool {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}
	return w.containsReplacement(b)
}

func (w *Walker) containsReplacement(b []byte) bool {
	for _, r := range w.Reps {
		if bytes.Contains(b, []byte(r.From)) {
			return true
		}
	}
	return false
}

func (w *Walker) Replace(filename string) error {
	// this is lazy, but whatever
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	if !w.containsReplacement(b) {
		return nil
	}
	for _, r := range w.Reps {
		b = bytes.Replace(b, []byte(r.From), []byte(r.To), -1)
	}
	if err := ioutil.WriteFile(filename, b, 0644); err != nil {
		return err
	}
	w.Modified = append(w.Modified, filename)
	return nil
}

func (w *Walker) Walk(path string, fi os.FileInfo, err error) error {
	name := fi.Name()
	if fi.IsDir() {
		return w.skipDir(name, fi)
	}
	if !strings.HasSuffix(name, ".go") {
		return nil
	}
	if err := w.Replace(path); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", path, err)
	}
	return nil
}

func (w *Walker) fmtImports(filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	out, err := imports.Process(filename, b, nil)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, 0644)
}

func (w *Walker) FormatImports() error {
	for _, name := range w.Modified {
		if err := w.fmtImports(name); err != nil {
			return err
		}
	}
	return nil
}

var IncludeFakes bool

func init() {
	flag.BoolVar(&IncludeFakes, "fake", false, "Modify fakes")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage %[1]s: PATH FROM:TO...\n\n"+
			"  Replace all occurances of FROM with TO in Go files.\n"+
			"  Example %[1]s . foo:bar baz:buzz\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
	}

	dirname := flag.Arg(0)
	if _, err := os.Stat(dirname); err != nil {
		flag.Usage()
	}

	w := Walker{IncludeFakes: IncludeFakes}
	for _, arg := range flag.Args()[1:] {
		a := strings.Split(arg, ":")
		if len(a) != 2 {
			fmt.Fprintf(os.Stderr, "invalid argument: %s\n", arg)
			flag.Usage()
		}
		w.Reps = append(w.Reps, Replace{a[0], a[1]})
	}

	start := time.Now()
	fmt.Println("Making replacements")
	if err := filepath.Walk(dirname, w.Walk); err != nil {
		Fatal(err)
	}

	fmt.Println("Formatting imports")
	if err := w.FormatImports(); err != nil {
		Fatal(err)
	}

	fmt.Printf("Succuss: %s\n", time.Since(start))
}

func Fatal(err interface{}) {
	if err == nil {
		return
	}
	errMsg := "Error"
	if _, file, line, _ := runtime.Caller(1); file != "" {
		errMsg = fmt.Sprintf("Error (%s:#%d)", filepath.Base(file), line)
	}
	switch e := err.(type) {
	case string, error, fmt.Stringer:
		fmt.Fprintf(os.Stderr, "%s: %s\n", errMsg, e)
	default:
		fmt.Fprintf(os.Stderr, "%s: %#v\n", errMsg, e)
	}
	os.Exit(1)
}
