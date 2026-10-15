package outsidemcp

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

// reportPageBytes is the content budget of one paginated report response.
const reportPageBytes = 64 << 10

type reportRoot struct {
	path string
	root *os.Root
}
type Reports struct{ roots []reportRoot }
type Report struct {
	Path       string `json:"path"`
	Offset     int64  `json:"offset"`
	NextOffset *int64 `json:"next_offset"`
	Content    string `json:"content"`
}

func OpenReports(paths []string) (*Reports, error) {
	r := &Reports{}
	for _, p := range paths {
		if !filepath.IsAbs(p) || filepath.Clean(p) == string(filepath.Separator) {
			r.Close()
			return nil, errors.New("report roots must be absolute directories narrower than the filesystem root")
		}
		root, err := os.OpenRoot(p)
		if err != nil {
			r.Close()
			return nil, err
		}
		r.roots = append(r.roots, reportRoot{filepath.Clean(p), root})
	}
	return r, nil
}
func (r *Reports) Close() error {
	var err error
	for _, p := range r.roots {
		err = errors.Join(err, p.root.Close())
	}
	return err
}
func (r *Reports) Read(path string, offset int64) (Report, error) {
	denied := errors.New("report path is not a readable regular UTF-8 file under a configured directory")
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || offset < 0 {
		return Report{}, denied
	}
	for _, allow := range r.roots {
		rel, err := filepath.Rel(allow.path, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		// Root confines symlink traversal atomically, including directory swaps.
		// Nonblocking open lets us reject FIFOs without waiting for a writer.
		f, err := allow.root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return Report{}, denied
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || offset > info.Size() {
			return Report{}, denied
		}
		data := make([]byte, reportPageBytes+utf8.UTFMax)
		n, err := f.ReadAt(data, offset)
		if err != nil && err != io.EOF {
			return Report{}, denied
		}
		data = data[:n]
		if len(data) > reportPageBytes {
			data = data[:reportPageBytes]
			for trim := 0; trim < utf8.UTFMax-1 && len(data) > 0 && !utf8.Valid(data); trim++ {
				data = data[:len(data)-1]
			}
		}
		if !utf8.Valid(data) || (n > 0 && len(data) == 0) {
			return Report{}, denied
		}
		result := Report{Path: path, Offset: offset, Content: string(data)}
		next := offset + int64(len(data))
		if next < info.Size() {
			result.NextOffset = &next
		}
		return result, nil
	}
	return Report{}, denied
}
