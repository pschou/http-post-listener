package exploder

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/pschou/go-tease"
)

type ZstdFile struct {
	z_reader io.Reader
	eof      bool
	count    int
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: testZstd,
		Read: readZstd,
		Type: "zstd",
	})
}

func testZstd(tr *tease.Reader) bool {
	tr.Seek(0, io.SeekStart)
	buf := make([]byte, 4)
	tr.Read(buf)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{0xFD, 0x2F, 0xB5, 0x28}) == 0
}

func readZstd(tr *tease.Reader, size int64) (Archive, error) {
	tr.Seek(0, io.SeekStart)
	r, err := zstd.NewReader(tr)
	if err != nil {
		if Debug {
			fmt.Println("Error reading zstd", err)
		}
		return nil, err
	}

	// do a test read to try to trigger a read error
	buf := []byte{0}
	n, err := r.Read(buf)

	// special case if we compressed an empty file
	if !(n == 0 && err == io.EOF) && err != nil {
		if Debug {
			fmt.Println("Error reading zstd", err)
		}
		return nil, err
	}

	tr.Seek(0, io.SeekStart)
	r, _ = zstd.NewReader(tr)
	ret := ZstdFile{
		z_reader: r,
		eof:      false,
	}

	tr.Pipe()
	return &ret, nil
}

func (i *ZstdFile) Type() string {
	return "zstd"
}

func (i *ZstdFile) IsEOF() bool {
	return i.eof
}

func (c *ZstdFile) Close() {
}

func (i *ZstdFile) Next() (path, name string, r io.Reader, err error) {
	if i.count == 0 {
		i.count = 1
		i.eof = true
		return ".", "pt_1", i.z_reader, nil
	}
	return "", "", nil, io.EOF
}
