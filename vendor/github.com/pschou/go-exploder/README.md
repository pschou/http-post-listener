# go-exploder

This is a generalized archive exploder which can take an array of archive formats and expand them for inspection.

Archive formats supported:
  - 7zip
  - bzip2
  - cab
  - debian
  - gzip / bgzf / apk
  - iso9660
  - lzma
  - rar
  - rpm
  - tar
  - xz
  - zip
  - zstd

# Example

Here is an example of an infinite explosion.  To get a finite number of layers extracted, set -1 to a value like 2 or 3.

```golang
  fh, err := os.Open("testdata.zip") // Open a file
  if err != nil {
    log.Fatal(err)
  }
  stat, err := fh.Stat() // Stat the file to get the size
  if err != nil {
    log.Fatal(err)
  }

  outputPath := "output/"

  err = exploder.Explode(outputPath, fh, stat.Size(), -1)
```

After this has processed, a folder named output will be created with layers upon layers of files in them.

# Documentation

Documentation and usage can be found at:

https://pkg.go.dev/github.com/pschou/go-exploder
