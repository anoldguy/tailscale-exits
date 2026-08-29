package infrastructure

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

// TestBuildLambdaZip guards the artifact that both createLambdaFunction and
// updateLambdaCode ship to AWS. The provided.al2023 runtime only looks for an
// entrypoint named "bootstrap" at the root of the zip, and a rename or a
// nested path would deploy cleanly and then fail at every invocation.
func TestBuildLambdaZip(t *testing.T) {
	// buildLambdaZip compiles ./lambda relative to the working directory,
	// so it only works from the project root.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir("../../.."); err != nil {
		t.Fatalf("failed to change to project root: %v", err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	zipBytes, err := buildLambdaZip()
	if err != nil {
		t.Fatalf("buildLambdaZip failed: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("buildLambdaZip did not produce a readable zip: %v", err)
	}

	var bootstrap *zip.File
	for _, f := range reader.File {
		if f.Name == "bootstrap" {
			bootstrap = f
		}
	}

	if bootstrap == nil {
		var names []string
		for _, f := range reader.File {
			names = append(names, f.Name)
		}
		t.Fatalf("zip must contain a 'bootstrap' entry at the root, got: %v", names)
	}

	if bootstrap.UncompressedSize64 == 0 {
		t.Errorf("bootstrap entry is empty")
	}
}
