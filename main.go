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
	"sync"
	"syscall"
	"time"
)

// Messages contains all messages
type Messages map[uint32]*Message

// Message is comprised of pkts
type Message struct {
	id            uint32          // message id
	set           map[uint32]bool // set used for incoming packets
	pkts          pkts            // packets
	done          chan bool
	checkSumTimer *time.Timer
	mux           *sync.Mutex
}

func (m *Message) ChecksumTimeout() {

	for {
		select {
		case <-m.checkSumTimer.C:
			m.Checksum()
			return
		case <-m.done:
			return
		}
	}
}

// Add add unique packet to message
func (m *Message) Add(p pkt) {
	m.mux.Lock()
	defer m.mux.Unlock()

	// check dupe
	if _, ok := m.set[p.offset]; !ok {
		m.set[p.offset] = true
		m.pkts = append(m.pkts, p)
	}

}

// NewMessage makes a new message type
func NewMessage(id uint32) *Message {
	t := time.NewTimer(5 * time.Second)
	done := make(chan bool)
	m := &Message{id: id, set: map[uint32]bool{}, pkts: []pkt{}, checkSumTimer: t, done: done, mux: &sync.Mutex{}}
	go m.ChecksumTimeout()
	return m
}

// Checksum generates checksum of message
func (m *Message) Checksum() {

	// sort all our pkts by offset
	sort.Sort(m.pkts)
	totalSize := 0
	payload := []byte{}

	nextOffset := uint32(0)
	missingOffsets := []uint32{}

	// range over offsets to make sure are all present
	for _, x := range m.pkts {
		totalSize += len(x.d)
		payload = append(payload, x.d...)
		if nextOffset != x.offset {
			missingOffsets = append(missingOffsets, nextOffset)
		}
		nextOffset = x.offset + uint32(x.s)
	}

	// any  missing offsets?
	if len(missingOffsets) > 0 {
		for _, off := range missingOffsets {
			fmt.Printf("message %d missing hole %d \n", m.id, off)
		}
		return
	}

	m.checkSumTimer.Stop()

	h := sha256.New()
	h.Write(payload)
	fmt.Printf("message: %d size: %d packets: %d sha256: %x \n", m.id, totalSize, len(m.pkts), h.Sum(nil))
}

// pkt a single udp packet
type pkt struct {
	offset uint32
	s      uint16
	d      []byte
}

type pkts []pkt

func (p pkts) Len() int           { return len(p) }
func (p pkts) Less(i, j int) bool { return p[i].offset < p[j].offset }
func (p pkts) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (p pkt) String() string {
	return fmt.Sprintf("%d %d %d", p.offset, p.s, len(p.d))
}

func main() {

	// start udp server
	addr := net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6789}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()

	// fatal error
	if err != nil {
		log.Fatal(err)
	}

	// create data structure
	msgs := Messages{}

	// start 4 consumers
	for i := 0; i < 4; i++ {
		go func(conn *net.UDPConn) {
			for {
				buffer := make([]byte, 512)
				_, _, err := conn.ReadFromUDP(buffer)

				if err != nil {
					return
				}

				id, pkt := decomposePkt(buffer)

				// start populating our map
				if _, ok := msgs[id]; !ok {
					msg := NewMessage(id)
					msgs[msg.id] = msg

					// lets cal checksum the last message
					if _, ok = msgs[id-1]; ok {
						msgs[id-1].Checksum()
					}
				}

				// add packet to msgs
				msgs[id].Add(pkt)
			}
		}(conn)
	}

	// handle daemon
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Println(sig)
		done <- true
	}()

	fmt.Println("awaiting")
	<-done
	fmt.Println("exiting")

}

func decomposePkt(buffer []byte) (uint32, pkt) {

	header, data := buffer[:12], buffer[12:]
	size := header[2:4]
	offset := header[4:8]
	id := header[8:]

	// convert to correct numbers
	i := binary.BigEndian.Uint32(id)
	o := binary.BigEndian.Uint32(offset)
	s := binary.BigEndian.Uint16(size)
	p := pkt{o, s, data[:s]}

	return i, p
}
