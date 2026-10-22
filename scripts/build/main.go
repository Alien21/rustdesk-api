// Build the current API working tree without generating files beside its sources.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func copyTree(source, destination string, root bool) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && (name == ".git" || name == "node_modules" ||
			(root && (name == "artifacts" || name == "release" || name == "data" ||
				name == "runtime" || name == ".idea" || name == ".vscode"))) {
			continue
		}
		from, to := filepath.Join(source, name), filepath.Join(destination, name)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("build input must not be a symlink: %s", from)
		}
		if entry.IsDir() {
			if err := os.MkdirAll(to, 0755); err != nil {
				return err
			}
			if err := copyTree(from, to, false); err != nil {
				return err
			}
			continue
		}
		input, err := os.Open(from)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			input.Close()
			return err
		}
		output, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func run(directory, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = directory
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func artifactDirectory(source string) (string, error) {
	base := os.Getenv("RUSTDESK_ARTIFACTS_DIR")
	if base == "" {
		return filepath.Join(source, "artifacts"), nil
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("RUSTDESK_ARTIFACTS_DIR must be an absolute central directory")
	}
	namespace := filepath.Join(filepath.Clean(base), "rustdesk-api")
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	var namespaceInfo os.FileInfo
	for current := namespace; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if info != nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("refusing redirected artifact path: %s", current)
			}
			if !info.IsDir() {
				return "", fmt.Errorf("artifact directory is not a directory: %s", current)
			}
			if os.SameFile(sourceInfo, info) {
				return "", fmt.Errorf("central API namespace must be outside the source tree")
			}
			if current == namespace {
				namespaceInfo = info
			}
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if namespaceInfo != nil {
		for current := source; ; current = filepath.Dir(current) {
			info, err := os.Stat(current)
			if err != nil {
				return "", err
			}
			if os.SameFile(namespaceInfo, info) {
				return "", fmt.Errorf("central API namespace must not contain the source tree")
			}
			if filepath.Dir(current) == current {
				break
			}
		}
	}
	return namespace, nil
}

func build() error {
	targetOS := flag.String("goos", runtime.GOOS, "target GOOS")
	targetArch := flag.String("goarch", runtime.GOARCH, "target GOARCH")
	cgo := flag.String("cgo", "1", "target CGO_ENABLED")
	docs := flag.String("docs", "auto", "auto, true or false (Swagger runs only in the source stage)")
	web := flag.String("web-dist", "", "optional compiled admin frontend")
	flag.Parse()
	for _, value := range []string{*targetOS, *targetArch} {
		if value == "" || strings.Trim(value, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
			return fmt.Errorf("invalid build target %q", value)
		}
	}
	if *docs != "auto" && *docs != "true" && *docs != "false" {
		return fmt.Errorf("docs must be auto, true or false")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "apimain.go")); err != nil {
		return fmt.Errorf("run the helper from the API repository: %w", err)
	}
	target := *targetOS + "-" + *targetArch
	artifacts, err := artifactDirectory(root)
	if err != nil {
		return err
	}
	stage := filepath.Join(artifacts, "build", target)
	publish := filepath.Join(artifacts, "publish", target)
	// A symlink must never turn these generated locations into a source/data directory.
	for _, path := range []string{artifacts, filepath.Dir(stage), stage, filepath.Dir(publish), publish} {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing redirected artifact path: %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if err := copyTree(root, stage, true); err != nil {
		return err
	}
	swag, err := exec.LookPath("swag")
	if *docs == "true" && err != nil {
		return fmt.Errorf("DOCS=true requires swag on PATH")
	}
	if *docs != "false" && err == nil {
		for _, instance := range []string{"api", "admin"} {
			excluded := "admin"
			if instance == "admin" {
				excluded = "api"
			}
			if err := run(stage, swag, "init", "-g", "cmd/apimain.go", "--output", "docs/"+instance,
				"--instanceName", instance, "--exclude", "http/controller/"+excluded); err != nil {
				return err
			}
		}
	} else {
		fmt.Println("Using checked-in Swagger documentation.")
	}
	for key, value := range map[string]string{"GOOS": *targetOS, "GOARCH": *targetArch, "CGO_ENABLED": *cgo} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	binary := "apimain"
	if *targetOS == "windows" {
		binary += ".exe"
	}
	// Compile first so a failed build does not remove the last successful output.
	built := filepath.Join(stage, binary)
	if err := run(stage, "go", "mod", "tidy"); err != nil {
		return err
	}
	if err := run(stage, "go", "build", "-o", built, "./cmd/apimain.go"); err != nil {
		return err
	}
	if *web == "" {
		*web = filepath.Join(root, "..", "rustdesk-api-web", "artifacts", "publish", "web")
		if base := os.Getenv("RUSTDESK_ARTIFACTS_DIR"); base != "" {
			*web = filepath.Join(base, "rustdesk-api-web", "publish", "web")
		}
		if _, err := os.Stat(filepath.Join(*web, "index.html")); err != nil {
			*web = ""
		}
	}
	if *web != "" {
		if _, err := os.Stat(filepath.Join(*web, "index.html")); err != nil {
			return fmt.Errorf("frontend is missing index.html: %w", err)
		}
	}
	if err := os.RemoveAll(publish); err != nil {
		return err
	}
	if err := os.MkdirAll(publish, 0755); err != nil {
		return err
	}
	for _, directory := range []string{"resources", "docs", "conf"} {
		destination := filepath.Join(publish, directory)
		if err := os.MkdirAll(destination, 0755); err != nil {
			return err
		}
		if err := copyTree(filepath.Join(stage, directory), destination, false); err != nil {
			return err
		}
	}
	if *web != "" {
		destination := filepath.Join(publish, "resources", "admin")
		if err := os.RemoveAll(destination); err != nil {
			return err
		}
		if err := os.MkdirAll(destination, 0755); err != nil {
			return err
		}
		if err := copyTree(*web, destination, false); err != nil {
			return err
		}
	}
	if err := os.Rename(built, filepath.Join(publish, binary)); err != nil {
		return err
	}
	for _, directory := range []string{"data", "runtime"} {
		if err := os.MkdirAll(filepath.Join(publish, directory), 0755); err != nil {
			return err
		}
	}
	fmt.Println("Build completed:", publish)
	return nil
}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "Build failed:", err)
		os.Exit(1)
	}
}
