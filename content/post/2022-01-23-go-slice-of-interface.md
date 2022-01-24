---
title: Accept Interface, Return Concrete Type, But What About Slices?
author:
  - Sharif Anani
date: '2022-01-23'
categories:
  - Go
tags:
  - Go
  - GoLang
  - Interface
toc: true
---

# 1. A Quick Introduction
One of Go's commonly known characteristics has long been the implicit satisfaction of interfaces.
Implicitly satisfying interfaces can (and has been) useful in many situations, especially those around file and network
I/O. This post isn't here to change anyone's opinion about this design choice, but rather to point out
a small inconsistency I've recently noticed through my day-to-day encounters with the language.

Although not explicitly a [Go Proverb](https://www.youtube.com/watch?v=PAAkCSZUG1c),
"accept interfaces and return concrete types" has been is something I've heard many times
throughout my short professional experience with the language. It has worked nicely for me!
Accepting (and creating) small interfaces can help reduce the maintenance effort when introducing new
functionality to an existing codebase, especially when I know that many other objects in the codebase
will already satisfy the interface would be ready to use immediately!

# 2. An Example to Illustrate
Let's take the example below, where we have:
1. A hypothetical serial port
2. A function to create an instance of it,
3. An exported method to send data through the port
4. A function to send the same data string through multiple ports

```go
type SerialPort struct {
	buffer [128]byte
	destination string
}

func (p *SerialPort) SendData(data string) error {
	_ = data
	// sends data to Destination
	return nil
}

func NewSerialPort(destination string) SerialPort {
	return SerialPort{
		destination: destination,
	}
}

func SendDataOnMultiplePorts(ports []SerialPort, data string) {
	for _, port := range ports {
		go port.SendData(data)
	}
}
```

After a few hypothetical months pass, we want to add network ports that can do pretty much the same thing!
```go
type NetworkPort struct {
	buffer [128]byte
	destination string
	ifaceId string
}

func (p *NetworkPort) SendData(data string) error {
	// sends data to Destination
	// implementation uses network ports
	_ = data
	return nil
}
func NewNetworkPort(destination string, ifaceName string) NetworkPort {
	return NetworkPort{
		destination: destination,
		ifaceId:   ifaceName
	}
}
```
Of course, we'd like for our `SendDataOnMultiplePorts` function to still work the same! What do we do here? We use an interface:

```go
// introduce the interface
type DataPort interface {
	SendData(data string) error
}

// update SendDataOnMultiplePorts
func SendDataOnMultiplePorts(ports []DataPort, data string) {
	for _, port := range ports {
		go port.SendData(data)
	}
}
```

Now, I should be able to pass a slice of `DataPort`s and data will get sent! Right?!

However, if the codebase already had a function such as the one below
```go
func main() {
	// get many serial ports
	var ports []*SerialPort
	for i := 0; i < 3; i++ {
		ports = append(ports, NewSerialPort(fmt.Sprint(i)))
	}
	SendDataOnMultiplePorts(ports, "hello")
}
```
The compiler lets us know:
```
cannot use ports (type []*SerialPort) as type []DataPort in argument to SendDataOnMultiplePorts
```

On first look, this looks rather unintuitive; `*SerialPort` *clearly* satisfies the `DataPort` interface!
However, the compiler doesn't agree.

It's not a very big deal to just change the type of `ports` to `[]DataPort`:
```go
// this will work
func main() {
	// get many serial ports
	var ports []DataPort
	for i := 0; i < 3; i++ {
		ports = append(ports, NewSerialPort(fmt.Sprint(i)))
	}
	SendDataOnMultiplePorts(ports, "hello")
}
```

However, this could be a bigger inconvenience if we were already using functions such as
```go
func MultipleNewSerialPorts(destination string, numPorts int) []*SerialPort {
	var ports []*SerialPort
	for i:=0; i<numPorts; i++ {
		ports = append(ports, NewSerialPort(fmt.Sprint(i)))
	}
}
```
and our `main` function was
```go
// this will work
func main() {
	// get many serial ports
	ports := MultipleNewSerialPorts(20)
	SendDataOnMultiplePorts(ports, "hello")
}
```
Because then we'll have to add a loop with that populates a slice of the right type
```go
// this will work
func main() {
	// get many serial ports
	ports := MultipleNewSerialPorts(20)
	var dataPorts []DataPort 
	for _, port := range ports {
		dataPorts = append(dataPorts, port)
	}
	SendDataOnMultiplePorts(dataPorts, "hello")
}
```
And that's unfortunate.
