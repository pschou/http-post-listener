package exploder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path"

	"github.com/pschou/go-tease"
)

type TarFile struct {
	a_reader *tar.Reader
	eof      bool
	hdr      *tar.Header
	size     int64
	count    int
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: testTar,
		Read: readTar,
		Type: "tar",
	})
}

func testTar(tr *tease.Reader) bool {
	tr.Seek(257, io.SeekStart)
	buf := make([]byte, 5)
	tr.Read(buf)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{0x75, 0x73, 0x74, 0x61, 0x72}) == 0
}

func readTar(tr *tease.Reader, size int64) (Archive, error) {
	ar := tar.NewReader(tr)
	hdr, err := ar.Next()
	if err != nil {
		if Debug {
			fmt.Println("Error reading tar", err)
		}
		return nil, err
	}

	ret := TarFile{
		a_reader: ar,
		eof:      false,
		size:     size,
		hdr:      hdr,
	}

	tr.Seek(0, io.SeekStart)
	tr.Pipe()
	return &ret, nil
}

func (i *TarFile) Type() string {
	return "tar"
}

func (i *TarFile) IsEOF() bool {
	return i.eof
}

func (c *TarFile) Close() {
	//if c.z_reader != nil {
	//	c.z_reader.Close()
	//}
}

func (i *TarFile) Next() (dir, name string, r io.Reader, err error) {
	var hdr *tar.Header
	for {
		if i.hdr != nil {
			hdr = i.hdr
			i.hdr = nil
		} else {
			hdr, err = i.a_reader.Next()
			if err != nil {
				return "", "", nil, io.EOF
			}
		}
		if hdr.Typeflag == tar.TypeReg {
			break
		}
	}
	r = i.a_reader
	dir, name = path.Split(hdr.Name)
	//fmt.Println("returning", dir, name, r, err)
	return
}
