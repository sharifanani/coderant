---
title: How good of a substitute are protocol buffers to JSON?
author:
  - Sharif Anani
date: 2022-02-17
categories:
  - Communication
tags:
  - Protobuf
  - Protocol Buffers
  - JSON
  - Serialization
  - Code Generation
toc: true
---

## Why are we here?
Over the past year or two, I've come within smelling distance of [Protocol Buffers](https://developers.google.com/protocol-buffers)
a few times, but I never actually got to touch them, do anything with them, or even contribute to a project where protocol
buffers were being utilized. Figured writing a blog post about it is good enough reason to give it a shot!

## What are we going to do with protobufs?
In this post, we're going to try to use protocol buffers to encode data coming in on a network socket and write it to disk.

## The use case

We have a hypothetical server, written in Go, and listens on a socket.
We'll send some structured JSON data to the server in a format it is expecting, and will write that information to a file.
Something like `./proto -listenTo sock1`. We'll use JSON as the first format, and then substitute it with protocol buffers
and see how we like it. Throughout, we'll be using `socat` with the `ABSTRACT-CONNECT:<string>` option to send
data to our server. Docs about `socat` usage are available [here](http://www.dest-unreach.org/socat/doc/socat.html)

Shall we?

## The Go server

Without further ado, our Go program is pretty simple here. The project hierarchy looks like:
```shell
.
├── go
│   ├── build
│   ├── go.mod
│   └── proto_demo
│       └── main.go
└── test_msg.json
```
I used `go mod init protobuf_demo_server` to create my module, but you can name yours whatever you want.

And `main.go`:
```go
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
)

type User struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	Id      int    `json:"id"`
	Content string `json:"content"`
	Sender  User   `json:"sender"`
}

var logger = log.Default()

var tempFile *os.File = nil

func getTempFile() (*os.File, error) {
	var err error
	if tempFile == nil {
		tempFile, err = ioutil.TempFile(".", "socket-")
		if err != nil {
			return nil, fmt.Errorf("error getting tempFile: %v\n", err)
		}
	}
	return tempFile, nil
}

func writeToDisk(message *Message) error {
	file, err := getTempFile()
	if err != nil {
		return fmt.Errorf("error getting temp file: %v\n", err)
	}
	err = file.Truncate(0)
	if err != nil {
		return err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	err = encoder.Encode(message)
	if err != nil {
		return fmt.Errorf("error decoding: %v\n", err)
	}
	return nil
}

func startSocketListener(address string) {
	listener, err := net.Listen("unix", address)
	if err != nil {
		logger.Fatalf("error dialing socket: %v\n", err)
	}
	logger.Printf("Listening on socket %v\n", address)
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Printf("error reading accepted connection: %v\n", err)
			continue
		}
		content, err := io.ReadAll(conn)
		if err != nil {
			logger.Printf("error reading: %v\n", err)
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(content))
		var msg Message
		err = decoder.Decode(&msg)
		if err != nil {
			logger.Printf("error decoding: %v\n", err)
			continue
		}
		err = writeToDisk(&msg)
		if err != nil {
			logger.Printf("error writing to disk: %v\n", err)
			continue
		}
		fmt.Println(string(content))
	}
}

func main() {
	defer func() {
		_ = tempFile.Close()
	}()
	// use an abstract namespace socket to avoid having to manage a file
	listenIn := flag.String("listenTo", "sock1", "the name of the abstract namespace socket to listen on")
	flag.Parse()
	listenTo := fmt.Sprintf("@%s", *listenIn)
	startSocketListener(listenTo)
}
```

For the test message, I'll be using
```json
{"id":0,"content":"Hello World!","sender":{"id":0,"name":"robotHamster"}}
```
Which I have saved in `test_msg.json`, but feel free to use whatever you want! Having it in a file will come in handy.

We can build the server by running
```shell
$ go build -o ./build/proto proto_demo/main.go
```

And then do something like:
```shell
> pwd
/project/dir/go/build
```
```shell
> ./proto -listenTo sock1
```
Which will start the server and show you some output
```shell
2022/02/17 21:31:46 Listening on socket @sock1
```
In a separate terminal, run the following command:

```shell
$ cat test_msg.json | socat - ABSTRACT-CONNECT:sock1
```
Which pipes the contents of the file through `stdin` to the socket.

In the same directory as your executable, you should now find a `sock-number` file, and its contents will be your message!

```shell
$ cat go/build/socket-1618341842
{"id":0,"content":"Hello World!","sender":{"id":0,"name":"robotHamster"}}
```
Cool! that was the easy part!

## Hello protobufs

To start with protocol buffers, it may be useful to read some of the [official documentation](https://developers.google.com/protocol-buffers/docs/proto3)
if you haven't done so.

In summary, protocol buffers are basically files describing a schema, and these files are used by the protocol buffer compiler
(`protoc`) to generate software packages that allow you to create and manipulate objects abiding by said schema in different languages.

Pretty fancy-sounding, eh?

Let's go ahead and create `<project-root>/my_message.proto` with the following contents:
```protobuf
syntax = "proto3";

option go_package = "./my_message";

message User {
  int64 id = 1;
  string name = 2;
}

message Message {
  int64 id = 1;
  string content = 2;
  User sender = 3;
}
```

In order to compile the protocol buffer into a go package, we'll want to install 2 things:

1. `protoc`, which can be installed from binary distributions at [their GitHub page](https://github.com/protocolbuffers/protobuf/releases/).
Make sure you download one of the `protoc-<version>-<os>-<arch>` releases, not the `protobuf-<lang>-<version>` releases.
I placed mine under `~/.local/share/bin/protoc`, but it should work as long as it's in one of the folders in your `PATH`.

2. Because we're using Go, there's a separate dependency that would need to be installed with
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

Alright. Once that's done, you can compile them with
```shell
$ protoc --go_out=./go/ my_message.proto
```
Then go into your `go/` folder and run `go mod tidy`. Your project now looks like this:

```shell
.
├── go
│   ├── build
│   ├── go.mod
│   ├── go.sum
│   ├── my_message
│   │   └── my_message.pb.go
│   └── proto_demo
│       └── main.go
├── my_message.proto
└── test_msg.json
```

We *could* take a look inside `my_message.pb.go`, but that kinda defeats the purpose, doesn't it? We'll leave it alone for now.

## Use protobufs to write to disk

Now that we have everything in place, let's make the necessary changes to make our server write a protobuf-serialized
version of our payload instead of the JSON one.

We can do this by updating our `writeToDisk` function to be the following

```go
// new import 	"protobuf_demo_server/my_message"
// new import	"google.golang.org/protobuf/proto"
func writeToDisk(message *Message) error {
	file, err := getTempFile()
	if err != nil {
		return fmt.Errorf("error getting temp file: %v\n", err)
	}
	err = file.Truncate(0)
	if err != nil {
		return err
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}
	msgAsProtobuf := my_message.Message{
		Id:      int64(message.Id),
		Content: message.Content,
		Sender:  &my_message.User{
			Id:   int64(message.Sender.Id),
			Name: message.Sender.Name,
		},
	}
	buf, err := proto.Marshal(&msgAsProtobuf)
	if err != nil {
		return fmt.Errorf("error marshaling: %v\n", err)
	}
	_, err = file.Write(buf)
	if err != nil {
		return fmt.Errorf("error writing: %v\n", err)
	}
	return nil
}
```

Now, let's re-build and run our server again!

```shell
$ go build -o ./build/proto proto_demo/main.go
$ ./build/proto -listenTo sock1
2022/02/17 22:22:07 Listening on socket @sock1
```

And send it the same data again

```shell
$ cat test_msg.json | socat - ABSTRACT-CONNECT:sock1
```

If we take a look inside the new file, we'll see that it's encoded differently:
```
> cat go/socket-74588476
                                                                                                                                                                                                              Hello World�
           robotHamster⏎
```
Hideous!


We can actually get a better view by using `protoc`:
```shell
$ cat go/socket-74588476 | protoc --decode_raw
1: 2
2: "Hello World!"
3 {
  1: 300
  2: "robotHamster"
}

```

Or, if we really wanted to take a good look:

```shell
$ cat go/socket-74588476 | protoc --decode Message my_message.proto
id: 2
content: "Hello World!"
sender {
  id: 300
  name: "robotHamster"
}
```

You'll notice that in the last payload, I changed the IDs of the message and sender. If I left them as `0`, they would
have been left out as those are the default values for their types. Super space efficient.

## What next?

Next post, I think I'll create a python CLI that prompts the user for message information, builds it (in a protobuf),
and sends it over to the go server.
IMO that would help show how protobufs can be just as portable as JSON, while having the added benefit of less space
usage, faster encode/decode, and unifying object definitions across languages.

The code for this post is available on https://github.com/sharifanani/protobufs_demo, and it'll be updated with the python CLI
when that comes about 😁