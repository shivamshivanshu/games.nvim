// Command schemagen writes the JSON Schema of the wire protocol and of each
// registered game's bodies, or checks that a directory holds exactly that.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/invopop/jsonschema"

	"github.com/shivamshivanshu/games.nvim/server/games"
)

const module = "github.com/shivamshivanshu/games.nvim/server"

func main() {
	out := flag.String("out", "", "directory to write the schema into")
	check := flag.String("check", "", "directory to compare with the generated schema")
	flag.Parse()
	if err := run(*out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
}

// run reads doc comments from the module in the working directory.
func run(out, check string) error {
	if (out == "") == (check == "") {
		return errors.New("give exactly one of -out and -check")
	}
	reflector := jsonschema.Reflector{}
	if err := reflector.AddGoComments(module, ".", jsonschema.WithFullComment()); err != nil {
		return fmt.Errorf("read doc comments: %w", err)
	}
	files, err := generate(reflector.CommentMap)
	if err != nil {
		return err
	}
	if out != "" {
		return write(out, files)
	}
	return compare(check, files)
}

// generate maps each schema file's path to its content.
func generate(comments map[string]string) (map[string][]byte, error) {
	schemas := map[string]*jsonschema.Schema{}
	protocolDoc, coreEnums, err := protocolSchema(comments)
	if err != nil {
		return nil, fmt.Errorf("protocol: %w", err)
	}
	schemas[protocolFile] = protocolDoc
	for _, g := range games.All() {
		t := g.Info().Type
		if schemas[gameFile(t)], err = gameSchema(g, comments, coreEnums); err != nil {
			return nil, fmt.Errorf("game %s: %w", t, err)
		}
	}
	files := make(map[string][]byte, len(schemas))
	for name, s := range schemas {
		content, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		files[name] = append(content, '\n')
	}
	return files, nil
}

func write(dir string, files map[string][]byte) error {
	stale, err := existing(dir)
	if err != nil {
		return err
	}
	for name := range files {
		delete(stale, name)
	}
	for name := range stale {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	for name, content := range files {
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func compare(dir string, files map[string][]byte) error {
	present, err := existing(dir)
	if err != nil {
		return err
	}
	var differ []string
	for name := range present {
		if _, ok := files[name]; !ok {
			differ = append(differ, name+" (not generated)")
		}
	}
	for name, content := range files {
		got, ok := present[name]
		switch {
		case !ok:
			differ = append(differ, name+" (missing)")
		case !bytes.Equal(got, content):
			differ = append(differ, name)
		}
	}
	if len(differ) > 0 {
		slices.Sort(differ)
		return fmt.Errorf("%s is stale, run make schema: %v", dir, differ)
	}
	return nil
}

// existing maps the path of each .json file under dir to its content.
func existing(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && p == dir {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = content
		return err
	})
	return files, err
}
