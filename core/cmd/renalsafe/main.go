// RenalSafe is an offline academic assessment CLI. It emits output only after audit.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"renalsafe/core/internal/audit"
	"renalsafe/core/internal/decision"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
)

func run(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("renalsafe", flag.ContinueOnError)
	kbPath := fs.String("kb", "../data/knowledge.json", "knowledge JSON path")
	auditPath := fs.String("audit", "", "required local audit JSONL path")
	verifyPath := fs.String("verify-audit", "", "verify a journal and exit")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if *verifyPath != "" {
		b, e := os.ReadFile(*verifyPath)
		if e != nil {
			return e
		}
		es, e := audit.Verify(b)
		if e != nil {
			return e
		}
		return json.NewEncoder(out).Encode(map[string]any{"verified_events": len(es)})
	}
	if *auditPath == "" {
		return fmt.Errorf("-audit is required")
	}
	b, e := os.ReadFile(*kbPath)
	if e != nil {
		return e
	}
	kb, e := rules.Load(b, filepath.Dir(*kbPath), false)
	if e != nil {
		return e
	}
	b, e = io.ReadAll(io.LimitReader(in, 1<<20+1))
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("request too large")
	}
	var req domain.Request
	if e = domain.Decode(b, &req); e != nil {
		return e
	}
	if len(req.Medications) == 0 || len(req.Medications) > 100 {
		return fmt.Errorf("1–100 medication contexts required")
	}
	a := decision.Assess(req, kb)
	event, e := audit.Append(*auditPath, a, kb.Knowledge())
	if e != nil {
		return e
	}
	return json.NewEncoder(out).Encode(event)
}
func main() {
	if e := run(os.Args[1:], os.Stdin, os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
