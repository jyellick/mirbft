package network

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/sample"
)

type Proposer interface {
	Proposal(nodes, i int) (uint64, []byte)
}

type LinearProposer struct{}

func (LinearProposer) Proposal(nodes, i int) (uint64, []byte) {
	return uint64(i), Uint64ToBytes(uint64(i))
}

type SkippingProposer struct {
	NodesToSkip []int
}

func (sp SkippingProposer) Proposal(nodes, i int) (uint64, []byte) {
	nonSkipped := nodes - len(sp.NodesToSkip)
	alreadySkipped := i / nonSkipped * len(sp.NodesToSkip)
	for j := 0; j <= i%nonSkipped; j++ {
		for _, n := range sp.NodesToSkip {
			if n == j {
				alreadySkipped++
			}
		}
	}

	next := uint64(i + alreadySkipped)
	return next, Uint64ToBytes(next)
}

type FakeLog struct {
	Entries []*pb.QEntry
	CommitC chan *pb.QEntry
}

func (fl *FakeLog) Apply(entry *pb.QEntry) {
	if entry.Requests == nil {
		// this is a no-op batch from a tick, or catchup, ignore it
		return
	}
	fl.Entries = append(fl.Entries, entry)
	fl.CommitC <- entry
}

func (fl *FakeLog) Snap() []byte {
	return Uint64ToBytes(uint64(len(fl.Entries)))
}

type TestConfig struct {
	NodeCount        int
	BucketCount      int
	MsgCount         int
	Proposer         Proposer
	TransportFilters []func(Transport) Transport
	Expectations     TestExpectations
}

type TestExpectations struct {
	Epoch *uint64
}

func Uint64ToPtr(value uint64) *uint64 {
	return &value
}

func Uint64ToBytes(value uint64) []byte {
	byteValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(byteValue, value)
	return byteValue
}

func BytesToUint64(value []byte) uint64 {
	return binary.LittleEndian.Uint64(value)
}

type Network struct {
	nodes      []*mirbft.Node
	fakeLogs   []*FakeLog
	processors []*sample.SerialProcessor
}

func (n *Network) GoRunNetwork(doneC <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(len(n.nodes))
	for i := range n.nodes {
		go func(i int, doneC <-chan struct{}) {
			defer GinkgoRecover()
			defer wg.Done()

			// TODO, these tests are flaky, and non-deterministic.
			// they need to be re-written onto a single thread with determinism
			// and the crazy high-stress non-deterministic ones moved to
			// a test requiring fewer constraints on the results.
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case actions := <-n.nodes[i].Ready():
					results := n.processors[i].Process(&actions)
					n.nodes[i].AddResults(*results)
				case <-n.nodes[i].Err():
					_, err := n.nodes[i].Status(context.Background())
					Expect(err).To(MatchError(mirbft.ErrStopped))
					return
				case <-ticker.C:
					n.nodes[i].Tick()
				}
			}
		}(i, doneC)
	}
}
