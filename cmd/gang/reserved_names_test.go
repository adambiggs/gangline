package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// An agent named after one of Gangline's own senders would deliver messages
// labelled exactly like Gangline's notices, which the contract tells agents
// Gangline itself emits. An agent already holding such a name stays reachable
// by commands that name it.
func TestAgentsCannotTakeAGanglineSenderName(t *testing.T) {
	for _, name := range []string{"usage-band", "Usage-Band", "compact", "contract", "operator", "Hitch"} {
		t.Run(name, func(t *testing.T) {
			_, err := parseHitch([]string{name}, "claude", "/work")
			if commandErr, ok := err.(commandError); !ok || commandErr.status != exitUsage || !strings.Contains(commandErr.text, strconv.Quote(name)) {
				t.Errorf("parseHitch(%q) = %#v; want a usage error naming it", name, err)
			}
			f := newStateFixture(t)
			f.add(t, "a", "worker", "codex")
			err = f.cmd.rename([]string{"worker", name})
			if commandErr, ok := err.(commandError); !ok || commandErr.status != exitUsage || !strings.Contains(commandErr.text, strconv.Quote(name)) {
				t.Errorf("rename worker %s = %#v; want a usage error naming it", name, err)
			}
			if _, err := f.run.resolve("worker"); err != nil {
				t.Errorf("worker no longer resolves after a refused rename: %v", err)
			}
		})
	}
	if _, err := parseHitch([]string{"usage-band", "--recover"}, "claude", "/work"); err != nil {
		t.Errorf("hitch usage-band --recover = %v; want an existing agent to stay recoverable", err)
	}
}

// Every label Gangline renders for itself must be a name no agent can take,
// so a new Gangline sender cannot ship without its reservation.
func TestEveryGanglineSenderLabelIsReserved(t *testing.T) {
	labels := ganglineSenderNames(t)
	wire, err := startupEnvelopeText(&core.StartupSections{Contract: "c", Doctrine: "d", Role: "r"}, "lead", "0123456789abcdef", "startup", "m")
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range regexp.MustCompile(`\[gang:([^#\]]+)#`).FindAllStringSubmatch(wire, -1) {
		labels = append(labels, match[1])
	}
	for _, want := range []string{"usage-band", "contract", "startup"} {
		found := false
		for _, label := range labels {
			found = found || label == want
		}
		if !found {
			t.Fatalf("label scan missed %q in %q; the scan no longer reads Gangline's senders", want, labels)
		}
	}
	for _, label := range labels {
		if validateNewAgentName(label) == nil {
			t.Errorf("Gangline sends as %q, which an agent may take", label)
		}
	}
}

// ganglineSenderNames reads every Sender literal of kind SenderGangline in the
// module's sources and returns its name. A Sender literal whose kind or name
// is not written out cannot be checked, so it fails the test.
func ganglineSenderNames(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root not found from the package directory: %v", err)
	}
	var names []string
	files := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isSenderType(literal.Type) {
				return true
			}
			kind, name, known := "", "", true
			for _, element := range literal.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					known = false
					continue
				}
				key, _ := field.Key.(*ast.Ident)
				switch {
				case key == nil:
					known = false
				case key.Name == "Kind":
					switch value := field.Value.(type) {
					case *ast.SelectorExpr:
						kind = value.Sel.Name
					case *ast.Ident:
						kind = value.Name
					}
					// Only a named sender kind constant is readable here.
					if !strings.HasPrefix(kind, "Sender") {
						known = false
					}
				case key.Name == "Name":
					value, ok := field.Value.(*ast.BasicLit)
					if !ok || value.Kind != token.STRING {
						name = "\x00"
						continue
					}
					name, _ = strconv.Unquote(value.Value)
				}
			}
			switch {
			case !known:
				t.Errorf("%s: Sender literal whose kind cannot be read, so its reservation cannot be checked", files.Position(literal.Pos()))
			case kind != "SenderGangline":
			case name == "\x00":
				t.Errorf("%s: Gangline sender name is not a string literal, so its reservation cannot be checked", files.Position(literal.Pos()))
			default:
				names = append(names, name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func isSenderType(expression ast.Expr) bool {
	switch typ := expression.(type) {
	case *ast.SelectorExpr:
		return typ.Sel.Name == "Sender"
	case *ast.Ident:
		return typ.Name == "Sender"
	}
	return false
}
