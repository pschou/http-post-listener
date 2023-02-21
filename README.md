# http-post

A simple HTTP POST/PUT handler

The intention of this program is to be a light weight file receiver and
processor.  The flags provided via the command line will setup the listener,
the keys, and the output directories.

When a file is uploaded to a given path, the prefix must match waht is
expected, in the example below this is the /file directory.  The sub-paths are
created to match the directory structure.  If a file already exists, an error
will be thrown back and nothing will be transferred.  Any partial or incomplete
downloads are deleted so re-uploads of the same file will not be prevented and
the disk will not fill up.

When a `-script` option is provided, the script will be ran with arguments as
follows:

1. Path to the original file
2. Group name (if specified in a token file)
3. An exploded directory with the contents of the archive (if the file is an
	 archive)

The exploded directory is a recursive extraction of any archived file in the
transfer payload.  This way deep inspections of the contents of the uploaded
files may be done inside the script.

When the `-rm` option is specified, the file will be deleted after the script
is done processing.

By providing a directory to the `-explode` flag, a sub directory will be
created with each upload's contents.  The directory format is as follows:
[provided path]/[random string]/[contents of archives...].  Whenever a zip file
is provided, the zip file's name will instead be a directory and the contents
of that directory will be the contents of that zip file.  Effectively, every
archive is expanded to disk for deep inspection.

## Example:

Server side:
```
$ ./http-post
output set to output/
2023/02/03 15:15:52 Listening with HTTP on :8080 at /file
2023/02/03 15:15:54 recieving file "output/123"...
2023/02/03 15:15:54 successfully transferred output/123
```

Client side to trigger an upload
```
$ echo -n MyData > test.file
$ curl -vvv -k -m 1000 -f -T test.file -H "X-Private-Token: 123" http://localhost:8080/file/123
* About to connect() to localhost port 8080 (#0)
*   Trying ::1...
* Connected to localhost (::1) port 8080 (#0)
> PUT /file/123 HTTP/1.1
> User-Agent: curl/7.29.0
> Host: localhost:8080
> Accept: */*
> Content-Length: 6
> Expect: 100-continue
>
< HTTP/1.1 100 Continue
* We are completely uploaded and fine
< HTTP/1.1 200 OK
< Date: Fri, 03 Feb 2023 20:13:21 GMT
< Content-Length: 0
<
* Connection #0 to host localhost left intact
```

Server side with script argument:
```
$ ./http-post  -script script.sh -rm
output set to output/
2023/02/03 15:18:42 Listening with HTTP on :8080 at /file
2023/02/03 15:18:44 recieving file "output/123"...
2023/02/03 15:18:44 successfully transferred output/123
2023/02/03 15:18:44 Calling script /bin/bash script.sh output/123
2023/02/03 15:18:44 ----- START script.sh output/123 -----
Got file output/123
md5=cdc1c966899bb156ae7fd772af756459 output/123

2023/02/03 15:18:44 ----- END script.sh output/123 -----
2023/02/03 15:18:44 removed output/123
```

Using tokens for validation:
```
$ curl -vvv -k -m 1000 -f -T test.file -H "X-Private-Token: 123" http://localhost:8080/file/123
```

```
$ ./http-post -tokens token.yml  -script script.sh  -rm
output set to output/
2023/02/03 15:44:29 Listening with HTTP on :8080 at /file
2023/02/03 15:44:31 recieving file "output/123"...
2023/02/03 15:44:31 successfully transferred output/123
2023/02/03 15:44:31 Calling script /bin/bash script.sh output/123 aGroup
2023/02/03 15:44:31 ----- START script.sh output/123 -----
Got file output/123
md5=0d599f0ec05c3bda8c3b8a68c32a1b47 output/123

2023/02/03 15:44:31 ----- END script.sh output/123 -----
2023/02/03 15:44:31 removed output/123
```

## Usage

```
$ ./http-post -h
HTTP-Post-Listener (github.com/pschou/http-post-listener, version: 0.1.20230220.2240)

This utility is intended to listen on a port and handle post requests, saving each
file to disk and then calling an optional script.

Usage: ./http-post [options]
  -CA string
        A PEM encoded CA's certificate file. (default "someCertCAFile")
  -cert string
        A PEM encoded certificate file. (default "someCertFile")
  -explode string
        Directory in which to explode an archive into for inspection
  -key string
        A PEM encoded private key file. (default "someKeyFile")
  -listen string
        Where to listen to incoming connections (example 1.2.3.4:8080) (default ":8080")
  -listenPath string
        Where to expect files to be posted (default "/file")
  -path string
        Directory which to save files (default "output/")
  -rm
        Automatically remove file after script has finished
  -script string
        Shell script to be called on successful post
  -script-shell string
        Shell to be used for script run (default "/bin/bash")
  -tls
        Enable TLS for secure transport
  -tokens string
        File to specify tokens for authentication
```
