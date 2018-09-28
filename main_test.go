package main

import (
	"testing"
)

func TestCheckSumPass(t *testing.T) {

	m := NewMessage(uint32(1))
	m.pkts = []pkt{
		pkt{
			uint32(0),
			uint16(2),
			[]byte("te"),
		},
		pkt{
			uint32(2),
			uint16(2),
			[]byte("te"),
		},
		pkt{
			uint32(4),
			uint16(2),
			[]byte("te"),
		},
	}

	_, missingOffsets := m.Checksum()
	if len(missingOffsets) > 0 {
		t.Error("should be no missing offsets")
	}
}

func TestCheckSumFail(t *testing.T) {
	m := NewMessage(uint32(1))
	m.pkts = []pkt{
		pkt{
			uint32(0),
			uint16(2),
			[]byte("te"),
		}, pkt{
			uint32(8),
			uint16(2),
			[]byte("te"),
		},
	}

	_, missingOffsets := m.Checksum()
	if len(missingOffsets) > 1 {
		t.Error("should be 1 missing offset")
	}

}
