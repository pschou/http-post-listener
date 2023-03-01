package exploder

import (
	"bytes"
	"fmt"
	"io"
	"path"

	"github.com/cavaliergopher/cpio"
	"github.com/cavaliergopher/rpm"
	"github.com/pschou/go-tease"
	"github.com/ulikunitz/xz"
)

type RPMFile struct {
	xz_reader   *xz.Reader
	cpio_reader *cpio.Reader
	pkg         *rpm.Package
	eof         bool
	size        int64
	count       int
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: testRPM,
		Read: readRPM,
		Type: "rpm",
	})
}

func testRPM(tr *tease.Reader) bool {
	tr.Seek(0, io.SeekStart)
	buf := make([]byte, 4)
	tr.Read(buf)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{0xED, 0xAB, 0xEE, 0xDB}) == 0
}

func readRPM(tr *tease.Reader, size int64) (Archive, error) {

	// Read the package headers
	pkg, err := rpm.Read(tr)
	if err != nil {
		if Debug {
			fmt.Println("Error reading rpm", err)
		}
		return nil, err
	}

	// Check the compression algorithm of the payload
	if compression := pkg.PayloadCompression(); compression != "xz" {
		return nil, fmt.Errorf("Unsupported compression: %s", compression)
	}

	// Attach a reader to decompress the payload
	xzReader, err := xz.NewReader(tr)
	if err != nil {
		return nil, err
	}

	// Check the archive format of the payload
	if format := pkg.PayloadFormat(); format != "cpio" {
		return nil, fmt.Errorf("Unsupported payload format: %s", format)
	}

	// Attach a reader to unarchive each file in the payload
	cpioReader := cpio.NewReader(xzReader)

	ret := RPMFile{
		xz_reader:   xzReader,
		cpio_reader: cpioReader,
		eof:         false,
		size:        size,
		pkg:         pkg,
	}

	tr.Pipe()
	return &ret, nil
}

func (i *RPMFile) Type() string {
	return "rpm"
}

func (i *RPMFile) IsEOF() bool {
	return i.eof
}

func (c *RPMFile) Close() {
	//if c.z_reader != nil {
	//	c.z_reader.Close()
	//}
}

func (i *RPMFile) Next() (dir, name string, r io.Reader, err error) {
	var hdr *cpio.Header
	for {
		// Move to the next file in the archive
		hdr, err = i.cpio_reader.Next()
		if err != nil {
			return "", "", nil, err
		}

		// Skip directories and other irregular file types in this example
		if hdr.Mode.IsRegular() {
			break
		}
	}
	r = i.cpio_reader
	dir, name = path.Split(hdr.Name)
	//fmt.Println("returning", dir, name, r, err)
	return
}
