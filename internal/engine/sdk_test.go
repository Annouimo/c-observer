//go:build darwin || linux

package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompilerSDKArgs(t *testing.T) {
	dir := t.TempDir()
	sdk := filepath.Join(dir, "macOS SDK")
	include := filepath.Join(sdk, "usr", "include")
	if err := os.MkdirAll(include, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(include, "stdio.h"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	xcrun := "#!/bin/sh\n" +
		"if [ \"$*\" != '--sdk macosx --show-sdk-path' ]; then exit 1; fi\n" +
		"printf '%s\\n' \"$TEST_MACOS_SDK\"\n"
	if err := os.WriteFile(filepath.Join(dir, "xcrun"), []byte(xcrun), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TEST_MACOS_SDK", sdk)
	t.Setenv("SDKROOT", "")

	args := []string{"-Wall"}
	want := []string{"-isysroot", sdk, "-Wall"}
	for _, sdkroot := range []string{"", "/", sdk} {
		t.Setenv("SDKROOT", sdkroot)
		got, err := compilerSDKArgs(context.Background(), "darwin", args)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("SDKROOT %q: got %q, %v; want %q", sdkroot, got, err, want)
		}
	}
	t.Setenv("SDKROOT", filepath.Join(dir, "missing SDK"))
	for _, args := range [][]string{{"-isysroot", "/custom"}, {"--sysroot=/custom"}} {
		got, err := compilerSDKArgs(context.Background(), "darwin", args)
		if err != nil || !reflect.DeepEqual(got, args) {
			t.Fatalf("explicit SDK should be preserved: got %q, %v", got, err)
		}
	}
	if got, err := compilerSDKArgs(context.Background(), "linux", args); err != nil || !reflect.DeepEqual(got, args) {
		t.Fatalf("non-Darwin args changed: got %q, %v", got, err)
	}
	if _, err := compilerSDKArgs(context.Background(), "darwin", nil); err == nil || !strings.Contains(err.Error(), "stdio.h") {
		t.Fatalf("missing SDK should report a direct diagnostic: %v", err)
	}
}
