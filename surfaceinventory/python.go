package surfaceinventory

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// Python/FastMCP extraction: decorator-registered MCP surfaces of the form
//
//	@mcp.tool()                      # name from def, description from docstring
//	@mcp.tool(name="x", description="y")
//	@mcp.resource("uri://template")  # first positional string is the URI
//	@mcp.prompt()
//
// matched line-wise (no Python AST). Single-line decorators only — the fleet
// idiom; a decorator whose arguments span lines loses kwargs but still
// resolves name via the following def.
var (
	pyDecoratorRe  = regexp.MustCompile(`^\s*@\w+\.(tool|resource|prompt)\s*(\(.*)?$`)
	pyDefRe        = regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)\s*\(`)
	pyKwNameRe     = regexp.MustCompile(`\bname\s*=\s*["']([^"']+)["']`)
	pyKwDescRe     = regexp.MustCompile(`\bdescription\s*=\s*["']([^"']+)["']`)
	pyPositionalRe = regexp.MustCompile(`^\(\s*["']([^"']+)["']`)
	pyDocstringRe  = regexp.MustCompile(`^\s*(?:"""|''')\s*(.*?)\s*(?:"""|''')?\s*$`)
)

var pyKindByDecorator = map[string]string{
	"tool":     KindMCPTool,
	"resource": KindMCPResource,
	"prompt":   KindMCPPrompt,
}

// scanPythonFile extracts FastMCP decorator registrations from one file.
func scanPythonFile(path, relFile string) ([]Surface, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Surface
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	type pending struct {
		kind, name, desc string
		line             int
	}
	var p *pending
	awaitingDocstring := -1 // index into out, -1 = none
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		if awaitingDocstring >= 0 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if m := pyDocstringRe.FindStringSubmatch(line); m != nil {
				out[awaitingDocstring].Description = m[1]
				out[awaitingDocstring].HasDescription = m[1] != ""
				awaitingDocstring = -1
				continue
			}
			// Not a docstring — stop waiting and process this line normally.
			awaitingDocstring = -1
		}

		if m := pyDecoratorRe.FindStringSubmatch(line); m != nil {
			p = &pending{kind: pyKindByDecorator[m[1]], line: lineNo}
			args := m[2]
			if n := pyKwNameRe.FindStringSubmatch(args); n != nil {
				p.name = n[1]
			} else if pos := pyPositionalRe.FindStringSubmatch(args); pos != nil {
				p.name = pos[1]
			}
			if d := pyKwDescRe.FindStringSubmatch(args); d != nil {
				p.desc = d[1]
			}
			continue
		}

		if p != nil {
			if m := pyDefRe.FindStringSubmatch(line); m != nil {
				name := p.name
				if name == "" {
					name = m[1]
				}
				out = append(out, Surface{
					Kind: p.kind, Name: name, Description: p.desc, HasDescription: p.desc != "",
					Pattern: "fastmcp.decorator", File: relFile, Line: p.line,
				})
				if p.desc == "" {
					awaitingDocstring = len(out) - 1
				}
				p = nil
			} else if !strings.HasPrefix(strings.TrimSpace(line), "@") && strings.TrimSpace(line) != "" {
				// Something other than another decorator or the def — bail.
				p = nil
			}
		}
	}
	return out, scanner.Err()
}

// isScannablePython reports whether rel is a non-test Python source file.
func isScannablePython(rel string) bool {
	if !strings.HasSuffix(rel, ".py") {
		return false
	}
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		base = rel[i+1:]
	}
	return !strings.HasPrefix(base, "test_") && !strings.HasSuffix(base, "_test.py")
}
