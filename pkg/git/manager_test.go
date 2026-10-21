package git

import (
	"archive/zip"
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestSanitizeZipEntryName(t *testing.T) {
	for name, want := range map[string]string{
		"a.go":       "a.go",
		"dir/a.go":   "dir/a.go",
		"dir/../a":   "a",
		"./dir//a":   "dir/a",
		"x/.gitkeep": "x/.gitkeep",
	} {
		got, err := sanitizeZipEntryName(name)
		require.NoError(t, err, name)
		require.Equal(t, want, got, name)
	}
	for _, name := range []string{
		"",
		".",
		"..",
		"../a",
		"dir/../../a",
		"/etc/passwd",
		`dir\a`,
		"a\x00b",
		".git/config",
		"dir/.git/hooks/pre-receive",
	} {
		_, err := sanitizeZipEntryName(name)
		require.Error(t, err, name)
	}
}

func TestUnzipFiles(t *testing.T) {
	files, err := UnzipFiles(zipOf(t, map[string]string{"dir/a.go": "package a\n"}))
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "dir/a.go", files[0].FileName)
	require.Equal(t, []byte("package a\n"), files[0].Content)
}

func TestUnzipFilesRejectsInvalidArchive(t *testing.T) {
	for name, data := range map[string][]byte{
		"not a zip":   []byte("not a zip"),
		"zip slip":    zipOf(t, map[string]string{"../evil": "x"}),
		"git dir":     zipOf(t, map[string]string{".git/config": "x"}),
		"absolute":    zipOf(t, map[string]string{"/etc/passwd": "x"}),
		"backslashes": zipOf(t, map[string]string{`..\evil`: "x"}),
	} {
		_, err := UnzipFiles(data)
		require.ErrorIs(t, err, ErrInvalidArchive, name)
	}
}

func TestUnzipFilesRejectsTooLargeArchive(t *testing.T) {
	big := string(make([]byte, MaxZipFileSize+1))
	_, err := UnzipFiles(zipOf(t, map[string]string{"big": big}))
	require.ErrorIs(t, err, ErrArchiveTooLarge)

	many := make(map[string]string, MaxZipFileCount+1)
	for i := range MaxZipFileCount + 1 {
		many[fmt.Sprintf("f%d", i)] = ""
	}
	_, err = UnzipFiles(zipOf(t, many))
	require.ErrorIs(t, err, ErrArchiveTooLarge)
}
