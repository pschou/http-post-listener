package main

import (
	"bufio"
	"errors"
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

	"github.com/inhies/go-bytesize"
	"github.com/pschou/go-exploder"
	"github.com/remeh/sizedwaitgroup"
)

var about = `HTTP-Post-Listener

This utility is intended to listen on a port and handle PUT/POST requests,
saving each file to disk and then calling an optional processing script.  The
optional explode flag will extract the file into a temporary path for deeper
inspection (like virus scanning).  The limit flag, if greater than 0, will
limit the number of concurrent uploads which are allowed at a given moment.`

var (
	basePath      = flag.String("path", "output/", "Directory which to save files")
	script        = flag.String("script", "", "Shell script to be called on successful post")
	scriptShell   = flag.String("script-shell", "/bin/bash", "Shell to be used for script run")
	listen        = flag.String("listen", ":8080", "Where to listen to incoming connections (example 1.2.3.4:8080)")
	listenPath    = flag.String("listenPath", "/file", "Where to expect files to be posted")
	enableTLS     = flag.Bool("tls", false, "Enable TLS for secure transport")
	enforce2way   = flag.Bool("2way", false, "Enforce two way SSL validation")
	remove        = flag.Bool("rm", false, "Automatically remove file after script has finished")
	enforceTokens = flag.Bool("enforce-tokens", false, "Enforce tokens, otherwise match only if one is provided")
	tokens        = flag.String("tokens", "", "File to specify tokens for authentication")
	dnList        = flag.String("dn-list", "", "File to specify DNs for authentication.\nIf provided the client must authenticate by presenting a certificate.")
	explode       = flag.String("explode", "", "Directory in which to explode an archive into for inspection")
	maxSize       = flag.String("max", "", "Maximum upload size permitted (for example: -max=8GB)")
	allowDup      = flag.Bool("allowDup", false, "Allow duplicate file names, but the first must complete before the replacement is sent")
	limit         = flag.Int("limit", 0, "Limit the number of uploads processed at a given moment to avoid disk bloat")
	tokenMap      = make(map[string]string)
	dnMap         = make(map[string]struct{})
	version       = ""

	swg           sizedwaitgroup.SizedWaitGroup
	errorTooLarge = errors.New("Upload too large")
	limitSize     int64
	enforceDNs    bool
)

func main() {
	flag.Usage = func() {
		lines := strings.SplitN(about, "\n", 2)
		fmt.Fprintf(os.Stderr, "%s (github.com/pschou/http-post-listener, version: %s)\n%s\n\nUsage: %s [options]\n",
			lines[0], version, lines[1], os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
The script has environment variables set if they are specified, including:
	HTTP_POST_FILE - Filename of the uploaded file (relative path from the CWD)
	HTTP_POST_GROUP - Group name from the Token file (this will be empty if no token was provided)
	HTTP_POST_EXPLODE_DIR - Extracted directory path from the upload, extracted down multiple layers
	HTTP_POST_CLIENT_DN - If there was a mutual TLS connection, this will be filled out with the DN
	HTTP_POST_ISSUER_DN - Ditto ^, this will be filled out with the issuer's DN`)
		fmt.Fprintln(os.Stderr, cipher_list)
	}

	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Println("Unrecognized flags:", flag.Args())
		flag.Usage()
		os.Exit(1)
	}
	if *enableTLS {
		loadTLS()
	}
	fmt.Println("Output set to", *basePath)

	if *maxSize != "" {
		bs, err := bytesize.Parse(*maxSize)
		if err != nil {
			fmt.Println("Invalid max size:", *maxSize, ",", err)
			os.Exit(1)
		}
		fmt.Println("Max set to:", bs.String())
		limitSize = int64(bs)
	}

	if *limit > 0 {
		fmt.Println("Sized wait group set to:", *limit)
		swg = sizedwaitgroup.New(*limit)
	}

	if *dnList != "" {
		enforceDNs = true
		if fh, err := os.Open(*dnList); err != nil {
			log.Fatal(err)
		} else {
			scanner := bufio.NewScanner(fh)
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "#", 2)
				dn := strings.TrimSpace(parts[0]) // Remove any extra spaces
				if len(dn) > 3 {
					dnMap[strings.ToLower(dn)] = struct{}{}
				}
			}
			fh.Close()
		}
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

	// Note: ServeMux also takes care of sanitizing the URL request path,
	// redirecting any request containing . or .. elements or repeated slashes to
	// an equivalent, cleaner URL.
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
	var clientDN, issuerDN string
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		clientDN = strings.TrimSpace(certPKIXString(r.TLS.PeerCertificates[0].Subject, ","))
		issuerDN = strings.TrimSpace(certPKIXString(r.TLS.PeerCertificates[0].Issuer, ","))
	}
	if enforceDNs {
		if r.TLS == nil {
			log.Println("Client didn't negotiate with TLS", r.RemoteAddr)
			return
		}
		if len(r.TLS.PeerCertificates) == 0 {
			log.Println("Client didn't present TLS creds", r.RemoteAddr)
			return
		}
		if _, found := dnMap[strings.ToLower(clientDN)]; !found {
			log.Println("Client dn was not found:", certPKIXString(r.TLS.PeerCertificates[0].Subject, ","))
			return
		}
	}

	var fh *os.File
	var filename string
	var uploadSize int64
	defer func() {
		if fh != nil {
			fh.Close()
			os.Remove(filename)
		}
		if err == errorTooLarge {
			errDetail := fmt.Sprintf("Error: File size limit %q, upload too large %d > %d", filename, uploadSize, limitSize)
			log.Println(errDetail)
			http.Error(w, errDetail, http.StatusRequestEntityTooLarge)
		} else if err != nil {
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

	// Verify that the right path is being hit on POST/PUT endpoint, an extra
	// sanity check is done
	if !strings.HasPrefix(r.URL.Path, *listenPath) || strings.Index(r.URL.Path, "..") >= 0 {
		err = fmt.Errorf("Path not allowed %q", filename)
		return
	}

	// This takes the prefix and removes it.
	filename = strings.TrimPrefix(r.URL.Path, *listenPath)

	// This Clean will fix the path format to match the local OS.
	filename = filepath.Clean(filename)

	// This will build the path to where to put the file
	filename = path.Join(*basePath, filename)

	if *limit > 0 {
		// Create a delay in the upload process here in case we have maxed out the
		// number of upload slots.  The choice of doing this AFTER the
		// authentication will then help the system absorb the to-be-failed events
		// by killing off the session early and freeing up the socket.  The idea
		// here is: by this point in the code, an upload is bound to be successful,
		// so now we'll start the wait group counter.
		swg.Add()
		defer swg.Done()
	}

	// Make sure the directory exists
	dir, _ := path.Split(filename)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}

	if *allowDup {
		Lock(filename)
		defer Unlock(filename)
		if stat, err := os.Stat(filename); err == nil {
			if stat.Mode().IsRegular() {
				os.Remove(filename)
			} else {
				log.Printf("Cannot remove file as the path is not a regualar file %q", filename)
			}
		}
	}

	// Open the file for writing.  If it already exists, do not allow overwrite.
	if fh, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666); err != nil {
		return
	}

	log.Printf("recieving file %q...\n", filename)

	// Copy the stream to disk into the final file location (at wire speed)
	if limitSize <= 0 {
		// No limit, copy everything (slurrrrp!)
		if _, err = io.Copy(fh, r.Body); err != nil {
			return
		}
	} else {
		// Copy while imposing a LimitReader restriction:
		if uploadSize, err = io.Copy(fh, io.LimitReader(r.Body, limitSize)); err != nil {
			return
		}
		// Try to read more, this will only happen if the limit has been reached.
		// By reading off bytes to the end of the upload file, we can then return a
		// sensible error response.
		if extra, _ := io.Copy(io.Discard, r.Body); extra > 0 {
			uploadSize += extra
			err = errorTooLarge
			return
		}
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

	// If a script is provided, call it with the arguments in this order:
	//
	// [1] Filename of the uploaded file (relative path from the CWD)
	// [2] Group name from the Token file (this will be empty if no token was provided)
	// [3] Extracted directory path from the upload, extracted down multiple layers
	//
	if *script != "" {
		cmd := exec.Command(*scriptShell, *script, filename, group, explodeDir)
		if clientDN != "" {
			cmd.Env = append(cmd.Env, "HTTP_POST_CLIENT_DN="+clientDN)
		}
		if issuerDN != "" {
			cmd.Env = append(cmd.Env, "HTTP_POST_ISSUER_DN="+clientDN)
		}
		if filename != "" {
			cmd.Env = append(cmd.Env, "HTTP_POST_FILE="+filename)
		}
		if group != "" {
			cmd.Env = append(cmd.Env, "HTTP_POST_GROUP="+group)
		}
		if explodeDir != "" {
			cmd.Env = append(cmd.Env, "HTTP_POST_EXPLODE_DIR="+explodeDir)
		}
		log.Printf("Calling script: %s %s %q %q %q\n", *scriptShell, *script, filename, group, explodeDir)
		output, err := cmd.Output()
		log.Println("----- START", *script, filename, "-----")
		fmt.Println(string(output))
		log.Println("----- END", *script, filename, "-----")
		if err != nil {
			log.Printf("error %s", err)
		}
	}

	// Clean up time
	if *remove {
		if explodeDir != "" {
			// Remove the exploded directory
			os.RemoveAll(explodeDir)
			log.Printf("removed %q\n", explodeDir)
		}
		// Remove the uploaded file
		os.Remove(filename)
		log.Printf("removed %q\n", filename)
	}
	return
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// This is used to generate random filenames for the extracted directory
func RandStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
