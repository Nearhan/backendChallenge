# UDP Server


First off I'd like to say this was a pretty fun challenge. Its been a little while since I've had to work with udp.

### INFO

- I programmed the server in go.
- I didn't take advantage of `SO_REUSEPORT`. I first thought it wasn't available in go at all, but after looking further I'd have to make the syscall myself and manual wrap the socket. There were libraries that did this but I wanted to just use the standard library
- I didn't have time to go after the fault tolerance aspect, but I did attempt to run 4 go routines that consumed off the single socket connection.
- When building with the race
- Not fault tolerant

```
❯ go build --race
```
Returns 0 errors.



I made a few assumptions.

- That messages will come over randomly. Meaning we might get a pkt from msg 1 then, then maybe a pkt from msg 5, etc etc
    - Because of this, I check if the file is completed with a timeout; this is an argument you can pass to the server, defaults to 30 seconds. Once a pkt of any message has been read by the server, the rest of the packets need to arrive before the timeout.


### Install 


### Run

This will set that checksum timeout to 5 seconds
it defaults to 30 seconds as per the request of the doc

```
go run main.go --timeout 5s 
```