// Copyright 2022 github.com/pschou/archive-exploder
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exploder

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/pschou/go-tease"
)

// Interface in which archives can be interfaced with directly
type Archive interface {
	Next() (name, path string, r io.Reader, err error)
	IsEOF() bool
	Close()
	Type() string
}

// Interface to test known file formats
type formatTest struct {
	Test func(*tease.Reader) bool
	Read func(*tease.Reader, int64) (Archive, error)
	Type string
}

// A slice with all the formats checking in as available, see the init() in every go file.
var formatTests = []formatTest{}

var Debug bool

// Explode the archive by looking at the file MagicBytes and then try that
// archive reader so as to extract layers of archives all at once.
//
// Note: Some layers are represented in a single extraction, while others, like
// tgz are actually two layers, a gzip on the first and a tar on the second.
//
// One must provide an io.Reader and the Size of provided reader for extraction.
func Explode(filePath string, in io.Reader, size int64, recursion int) (err error) {
	if recursion == 0 {
		// If we have reached the max depth, print out any file / archive without testing
		var n int64
		n, err = writeFile(filePath, in)
		if err != nil && Debug {
			fmt.Println("Copy err:", err)
		}
		if size >= 0 && n != size {
			log.Println("Reader.MaxDepth: copied file size does not match")
		}
		return
	}

	tr := tease.NewReader(in)

	// Create an array for matching archive formats
	matches := []formatTest{}

	for _, ft := range formatTests {
		if ft.Test(tr) {
			matches = append(matches, ft)
		}
	}

	tr.Seek(0, io.SeekStart)

	switch len(matches) {
	case 0:
		if Debug {
			fmt.Println("no archive match for", filePath)
		}
		var n int64
		tr.Seek(0, io.SeekStart)
		tr.Pipe()
		n, err = writeFile(filePath, tr)
		if size >= 0 && n != size {
			log.Println("copied file size,", n, ", and expected,", size, ", do not match")
		}
	case 1:
		// We found only one potential archive match, go ahead and explode it.
		tr.Seek(0, io.SeekStart)
		ft := matches[0]
		if Debug {
			fmt.Println("archive match for", filePath, "type", ft.Type)
		}
		if arch, err := ft.Read(tr, size); err == nil {
			if err != nil {
				fmt.Println("Read test failed for", arch.Type(), "file", filePath)
				fmt.Println("err:", err)
				tr.Seek(0, io.SeekStart)
				tr.Pipe()
				_, err = writeFile(filePath, tr)
				return err
			}
			//defer arch.Close()
			for !arch.IsEOF() {
				a_dir, a_file, r, err := arch.Next()
				to_read := int64(-1)
				if lr, ok := (r).(*io.LimitedReader); ok {
					to_read = lr.N
				}

				if err != nil {
					break
				}

				// If we have another file, try exploding that
				Explode(path.Join(filePath, a_dir, a_file), r, to_read, recursion-1)
			}
		} else {
			fmt.Println("Warning: MagicBytes indicated and archive (", ft.Type, ") but failed to expand:", err)
			fmt.Println("  ", filePath)
			tr.Seek(0, io.SeekStart)
			tr.Pipe()
			_, err = writeFile(filePath, tr)
			if err != nil && Debug {
				fmt.Println("Copy err:", err)
			}
		}
	default:
		if Debug {
			fmt.Println("Archive", filePath, "matches multiple formats, what to do?")
			for _, ft := range matches {
				fmt.Println("  ", ft.Type)
			}
		}
		tr.Seek(0, io.SeekStart)
		tr.Pipe()
		_, err = writeFile(filePath, tr)
		if err != nil && Debug {
			fmt.Println("Copy err:", err)
		}
	}

	return
}

func writeFile(filePath string, in io.Reader) (int64, error) {
	dir, _ := path.Split(filePath)
	if Debug {
		fmt.Println("Writing out file", filePath, "in", dir)
	}
	ensureDir(dir)
	out, err := os.Create(filePath)
	if err != nil {
		log.Println("= Error creating file", filePath, "err:", err)
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, in)
}
