package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/tree2"
)

func TestTree2CommandDefaultsPathAndUsesFallbackWidth(t *testing.T) {
	var got tree2.Options
	var printed bytes.Buffer
	cmd := newTree2Command(tree2Deps{
		out: &printed,
		build: func(opts tree2.Options) ([]tree2.Node, error) {
			got = opts
			return []tree2.Node{{Name: "internal", IsDir: true}}, nil
		},
		render: func([]tree2.Node, tree2.Options) string { return "rendered" },
		getWidth: func() (int, error) {
			return 0, errors.New("not a terminal")
		},
	})
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got.RootPath != "." {
		t.Fatalf("RootPath = %q, want .", got.RootPath)
	}
	if got.Width != tree2.DefaultWidth {
		t.Fatalf("Width = %d, want %d", got.Width, tree2.DefaultWidth)
	}
	if got.Limit != tree2.DefaultLimit {
		t.Fatalf("Limit = %d, want %d", got.Limit, tree2.DefaultLimit)
	}
	if printed.String() != "rendered\n" {
		t.Fatalf("printed = %q", printed.String())
	}
}

func TestTree2CommandFlagsFlowIntoOptions(t *testing.T) {
	var got tree2.Options
	cmd := newTree2Command(tree2Deps{
		build: func(opts tree2.Options) ([]tree2.Node, error) {
			got = opts
			return []tree2.Node{{Name: "docs", IsDir: true}}, nil
		},
		render: func([]tree2.Node, tree2.Options) string { return "" },
		getWidth: func() (int, error) {
			t.Fatal("getWidth should not be called when --width is set")
			return 0, nil
		},
	})
	cmd.SetArgs([]string{"./internal", "--width", "120", "--cols", "3", "--files", "--all", "--limit", "0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got.RootPath != "./internal" || got.Width != 120 || got.Columns != 3 {
		t.Fatalf("unexpected options: %+v", got)
	}
	if !got.IncludeFiles || !got.IncludeHidden {
		t.Fatalf("expected files/all true: %+v", got)
	}
	if got.Limit != 0 {
		t.Fatalf("Limit = %d, want 0", got.Limit)
	}
}

func TestTree2CommandRejectsInvalidNumericFlags(t *testing.T) {
	tests := [][]string{
		{"--width", "-1"},
		{"--cols", "-1"},
		{"--limit", "-1"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newTree2Command(tree2Deps{})
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
