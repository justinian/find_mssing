package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/cespare/xxhash"
)

var sizeNames = map[float64]string{
	1024 * 1024 * 1024 * 1024: "TiB",
	1024 * 1024 * 1024:        "GiB",
	1024 * 1024:               "MiB",
	1024:                      "KiB",
}

func fileSize(s float64) string {
	for n, l := range sizeNames {
		if s > n {
			return fmt.Sprintf("%0.2f %s", s/n, l)
		}
	}

	return fmt.Sprintf("%0.0 B", s)
}

type hashList struct {
	files map[uint64]string
	size  int64
}

func (h *hashList) addFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Opening file '%s' - %w", path, err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Reading file '%s' - %w", path, err)
	}

	hash := xxhash.Sum64(data)
	h.files[hash] = path
	return nil
}

func (h *hashList) walk(path string, d fs.DirEntry, e error) error {
	if d.IsDir() {
		fmt.Println(path)
		return nil
	}

	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("Getting file info '%s' - %w", path, err)
	}

	h.size += info.Size()
	return h.addFile(path)
}

func buildHashList(dirs []string) hashList {
	hashes := hashList{
		files: make(map[uint64]string, 1000),
		size:  0,
	}

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, hashes.walk)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v", err)
		}
	}

	return hashes
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <source> <dest> [<dest> ...]\n", os.Args[0])
		return
	}

	sstart := time.Now()
	sources := buildHashList([]string{os.Args[1]})
	dstart := time.Now()
	dests := buildHashList(os.Args[2:])
	end := time.Now()

	tdur := end.Sub(sstart)
	sdur := dstart.Sub(sstart)
	ddur := end.Sub(dstart)

	fmt.Printf("                 total read time: %v\n\n", tdur)

	fmt.Printf("              source files count: %d\n", len(sources.files))
	fmt.Printf("               source files size: %s\n", fileSize(float64(sources.size)))
	fmt.Printf("         source files read speed: %s/s\n\n", fileSize(float64(sources.size)/sdur.Seconds()))

	fmt.Printf("         destination files count: %d\n", len(dests.files))
	fmt.Printf("          destination files size: %s\n", fileSize(float64(dests.size)))
	fmt.Printf("    destination files read speed: %s/s\n\n", fileSize(float64(dests.size)/ddur.Seconds()))

	missing := make([]string, 0, len(sources.files))
	for hash, path := range sources.files {
		if _, ok := dests.files[hash]; !ok {
			missing = append(missing, path)
		}
	}

	fmt.Printf("             MISSING files: %d\n", len(missing))

	if len(missing) == 0 {
		return
	}

	outpath := filepath.Join(os.Args[1], "missing_files.txt")
	fmt.Printf("\nWriting missing file report to: %s\n", outpath)

	out, err := os.Create(outpath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for i := range missing {
		fmt.Fprintf(out, "%s\n", missing[i])
	}

	out.Close()
}
