/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/sample"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SampleLog is a log which simply tracks the total number of bytes committed,
// and the last 8 of those bytes in a round robin fashion.
type SampleLog struct {
	LastBytes  []byte
	TotalBytes uint64
	Position   int
}

func (sl *SampleLog) Apply(entry *pb.QEntry) {
	for _, request := range entry.Requests {
		sl.TotalBytes += uint64(len(request.Digest))
		for _, b := range request.Digest {
			sl.Position = (sl.Position + 1) % len(sl.LastBytes)
			sl.LastBytes[sl.Position] = b
		}
	}
}

func (sl *SampleLog) Snap() []byte {
	value := make([]byte, len(sl.LastBytes))
	copy(value, sl.LastBytes)
	return value
}

func (sl *SampleLog) CheckSnap(id, attestation []byte) error {
	return nil
}

type DemoEnv struct {
	DoneC     chan struct{}
	DemoNodes []*DemoNode
	Logger    *zap.Logger
	Mutex     sync.Mutex
}

type DemoNode struct {
	Log       *SampleLog
	Actions   *mirbft.Actions
	Processor *sample.SerialProcessor
	Node      *mirbft.Node
}

func NewDemoEnv() (*DemoEnv, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, errors.WithMessage(err, "could not create logger")
	}

	doneC := make(chan struct{})

	nodes := make([]*mirbft.Node, 4)
	replicas := []mirbft.Replica{{ID: 0}, {ID: 1}, {ID: 2}, {ID: 3}}
	for i := range nodes {
		config := &mirbft.Config{
			ID:     uint64(i),
			Logger: logger.Named(fmt.Sprintf("node%d", i)),
			BatchParameters: mirbft.BatchParameters{
				CutSizeBytes: 1,
			},
			SuspectTicks:         4,
			NewEpochTimeoutTicks: 8,
			HeartbeatTicks:       2,
		}

		node, err := mirbft.StartNewNode(config, doneC, replicas)
		if err != nil {
			close(doneC)
			return nil, errors.WithMessagef(err, "could not start node %d", i)
		}

		nodes[i] = node
	}

	demoNodes := make([]*DemoNode, 4)

	for i, node := range nodes {
		sampleLog := &SampleLog{
			LastBytes: make([]byte, 8),
		}

		processor := &sample.SerialProcessor{
			Node:      node,
			Validator: sample.ValidatorFunc(func(*mirbft.Request) error { return nil }),
			Hasher:    sha256.New,
			Committer: &sample.SerialCommitter{
				Log:               sampleLog,
				OutstandingSeqNos: map[uint64]*mirbft.Commit{},
			},
			Link:  sample.NewFakeLink(node.Config.ID, nodes, doneC),
			DoneC: doneC,
		}

		demoNodes[i] = &DemoNode{
			Node:      node,
			Actions:   &mirbft.Actions{},
			Log:       sampleLog,
			Processor: processor,
		}
	}

	return &DemoEnv{
		DemoNodes: demoNodes,
		DoneC:     doneC,
		Logger:    logger,
	}, nil
}

func (de *DemoEnv) GetNode(w http.ResponseWriter, r *http.Request) (int, bool) {
	vars := mux.Vars(r)
	key := vars["id"]
	var id int
	if n, err := fmt.Sscanf(key, "%d", &id); err != nil || n != 1 || id < 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Node '%s' not parseable", key)
		return 0, false
	}

	if id > len(de.DemoNodes) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Network is configured with only %d nodes\n", len(de.DemoNodes))
		return 0, false
	}
	return id, true
}

func (de *DemoEnv) HandleTick(w http.ResponseWriter, r *http.Request) {
	id, ok := de.GetNode(w, r)
	if !ok {
		return
	}

	de.Mutex.Lock()
	defer de.Mutex.Unlock()

	demoNode := de.DemoNodes[id]

	de.Logger.Info("handling tick request for node", zap.Int("node", id), zap.Int("length", demoNode.Actions.Length()))

	demoNode.Node.Tick()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
}

func (de *DemoEnv) HandleProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := de.GetNode(w, r)
	if !ok {
		return
	}

	de.Mutex.Lock()
	defer de.Mutex.Unlock()

	demoNode := de.DemoNodes[id]

	de.Logger.Info("handling process request for node", zap.Int("node", id), zap.Int("length", demoNode.Actions.Length()))

	demoNode.Node.AddResults(*demoNode.Processor.Process(demoNode.Actions))
	demoNode.Actions.Clear()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
}

func (de *DemoEnv) HandleStatus(w http.ResponseWriter, r *http.Request) {
	de.Logger.Info("handling status request")

	de.Mutex.Lock()
	defer de.Mutex.Unlock()

	nodeStatuses := []map[string]interface{}{}
	for i, demoNode := range de.DemoNodes {
		select {
		case actions := <-demoNode.Node.Ready():
			demoNode.Actions.Append(&actions)
		default:
		}

		nodeStatus := map[string]interface{}{}
		nodeStatus["log"] = demoNode.Log

		nodeStatus["actions"] = map[string]int{
			"broadcast":  len(demoNode.Actions.Broadcast),
			"unicast":    len(demoNode.Actions.Unicast),
			"preprocess": len(demoNode.Actions.Preprocess),
			"process":    len(demoNode.Actions.Process),
			"commit":     len(demoNode.Actions.Commits),
			"total":      demoNode.Actions.Length(),
		}

		status, err := demoNode.Node.Status(r.Context())
		if err != nil {
			// context canceled, or server stopped, return an error
			return
		}

		nodeStatus["stateMachine"] = status

		nodeStatus["ID"] = i

		nodeStatuses = append(nodeStatuses, nodeStatus)
	}

	bytes, err := json.Marshal(nodeStatuses)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func (de *DemoEnv) HandlePropose(w http.ResponseWriter, r *http.Request) {
	id, ok := de.GetNode(w, r)
	if !ok {
		return
	}
	de.Logger.Info("handling proposal for node", zap.Int("node", id))

	de.Mutex.Lock()
	defer de.Mutex.Unlock()

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		de.Logger.Warn("could not read body")
		return
	}

	de.Logger.Info("proposing request")
	err = de.DemoNodes[id].Node.Propose(r.Context(), reqBody)
	if err != nil {
		// context canceled, or server stopped
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (de *DemoEnv) Serve() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/node/{id}/process", de.HandleProcess)
	router.HandleFunc("/node/{id}/tick", de.HandleTick)
	router.HandleFunc("/status", de.HandleStatus)
	router.HandleFunc("/node/{id}/propose", de.HandlePropose).Methods("POST")
	de.Logger.Info("Starting HTTP server on port 10000")
	de.Logger.Fatal(http.ListenAndServe(":10000", router).Error())
}

func main() {
	de, err := NewDemoEnv()
	if err != nil {
		panic(err)
	}

	de.Serve()
}
