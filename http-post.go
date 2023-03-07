package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pschou/go-exploder"
	"github.com/remeh/sizedwaitgroup"
)

var about = `HTTP-Post-Listener

This utility is intended to listen on a port and handle post requests, saving
each file to disk and then calling an optional script.`

var (
	basePath      = flag.String("path", "output/", "Directory which to save files")
	script        = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell   = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	listen        = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath    = flag.String("listenPath", "/file", "Where to expect files to be posted")
	enableTLS     = flag.Bool("tls", false, "Enable TLS for secure transport")
	remove        = flag.Bool("rm", false, "Automatically remove file after script has finished")
	enforceTokens = flag.Bool("token-enforce", false, "Enforce tokens, otherwise match only if one is provided")
	tokens        = flag.String("tokens", "", "File to specify tokens for authentication")
	explode       = flag.String("explode", "", "Directory in which to explode an archive into for inspection")
	limit         = flag.Int("limit", 0, "Limit the number of downloads/processing at a given moment to avoid disk bloat")
	tokenMap      = make(map[string]string)
	version       = ""

	swg sizedwaitgroup.SizedWaitGroup
)

func main() {
	flag.Usage = func() {
		lines := strings.SplitN(about, "\n", 2)
		fmt.Fprintf(os.Stderr, "%s (github.com/pschou/http-post-listener, version: %s)\n%s\n\nUsage: %s [options]\n",
			lines[0], version, lines[1], os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if *enableTLS {
		loadTLS()
	}
	fmt.Println("Output set to", *basePath)

	if *limit > 0 {
		fmt.Println("Sized wait group set to:", *limit)
		swg = sizedwaitgroup.New(*limit)
	}

	if *tokens != "" {
		if fh, err := os.Open(*tokens); err != nil {
			log.Fatal(err)
		} else {
			scanner := bufio.NewScanner(fh)
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), ":", 2)
				if len(parts) == 2 && !strings.HasPrefix(parts[0], "#") {
					tokenMap[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
				}
			}
			fh.Close()
		}
	}

	http.HandleFunc("/", uploadHandler)
	if *enableTLS {
		log.Println("Listening with HTTPS on", *listen, "at", *listenPath)
		server := &http.Server{Addr: *listen, TLSConfig: tlsConfig}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Println("Listening with HTTP on", *listen, "at", *listenPath)
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var fh *os.File
	var filename string
	defer func() {
		if fh != nil {
			fh.Close()
			os.Remove(filename)
		}
		if err != nil {
			log.Println("Error:", err)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		r.Body.Close()
	}()

	switch r.Method {
	case "PUT", "POST":
	default:
		err = fmt.Errorf("Method not handled %q", r.Method)
		return
	}

	// Use tokens file for validating connection
	var group string
	var ok bool
	if myToken := r.Header.Get("X-Private-Token"); myToken != "" {
		// A token was provided in the header
		group, ok = tokenMap[myToken]
		if !ok {
			// Token was provided and didn't match any entry
			err = fmt.Errorf("Token not matched %q", myToken)
			return
		}
	} else {
		// A token was missing or empty
		if *enforceTokens {
			err = fmt.Errorf("Token was not provided")
			return
		}
	}

	// Flatten any of the /../ junk
	filename = filepath.Clean(r.URL.Path)

	// Verify that the right path is being hit on POST/PUT endpoint
	if !strings.HasPrefix(filename, *listenPath) || strings.HasPrefix(filename, "..") {
		err = fmt.Errorf("Path not allowed %q", filename)
		return
	}

	// Build the exact path to where to put the file
	filename = path.Join(*basePath, strings.TrimPrefix(filename, *listenPath))

	if *limit > 0 {
		swg.Add()
		defer swg.Done()
	}

	// Make sure the directory exists
	dir, _ := path.Split(filename)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}

	// Open the file for writing, if it alreade exists, do not allow overwrite
	if fh, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666); err != nil {
		return
	}

	log.Printf("recieving file %q...\n", filename)

	// Copy the stream to disk
	if _, err = io.Copy(fh, r.Body); err != nil {
		return
	}

	// If we need to explode the file, do so here
	var explodeDir string
	if *explode != "" {
		explodeDir = path.Join(*explode, RandStringBytes(8))
		stat, _ := fh.Stat()     // Get the file size
		fh.Seek(0, io.SeekStart) // Seek to the beginning
		data := path.Join(explodeDir, "data")
		os.MkdirAll(explodeDir, 0755)
		exploder.Explode(data, fh, stat.Size(), -1)
	}
	fh.Close()
	fh = nil

	log.Printf("successfully transferred %q\n", filename)

	if *script != "" {
		log.Println("Calling script", *scriptShell, *script, filename, group, explodeDir)
		output, err := exec.Command(*scriptShell, *script, filename, group, explodeDir).Output()
		log.Println("----- START", *script, filename, "-----")
		fmt.Println(string(output))
		log.Println("----- END", *script, filename, "-----")
		if err != nil {
			log.Printf("error %s", err)
		}
	}

	if *remove {
		if explodeDir != "" {
			os.RemoveAll(explodeDir)
			log.Printf("removed %q\n", explodeDir)
		}
		os.Remove(filename)
		log.Printf("removed %q\n", filename)
	}
	return
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
