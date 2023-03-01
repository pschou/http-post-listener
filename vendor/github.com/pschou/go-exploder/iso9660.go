package exploder

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"

	//"github.com/kdomanski/iso9660"
	"github.com/pschou/go-iso9660"
	"github.com/pschou/go-tease"
)

type iso9660File struct {
	reader     *tease.Reader
	files      []*iso9660.File
	fileToPath map[*iso9660.File]string
	i_file     int
	size       int64
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: testISO9660,
		Read: readISO9660,
		Type: "iso9660",
	})
}

func testISO9660(tr *tease.Reader) bool {
	buf := make([]byte, 5)
	sp, err := tr.Seek(32769, io.SeekStart)
	if err != nil || sp != 32769 {
		return false
	}
	n, err := tr.Read(buf)
	if err != nil || n != 5 {
		return false
	}
	//fmt.Println("cp=", cp, err)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{'C', 'D', '0', '0', '1'}) == 0
}

func readISO9660(tr *tease.Reader, size int64) (Archive, error) {
	img, err := iso9660.OpenImage(tr)
	if err != nil {
		return nil, fmt.Errorf(" error opening iso image: %s", err)
	}

	f, err := img.RootDir()
	if err != nil {
		return nil, err
	}

	if !f.IsDir() {
		return nil, fmt.Errorf("%s (claimed as root) is not a directory", f.Name())
	}

	ret := make([]*iso9660.File, 0)
	FileToPath := make(map[*iso9660.File]string)

	var getAll func(string, *iso9660.File) error
	getAll = func(currentPath string, f *iso9660.File) error {
		children, err := f.GetChildren()
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.IsDir() {
				err := getAll(path.Join(currentPath, child.Name()), child)
				if err != nil {
					return err
				}
			} else {
				ret = append(ret, child)
				FileToPath[child] = currentPath
			}
		}
		return nil
	}

	err = getAll("./", f)
	if err != nil {
		return nil, err
	}

	sort.Slice(ret, func(i, j int) bool { return ret[i].DataOffset() < ret[j].DataOffset() })

	//for i, f := range ret {
	//	fmt.Println(i, f.DataOffset(), f.Size(), f)
	//}
	tr.Seek(0, io.SeekStart)
	tr.Pipe()
	return &iso9660File{
		reader:     tr,
		files:      ret,
		fileToPath: FileToPath,
		size:       size,
	}, nil

}

func (i *iso9660File) Type() string {
	return "iso9660"
}
func (i *iso9660File) Close() {
	if i.reader != nil {
		i.reader.Close()
	}
}

func (i *iso9660File) IsEOF() bool {
	return i.i_file >= len(i.files)
}

func (i *iso9660File) Next() (path, name string, r io.Reader, err error) {
	if i.i_file >= len(i.files) {
		err = io.EOF
		return
	}
	if i.i_file > 0 {
		cf := i.files[i.i_file]
		for cf.DataOffset()+cf.Size() > i.files[i.i_file].DataOffset() {
			i.i_file++
			if i.i_file >= len(i.files) {
				err = io.EOF
				return
			}
		}
	}
	f := i.files[i.i_file]
	i.i_file++
	//i.files = i.files[1:]
	var n int64
	n, err = i.reader.Seek(f.DataOffset(), io.SeekStart)
	if Debug {
		fmt.Println(f.DataOffset(), f.Name(), f.Size(), n)
	}
	if err != nil {
		return
	}
	path = i.fileToPath[f]
	name = f.Name()
	//fmt.Println("  limits set:", f.DataOffset(), f.Size())
	r = io.LimitReader(i.reader, f.Size())
	return
}
