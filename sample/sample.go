/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"context"
	"fmt"
	"hash"
	"runtime/debug"
	"time"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
)

type ValidatorFunc func(*mirbft.Request) error

func (vf ValidatorFunc) Validate(request *mirbft.Request) error {
	return vf(request)
}

type Hasher func() hash.Hash

type Validator interface {
	Validate(*mirbft.Request) error
}

type Link interface {
	Send(dest uint64, msg *pb.Msg)
}

type Log interface {
	Apply(*pb.QEntry)
	Snap() (id []byte)
}

type SerialCommitter struct {
	Log                    Log
	LastCommittedSeqNo     uint64
	OutstandingSeqNos      map[uint64]*mirbft.Commit
	OutstandingCheckpoints map[uint64]struct{}
}

func (sc *SerialCommitter) Commit(commits []*mirbft.Commit) []*mirbft.CheckpointResult {
	for _, commit := range commits {
		// Note, this pattern is easy to understand, but memory inefficient.
		// A ring buffer of size equal to the log size would produce far less
		// garbage.
		sc.OutstandingSeqNos[commit.QEntry.SeqNo] = commit
	}

	var results []*mirbft.CheckpointResult

	for currentSeqNo := sc.LastCommittedSeqNo + 1; len(sc.OutstandingSeqNos) > 0; currentSeqNo++ {
		entry, ok := sc.OutstandingSeqNos[currentSeqNo]
		if !ok {
			break
		}
		sc.Log.Apply(entry.QEntry) // Apply the entry
		sc.LastCommittedSeqNo = currentSeqNo
		delete(sc.OutstandingSeqNos, currentSeqNo)

		if entry.Checkpoint {
			value := sc.Log.Snap()
			results = append(results, &mirbft.CheckpointResult{
				SeqNo: sc.LastCommittedSeqNo,
				Value: value,
			})
		}
	}

	return results
}

type SerialProcessor struct {
	Link      Link
	Validator Validator
	Hasher    Hasher
	Committer *SerialCommitter
	Node      *mirbft.Node
	PauseC    chan struct{}
	DoneC     <-chan struct{}
}

func (c *SerialProcessor) Process(actions *mirbft.Actions) *mirbft.ActionResults {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n\n!!! Recovered from crash: %v \nPrinting state machine status\n", r)
			debug.PrintStack()
			ctx, cancel := context.WithTimeout(context.TODO(), 50*time.Millisecond)
			defer cancel()
			status, err := c.Node.Status(ctx)
			if err != nil {
				fmt.Printf("Could not get status: %s", err)
			} else {
				fmt.Printf("\n%s\n", status.Pretty())
			}
			panic(r)
		}
	}()

	actionResults := &mirbft.ActionResults{
		Preprocessed: make([]*mirbft.PreprocessResult, len(actions.Preprocess)),
		Processed:    make([]*mirbft.ProcessResult, len(actions.Process)),
	}

	for _, broadcast := range actions.Broadcast {
		for _, replica := range c.Node.Replicas {
			if replica.ID == c.Node.Config.ID {
				c.Node.Step(context.TODO(), replica.ID, broadcast)
			} else {
				c.Link.Send(replica.ID, broadcast)
			}
		}
	}

	for _, unicast := range actions.Unicast {
		c.Link.Send(unicast.Target, unicast.Msg)
	}

	for i, request := range actions.Preprocess {
		invalid := false
		if err := c.Validator.Validate(request); err != nil {
			invalid = true
		}

		h := c.Hasher()

		actionResults.Preprocessed[i] = &mirbft.PreprocessResult{
			RequestData: request.ClientRequest,
			Digest:      h.Sum(request.ClientRequest.Data),
			Invalid:     invalid,
		}
	}

	for i, batch := range actions.Process {
		h := c.Hasher()
		for _, preprocessResult := range batch.Requests {
			h.Write(preprocessResult.Digest)
		}

		actionResults.Processed[i] = &mirbft.ProcessResult{
			Batch:  batch,
			Digest: h.Sum(nil),
		}
	}

	actionResults.Checkpoints = c.Committer.Commit(actions.Commits)

	return actionResults
}

type FakeLink struct {
	Buffers map[uint64]chan *pb.Msg
}

func NewFakeLink(source uint64, nodes []*mirbft.Node, doneC <-chan struct{}) *FakeLink {
	buffers := map[uint64]chan *pb.Msg{}
	for _, node := range nodes {
		if node.Config.ID == source {
			continue
		}
		buffer := make(chan *pb.Msg, 1000)
		buffers[node.Config.ID] = buffer
		go func(node *mirbft.Node) {
			for {
				select {
				case msg := <-buffer:
					node.Step(context.TODO(), source, msg)
				case <-doneC:
					return
				}
			}
		}(node)
	}
	return &FakeLink{
		Buffers: buffers,
	}
}

func (fl *FakeLink) Send(dest uint64, msg *pb.Msg) {
	fl.Buffers[dest] <- msg
}
