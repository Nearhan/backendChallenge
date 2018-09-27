package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sort"
	"syscall"
)

// Messages contains all messages
type Messages map[uint32]Message

// Message is comprised of pkts
type Message struct {
	id   uint32        // message id
	set  map[uint]bool // set used for incoming packets
	pkts pkt           // packets
}

// pkt a single udp packet
type pkt struct {
	offset uint32
	s      uint16
	d      []byte
}

type pkts []pkt

func (p pkts) Calculate() string {
	sort.Sort(p)
	ts := 0
	b := []byte{}

	nextOffset := uint32(0)
	for _, x := range p {
		ts += len(x.d)
		b = append(b, x.d...)
		if nextOffset != x.offset {
			fmt.Printf("hole at %d", nextOffset)
		} else {
			nextOffset = x.offset + uint32(x.s)
		}

	}
	h := sha256.New()
	h.Write(b)
	return fmt.Sprintf("size: %d packets: %d hash: %x ", ts, len(p), h.Sum(nil))

}

func (p pkts) Len() int           { return len(p) }
func (p pkts) Less(i, j int) bool { return p[i].offset < p[j].offset }
func (p pkts) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (p pkt) String() string {
	return fmt.Sprintf("%d %d %d", p.offset, p.s, len(p.d))
}

func main() {

	// start udp server
	addr := net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6789}
	server, err := net.ListenUDP("udp", &addr)
	defer server.Close()

	// check error
	if err != nil {
		log.Fatal(err)
	}

	// create data structure
	offsetsList := map[uint32]pkts{}
	old := uint32(1)
	fmt.Println(old)

	go func() {
		for {

			buffer := make([]byte, 512)
			read, _, err := server.ReadFromUDP(buffer)

			if err != nil {
				fmt.Println(read)
				fmt.Println(err)
			}

			id, offset, size, data := decomposePkt(buffer)

			if old == id {

				list := offsetsList[id]
				list = append(list, pkt{offset, size, data})
				offsetsList[id] = list

			} else {

				//calculate something
				fmt.Printf("message # %d ", old)
				fmt.Println(offsetsList[old].Calculate())
				old = id

				if _, ok := offsetsList[old]; !ok {
					offsetsList[old] = pkts{pkt{offset, size, data}}

				}

			}
		}
	}()

	// values on a channel. We'll create a channel to
	// receive these notifications (we'll also make one to
	// notify us when the program can exit).
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	// `signal.Notify` registers the given channel to
	// receive notifications of the specified signals.
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// This goroutine executes a blocking receive for
	// signals. When it gets one it'll print it out
	// and then notify the program that it can finish.
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		done <- true
	}()

	// The program will wait here until it gets the
	// expected signal (as indicated by the goroutine
	// above sending a value on `done`) and then exit.
	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
	fmt.Println(offsetsList[10].Calculate())

}

func decomposePkt(buffer []byte) (uint32, uint32, uint16, []byte) {

	header, data := buffer[:12], buffer[12:]
	size := header[2:4]
	offset := header[4:8]
	id := header[8:]

	// convert to correct numbers
	i := binary.BigEndian.Uint32(id)
	o := binary.BigEndian.Uint32(offset)
	s := binary.BigEndian.Uint16(size)

	return i, o, s, data[:s]
}
