// Command curriculum-validate checks curriculum documents and exits with a
// three-valued code.
//
//	0  every document is coherent and every question was answerable
//	1  at least one document holds a real finding
//	2  at least one question could not be answered, or no document was checked
//
// A 2 is never a pass. In particular, being given nothing to check is a 2 and
// not a 0: a validator that reports success over an empty input set has
// measured nothing.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vasic-digital/curriculum-kit/pkg/curriculum"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("curriculum-validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	quiet := fs.Bool("quiet", false, "print only the summary line")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) == 0 {
		fmt.Fprintln(stderr, "UNDETERMINED — no document given; nothing was checked, and that is not a pass")
		return 2
	}

	worst := 0
	findings, undet := 0, 0
	for _, path := range files {
		rc, nf, nu := check(path, *quiet, stdout, stderr)
		findings += nf
		undet += nu
		// Precedence: 1 outranks 2 outranks 0. A file that is measurably broken
		// must not be masked by another file that merely could not be judged.
		if rc == 1 || (rc == 2 && worst == 0) {
			if rc == 1 {
				worst = 1
			} else {
				worst = 2
			}
		}
	}

	switch worst {
	case 0:
		fmt.Fprintf(stdout, "OK — %d document(s), 0 finding(s), 0 undetermined\n", len(files))
	case 1:
		fmt.Fprintf(stdout, "FINDING — %d document(s), %d finding(s), %d undetermined\n", len(files), findings, undet)
	default:
		fmt.Fprintf(stdout, "UNDETERMINED — %d document(s), 0 finding(s), %d undetermined; this is not a pass\n", len(files), undet)
	}
	return worst
}

func check(path string, quiet bool, stdout, stderr *os.File) (rc, findings, undet int) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "UNDETERMINED %s — cannot read: %v\n", path, err)
		return 2, 0, 1
	}
	defer f.Close()

	doc, err := curriculum.DecodeDocument(f)
	if err != nil {
		// A file that will not parse is an absence of evidence about the catalog
		// it was meant to hold, not a measured statement about that catalog.
		fmt.Fprintf(stderr, "UNDETERMINED %s — %v\n", path, err)
		return 2, 0, 1
	}

	rep := doc.Validate()
	if !quiet {
		for _, fd := range rep.Findings {
			fmt.Fprintf(stdout, "  %s: %s\n", path, fd)
		}
	}
	return rep.Verdict(), len(rep.FindingRows()), len(rep.Undetermined())
}
