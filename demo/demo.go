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
	"github.com/IBM/mirbft/consumer"
	"github.com/IBM/mirbft/sample"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SampleLog is a log which simply trakcs the total number of bytes committed,
// and the last 256 of those bytes in a round robin fashion.
type SampleLog struct {
	LastBytes  [256]byte
	TotalBytes uint64
	Position   int
}

func (sl *SampleLog) Apply(entry *consumer.Entry) {
	for _, data := range entry.Batch {
		sl.TotalBytes += uint64(len(data))
		for _, b := range data {
			sl.Position = (sl.Position + 1) % 256
			sl.LastBytes[sl.Position] = b
		}
	}
}

func (sl *SampleLog) Snap() ([]byte, []byte) {
	value := make([]byte, 256)
	copy(sl.LastBytes[:], value)
	return value, nil
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
	Actions   *consumer.Actions
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
		config := &consumer.Config{
			ID:     uint64(i),
			Logger: logger.Named(fmt.Sprintf("node%d", i)),
			BatchParameters: consumer.BatchParameters{
				CutSizeBytes: 1,
			},
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
		sampleLog := &SampleLog{}

		processor := &sample.SerialProcessor{
			Node:      node,
			Validator: sample.ValidatorFunc(func([]byte) error { return nil }),
			Hasher: sample.HasherFunc(func(data []byte) []byte {
				sum := sha256.Sum256(data)
				return sum[:]
			}),
			Committer: &sample.SerialCommitter{
				Log:                  sampleLog,
				OutstandingSeqBucket: map[uint64]map[uint64]*consumer.Entry{},
			},
			Link:  sample.NewFakeLink(node.Config.ID, nodes, doneC),
			DoneC: doneC,
		}

		demoNodes[i] = &DemoNode{
			Node:      node,
			Actions:   &consumer.Actions{},
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

func (de *DemoEnv) HandleProcess(w http.ResponseWriter, r *http.Request) {
	id, ok := de.GetNode(w, r)
	if !ok {
		return
	}

	de.Mutex.Lock()
	defer de.Mutex.Unlock()

	demoNode := de.DemoNodes[id]

	de.Logger.Info("handling process request for node", zap.Int("node", id), zap.Int("actions", demoNode.Actions.Length()))

	demoNode.Processor.Process(demoNode.Actions)
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
		var err error
		nodeStatus["log"], err = json.Marshal(demoNode.Log)
		if err != nil {
			panic(err)
		}

		nodeStatus["actions"] = map[string]int{
			"broadcast":  len(demoNode.Actions.Broadcast),
			"unicast":    len(demoNode.Actions.Unicast),
			"preprocess": len(demoNode.Actions.Preprocess),
			"digest":     len(demoNode.Actions.Digest),
			"validate":   len(demoNode.Actions.Validate),
			"commit":     len(demoNode.Actions.Commit),
			"checkpoint": len(demoNode.Actions.Checkpoint),
			"total":      demoNode.Actions.Length(),
		}

		status, err := demoNode.Node.Status(r.Context(), mirbft.JSONEncoding)
		if err != nil {
			// context canceled, or server stopped, return an error
			return
		}
		statusAsMap := map[string]interface{}{}
		err = json.Unmarshal([]byte(status), &statusAsMap)

		nodeStatus["stateMachine"] = statusAsMap

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
