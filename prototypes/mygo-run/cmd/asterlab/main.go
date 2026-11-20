package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"asterlab-mygo-prototype/internal/astercore"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/vault"
)

const (
	exitOK       = 0
	exitUsage    = 2
	exitNotFound = 3
	exitBlocked  = 4
	exitInvalid  = 5
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func emitJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func emitError(w io.Writer, code int, msg string) int {
	emitJSON(w, map[string]any{
		"ok":    false,
		"code":  code,
		"error": msg,
	})
	return code
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("asterlab", flag.ContinueOnError)
	fs.SetOutput(stderr)
	vaultDir := fs.String("vault", "", "AsterLab Vault directory (defaults to local Vault)")
	if err := fs.Parse(args); err != nil {
		return emitError(stdout, exitUsage, err.Error())
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return emitError(stdout, exitUsage, "missing subcommand (try: tools --json | vault.list | vault.search | asterdoc.read | asterdoc.assess | asterdoc.analyze)")
	}
	cmd := rest[0]
	cmdArgs := rest[1:]

	resolvedVault := strings.TrimSpace(*vaultDir)
	if resolvedVault == "" {
		resolvedVault = vault.DefaultVaultDir()
	}

	if cmd == "tools" {
		emitJSON(stdout, map[string]any{
			"ok":    true,
			"vault": resolvedVault,
			"tools": []map[string]any{
				{"name": "tools", "description": "List available CLI tools as JSON"},
				{"name": "vault.list", "description": "List .asterml experiments with optional --status / --tag filters"},
				{"name": "vault.search", "description": "Full-text search across all .asterml files"},
				{"name": "asterdoc.read", "description": "Parse and return AsterML frontmatter & modules"},
				{"name": "asterdoc.assess", "description": "Run Pure Go ARRIVE/control/dilution scientific diagnostics"},
				{"name": "asterdoc.analyze", "description": "Run deterministic CCK-8 / RT-qPCR / descriptive analysis"},
			},
		})
		return exitOK
	}

	v, err := vault.Open(resolvedVault)
	if err != nil {
		return emitError(stdout, exitNotFound, fmt.Sprintf("open vault %q: %v", resolvedVault, err))
	}
	idx := vault.NewVaultIndex(v)

	switch cmd {
	case "vault.list":
		sub := flag.NewFlagSet("vault.list", flag.ContinueOnError)
		sub.SetOutput(stderr)
		status := sub.String("status", "", "Filter by status (planned|in_progress|completed)")
		tag := sub.String("tag", "", "Filter by tag")
		folder := sub.String("folder", "", "Filter by folder prefix")
		if err := sub.Parse(cmdArgs); err != nil {
			return emitError(stdout, exitUsage, err.Error())
		}
		if err := idx.Refresh(); err != nil {
			return emitError(stdout, exitNotFound, err.Error())
		}
		rows := idx.QueryBases(vault.BasesQuery{
			FolderPrefix: *folder,
			Status:       *status,
			Tag:          *tag,
		})
		emitJSON(stdout, map[string]any{
			"ok":    true,
			"vault": v.Root,
			"count": len(rows),
			"files": rows,
		})
		return exitOK

	case "vault.search":
		if len(cmdArgs) == 0 || strings.TrimSpace(strings.Join(cmdArgs, " ")) == "" {
			return emitError(stdout, exitUsage, "vault.search requires a query string")
		}
		if err := idx.Refresh(); err != nil {
			return emitError(stdout, exitNotFound, err.Error())
		}
		q := strings.TrimSpace(strings.Join(cmdArgs, " "))
		hits := idx.SearchFullText(q, 50)
		emitJSON(stdout, map[string]any{
			"ok":    true,
			"query": q,
			"count": len(hits),
			"hits":  hits,
		})
		return exitOK

	case "asterdoc.read", "asterdoc.assess", "asterdoc.analyze":
		if len(cmdArgs) == 0 || strings.TrimSpace(cmdArgs[0]) == "" {
			return emitError(stdout, exitUsage, cmd+" requires a relative .asterml file path")
		}
		rel := cmdArgs[0]
		abs := v.AbsPath(rel)
		rawBytes, rErr := os.ReadFile(abs)
		if rErr != nil {
			return emitError(stdout, exitNotFound, fmt.Sprintf("read %q: %v", rel, rErr))
		}
		src := string(rawBytes)
		fm, _, _ := asterdoc.ScanFrontmatter(src)
		doc, pErr := asterdoc.Parse(src)
		if pErr != nil {
			return emitError(stdout, exitInvalid, fmt.Sprintf("parse %q: %v", rel, pErr))
		}
		if cmd == "asterdoc.read" {
			steps := make([]map[string]any, 0)
			for _, m := range doc.Modules {
				if m.Kind == "step" {
					steps = append(steps, m.Data)
				}
			}
			emitJSON(stdout, map[string]any{
				"ok":          true,
				"relPath":     rel,
				"frontmatter": fm,
				"steps":       steps,
				"moduleCount": len(doc.Modules),
			})
			return exitOK
		}
		if cmd == "asterdoc.assess" {
			rep := astercore.AssessDocument(doc, nil)
			emitJSON(stdout, map[string]any{
				"ok":         rep.Blocking == 0,
				"relPath":    rel,
				"assessment": rep,
			})
			if rep.Blocking > 0 {
				return exitBlocked
			}
			return exitOK
		}
		sums := astercore.AnalyzeDocument(doc, rel)
		emitJSON(stdout, map[string]any{
			"ok":        true,
			"relPath":   rel,
			"count":     len(sums),
			"summaries": sums,
		})
		return exitOK

	default:
		return emitError(stdout, exitUsage, fmt.Sprintf("unknown subcommand %q", cmd))
	}
}
