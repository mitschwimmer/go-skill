// Command visualtest validates scenario catalogs and compares captured revisions.
// Browser capture itself stays in the consuming application's Go tests.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mitschwimmer/go-skill/visualtest"
)

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: visualtest validate <scenarios-dir> | compare --base DIR --candidate DIR --out DIR")
	}
	switch args[0] {
	case "validate":
		if len(args) != 2 {
			return errors.New("usage: visualtest validate <scenarios-dir>")
		}
		definitions, err := visualtest.Load(args[1])
		if err != nil {
			return err
		}
		if len(definitions) == 0 {
			return errors.New("no scenario definitions found")
		}
		fmt.Printf("Validated %d scenarios\n", len(definitions))
		return nil
	case "compare":
		flags := flag.NewFlagSet("compare", flag.ContinueOnError)
		base := flags.String("base", "", "baseline capture directory")
		candidate := flags.String("candidate", "", "PR capture directory")
		output := flags.String("out", "", "changed-only HTML and JSON report directory")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *base == "" || *candidate == "" || *output == "" {
			return errors.New("compare requires --base, --candidate and --out")
		}
		summary, err := visualtest.Compare(*base, *candidate, *output)
		if err != nil {
			return err
		}
		fmt.Print(summary.MarkdownSummary())
		if file := os.Getenv("GITHUB_STEP_SUMMARY"); file != "" {
			f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			_, writeErr := f.WriteString(summary.MarkdownSummary())
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			return closeErr
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
