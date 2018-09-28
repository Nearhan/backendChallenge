package main

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"flag"
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

var timeOut = flag.Duration("timeout", time.Second*30, "time to check if a message should be checked")

// Messages contains all messages
type Messages struct {
	msgs map[uint32]*Message
	mux  *sync.Mutex
}

// NewMessages builds message store
func NewMessages() *Messages {
	return &Messages{
		msgs: map[uint32]*Message{},
		mux:  &sync.Mutex{},
	}
}

// AddMessage ...
func (m *Messages) AddMessage(msg *Message) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.msgs[msg.id] = msg
}

// GetMessage ...
func (m *Messages) GetMessage(id uint32) (*Message, bool) {
	m.mux.Lock()
	defer m.mux.Unlock()

	if msg, ok := m.msgs[id]; !ok {
		return nil, false
	} else {
		return msg, true
	}

}

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
	t := time.NewTimer(*timeOut)
	done := make(chan bool)

	m := &Message{
		id:            id,
		set:           map[uint32]bool{},
		pkts:          []pkt{},
		checkSumTimer: t,
		done:          done,
		mux:           &sync.Mutex{},
	}

	go m.ChecksumTimeout()
	return m
}

// Checksum generates checksum of message
func (m *Message) Checksum() {
	m.mux.Lock()
	defer m.mux.Unlock()

	// sort all our pkts by offset
	sort.Sort(m.pkts)
	totalSize := 0
	payload := []byte{}

	nextOffset := uint32(0)
	missingOffsets := []uint32{}

	// range over offsets to make sure all are  present
	for _, x := range m.pkts {
		totalSize += len(x.d)
		payload = append(payload, x.d...)
		if nextOffset != x.offset {
			missingOffsets = append(missingOffsets, nextOffset)
		}
		nextOffset = x.offset + uint32(x.s)
	}

	// any missing offsets?
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

func parsePacket(buffer []byte) (uint32, pkt, error) {

	if len(buffer) < 12 {
		return 0, pkt{}, errors.New("buffer header missing")

	}

	header, data := buffer[:12], buffer[12:]
	size := header[2:4]
	offset := header[4:8]
	id := header[8:]

	// convert to correct numbers
	i := binary.BigEndian.Uint32(id)
	o := binary.BigEndian.Uint32(offset)
	s := binary.BigEndian.Uint16(size)
	p := pkt{o, s, data[:s]}
	return i, p, nil
}

// main
func main() {
	// parse our the flags
	flag.Parse()

	// start udp server
	addr := net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6789}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()

	// fatal error
	if err != nil {
		log.Fatal(err)
	}

	// create data structure
	msgs := NewMessages()

	// start 4 consumers to read off the socket
	for i := 0; i < 4; i++ {
		go func(conn *net.UDPConn, msgs *Messages) {
			for {

				// start reading off conn
				buffer := make([]byte, 512)
				read, _, err := conn.ReadFromUDP(buffer)

				if read == 0 || err != nil {
					fmt.Println("reading from the socket failed")
					return
				}

				id, pkt, err := parsePacket(buffer)
				// buffer isn't correct length
				if err != nil {
					fmt.Println(err)
					continue
				}

				// start populating our map
				msg, ok := msgs.GetMessage(id)
				if !ok {
					msg = NewMessage(id)
					msgs.AddMessage(msg)

					/*
						if lstMsg, ok := msgs.GetMessage(id - 1); ok {
							//lstMsg.Checksum()
						}
					*/
				}

				// add packet to msgs
				msg.Add(pkt)
			}
		}(conn, msgs)
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
