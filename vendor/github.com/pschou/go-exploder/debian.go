package exploder

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pschou/go-tease"
	deb "pault.ag/go/debian/deb"
)

type DEBFile struct {
	ar     *deb.Ar
	ar_ent *deb.ArEntry
	eof    bool
	size   int64
}

func init() {
	formatTests = append(formatTests, formatTest{
		Test: testDEB,
		Read: readDEB,
		Type: "debian",
	})
}

func testDEB(tr *tease.Reader) bool {
	tr.Seek(0, io.SeekStart)
	buf := make([]byte, 8)
	tr.Read(buf)
	tr.Seek(0, io.SeekStart)
	return bytes.Compare(buf, []byte{
		0x21, 0x3C, 0x61, 0x72, 0x63, 0x68, 0x3E, 0x0A}) == 0
}

func readDEB(tr *tease.Reader, size int64) (Archive, error) {

	ar, err := deb.LoadAr(tr)
	if err != nil {
		if Debug {
			fmt.Println("Error reading debian", err)
		}
		return nil, err
	}
	ar_ent, err := ar.Next()
	if err != nil {
		return nil, err
	}
	tr.Pipe()
	ret := DEBFile{
		ar:     ar,
		ar_ent: ar_ent,
		eof:    false,
		size:   size,
	}

	return &ret, nil
}

func (i *DEBFile) Type() string {
	return "debian"
}

func (i *DEBFile) IsEOF() bool {
	return i.eof
}

func (c *DEBFile) Close() {
	//if c.z_reader != nil {
	//	c.z_reader.Close()
	//}
}

func (i *DEBFile) Next() (dir, name string, r io.Reader, err error) {
	var ar_ent *deb.ArEntry
	for {
		if i.ar_ent != nil {
			ar_ent = i.ar_ent
			i.ar_ent = nil
		} else {
			ar_ent, err = i.ar.Next()
			if err == io.EOF {
				return "", "", nil, err
			}
		}
		if ar_ent.IsTarfile() {
			break
		}
	}
	var c interface{}
	_, c, err = ar_ent.Tarfile()
	if err != nil {
		return "", "", nil, err
	}
	if ir, ok := (c).(io.Reader); ok {
		r = ir
	} else {
		return "", "", nil, io.EOF
	}
	dir = "."
	name = ar_ent.Name
	return
}
