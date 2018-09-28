# UDP Server


First off I'd like to say this was a pretty fun challenge. Its been a little while since I've had to work with udp.

### INFO

- I programmed the server in go.
- I didn't take advantage of `SO_REUSEPORT`. I first thought it wasn't available in go at all, but after looking further I'd have to make the syscall myself and manual wrap the socket. There were libraries that did this but I wanted to just use the standard library
- I didn't have time to go after the fault tolerance aspect, but I did attempt to run 4 go routines that consumed off the single socket connection.


I made a few assumptions