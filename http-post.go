package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

var about = `HTTP-Post-Listener

This utility is intended to listen on a port and handle post requests, saving each
file to disk and then calling an optional script.`

var (
	basePath    = flag.String("path", "output/", "Directory which to save files")
	script      = flag.String("script", "", "Shell script to be called on successful run")
	scriptShell = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	listen      = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath  = flag.String("listenPath", "/file", "Where to expect files to be posted")
	enableTLS   = flag.Bool("tls", false, "Enable TLS for secure transport")
	remove      = flag.Bool("rm", false, "Automatically remove file after script has finished")
	version     = ""
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
	fmt.Println("output set to", *basePath)

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
	defer func() {
		if fh != nil {
			fh.Close()
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
	// Flatten any of the /../ junk
	filename := filepath.Clean(r.URL.Path)

	// Verify that the right path is being hit on POST/PUT endpoint
	if !strings.HasPrefix(filename, *listenPath) {
		err = fmt.Errorf("Path not allowed %q", filename)
		return
	}

	// Build the exact path to where to put the file
	filename = path.Join(*basePath, strings.TrimPrefix(filename, *listenPath))

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
	fh.Close()
	fh = nil

	log.Println("successfully transferred", filename)

	if *script != "" {
		log.Println("Calling script", "/bin/bash", *script, filename)
		output, err := exec.Command("/bin/bash", *script, filename).Output()
		log.Println("----- START", *script, filename, "-----")
		fmt.Println(string(output))
		log.Println("----- END", *script, filename, "-----")
		if err != nil {
			log.Printf("error %s", err)
		}
	}

	if *remove {
		os.Remove(filename)
		log.Println("removed", filename)
	}
	return
}
