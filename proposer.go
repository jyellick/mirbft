/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mirbft

import (
	"container/list"
	"encoding/binary"

	pb "github.com/IBM/mirbft/mirbftpb"
)

func uint64ToBytes(value uint64) []byte {
	byteValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(byteValue, value)
	return byteValue
}

type proposer struct {
	myConfig        *pb.StateEvent_InitialParameters
	proposalBuckets map[bucketID]*proposalBucket
	readyIterator   *readyIterator
	totalBuckets    int
}

type proposalBucket struct {
	requestCount       uint32
	pending            []*clientRequest
	bucketID           bucketID
	checkpointInterval uint64

	// currentCheckpoint is initially set to the base checkpoint value.  It is incremented by
	// the caller when querying for available batches, as the caller supplies the current sequence
	// number (which will increase monotonically).  If the current sequence number is beyond the
	// next checkpoint, then the nextReadyList is moved to the readyList and reinitialized.
	currentCheckpoint uint64

	// readyList is all of the requests which are valid at or before the current sequence
	readyList *list.List

	// nextReadyList is all of the requests which are valid after the next checkpoint
	// when we advance beyond that checkpoint sequence, we push this list onto the back
	// of the ready list and re-initialize this list.
	nextReadyList *list.List
}

func newProposer(baseCheckpoint uint64, checkpointInterval uint64, myConfig *pb.StateEvent_InitialParameters, clientTracker *clientTracker, buckets map[bucketID]nodeID) *proposer {
	proposalBuckets := map[bucketID]*proposalBucket{}
	for bucketID, id := range buckets {
		if id != nodeID(myConfig.Id) {
			continue
		}
		proposalBuckets[bucketID] = &proposalBucket{
			currentCheckpoint:  baseCheckpoint,
			checkpointInterval: checkpointInterval,
			bucketID:           bucketID,
			readyList:          list.New(),
			nextReadyList:      list.New(),
			requestCount:       myConfig.BatchSize,
			pending:            make([]*clientRequest, 0, 1), // TODO, might be interesting to play with not preallocating for performance reasons
		}
	}

	return &proposer{
		myConfig:        myConfig,
		totalBuckets:    len(buckets),
		proposalBuckets: proposalBuckets,
		readyIterator:   clientTracker.readyList.iterator(),
	}
}

func (p *proposer) advance() {
	for p.readyIterator.hasNext() {
		crn := p.readyIterator.next()
		if crn.committed != nil {
			// This seems like an odd check, but it's possible that
			// this request already committed in a previous view but that
			// we have not been able to garbage collect it yet.
			continue
		}

		bucketID := bucketID((crn.reqNo + crn.clientID) % uint64(p.totalBuckets))

		proposalBucket, ok := p.proposalBuckets[bucketID]
		if !ok {
			// we don't own this bucket
			continue
		}

		if len(crn.strongRequests) > 1 {
			if _, ok := crn.strongRequests[""]; !ok {
				panic("dev sanity test")
			}

			// We must have a null request here, so prefer it.
			proposalBucket.queueRequest(crn.validAfterSeqNo, crn.strongRequests[""])
		} else {
			if len(crn.strongRequests) != 1 {
				panic("dev sanity test")
			}

			// There must be exactly one strong request
			for _, clientReq := range crn.strongRequests {
				proposalBucket.queueRequest(crn.validAfterSeqNo, clientReq)
				break
			}
		}
	}
}

func (p *proposer) proposalBucket(bucketID bucketID) *proposalBucket {
	return p.proposalBuckets[bucketID]
}

func (prb *proposalBucket) queueRequest(validAfterSeqNo uint64, cr *clientRequest) {
	if prb.currentCheckpoint >= validAfterSeqNo {
		if validAfterSeqNo != prb.currentCheckpoint+prb.checkpointInterval {
			panic("dev sanity check")
		}
		prb.readyList.PushBack(cr)
	} else {
		prb.nextReadyList.PushBack(cr)
	}
}

func (prb *proposalBucket) advance(toSeqNo uint64) {
	if toSeqNo >= prb.currentCheckpoint+prb.checkpointInterval {
		prb.currentCheckpoint += prb.checkpointInterval
		prb.readyList.PushBackList(prb.nextReadyList)
		prb.nextReadyList = list.New()
	}

	for uint32(len(prb.pending)) < prb.requestCount {
		if prb.readyList.Len() == 0 {
			break
		}

		prb.pending = append(
			prb.pending,
			prb.readyList.Remove(prb.readyList.Front()).(*clientRequest),
		)
	}
}

func (prb *proposalBucket) hasOutstanding(forSeqNo uint64) bool {
	prb.advance(forSeqNo)
	return uint32(len(prb.pending)) > 0
}

func (prb *proposalBucket) hasPending(forSeqNo uint64) bool {
	prb.advance(forSeqNo)
	return len(prb.pending) > 0 && uint32(len(prb.pending)) == prb.requestCount
}

func (prb *proposalBucket) next() []*clientRequest {
	result := prb.pending
	prb.pending = make([]*clientRequest, 0, prb.requestCount)
	return result
}
