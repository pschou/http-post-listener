# http-post

A simple HTTP POST/PUT handler

This utility is intended to listen on a port and handle PUT/POST requests,
saving each file to disk and then calling an optional processing script.  The
optional explode flag will extract the file into a temporary path for deeper
inspection (like virus scanning).  The limit flag, if greater than 0, will
limit the number of concurrent uploads which are allowed at a given moment.

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
