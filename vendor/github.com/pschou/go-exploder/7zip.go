package exploder

import (
	"bytes"
	"fmt"
	"io"
	"path"

	"github.com/bodgit/sevenzip"
	"github.com/pschou/go-tease"
)

type SevenZipFile struct {
	z_reader *sevenzip.Reader
	eof      bool
	count    int
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: test7Zip,
		Read: read7Zip,
		Type: "7zip",
	})
}

func test7Zip(tr *tease.Reader) bool {
	tr.Seek(0, io.SeekStart)
	buf := make([]byte, 6)
	tr.Read(buf)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}) == 0
}

func read7Zip(tr *tease.Reader, size int64) (Archive, error) {
	tr.Seek(0, io.SeekStart)
	if size < 10 {
		size = 2048
	}
	zr, err := sevenzip.NewReader(tr, size)
	if err != nil {
		if Debug {
			fmt.Println("Error reading 7zip", err)
		}
		return nil, err
	}

	ret := SevenZipFile{
		z_reader: zr,
		eof:      false,
	}

	tr.Seek(0, io.SeekStart)
	tr.Pipe()
	return &ret, nil
}

func (i *SevenZipFile) Type() string {
	return "7zip"
}

func (i *SevenZipFile) IsEOF() bool {
	return i.eof
}

func (c *SevenZipFile) Close() {
	//if c.z_reader != nil {
	//	c.z_reader.Close()
	//}
}

func (i *SevenZipFile) Next() (dir, name string, r io.Reader, err error) {
	var f *sevenzip.File
	for {
		if i.count >= len(i.z_reader.File) {
			return "", "", nil, io.EOF
		}
		f = i.z_reader.File[i.count]
		i.count++
		if !f.FileInfo().IsDir() {
			break
		}
	}

	r, err = f.Open()
	if err != nil {
		return "", "", nil, err
	}
	dir, name = path.Split(f.Name)
	//fmt.Println("path", dir, name, "f.Name=", f.Name)
	return
}
