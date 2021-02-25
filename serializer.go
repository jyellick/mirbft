/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mirbft

import (
	"sync"

	"github.com/IBM/mirbft/pkg/pb/msgs"
	"github.com/IBM/mirbft/pkg/pb/state"
	"github.com/IBM/mirbft/pkg/statemachine"
	"github.com/IBM/mirbft/pkg/status"

	"github.com/pkg/errors"
)

// serializer provides a single threaded way to access the Mir state machine
// and passes work to/from the state machine.
type serializer struct {
	actionsC       chan Actions
	clientActionsC chan ClientActions
	doneC          chan struct{}
	resultsC       chan *state.EventActionResults
	clientResultsC chan *state.EventClientActionResults
	transferC      chan *state.Event_Transfer
	statusC        chan chan<- *status.StateMachine
	stepC          chan *state.Event_Step
	tickC          chan struct{}
	errC           chan struct{}

	myConfig   *Config
	walStorage WALStorage

	exitMutex  sync.Mutex
	exitErr    error
	exitStatus *status.StateMachine
}

func newSerializer(myConfig *Config, walStorage WALStorage) (*serializer, error) {

	s := &serializer{
		actionsC:       make(chan Actions),
		clientActionsC: make(chan ClientActions),
		doneC:          make(chan struct{}),
		clientResultsC: make(chan *state.EventClientActionResults),
		resultsC:       make(chan *state.EventActionResults),
		transferC:      make(chan *state.Event_Transfer),
		statusC:        make(chan chan<- *status.StateMachine),
		stepC:          make(chan *state.Event_Step),
		tickC:          make(chan struct{}),
		errC:           make(chan struct{}),
		myConfig:       myConfig,
		walStorage:     walStorage,
	}
	go s.run()
	return s, nil
}

func (s *serializer) stop() {
	s.exitMutex.Lock()
	select {
	case <-s.doneC:
	default:
		close(s.doneC)
	}
	s.exitMutex.Unlock()
	<-s.errC
}

func (s *serializer) getExitErr() error {
	s.exitMutex.Lock()
	defer s.exitMutex.Unlock()
	return s.exitErr
}

// TODO, add assertion in tests that log levels match
type logAdapter struct {
	Logger
}

func (la logAdapter) Log(level statemachine.LogLevel, msg string, args ...interface{}) {
	la.Logger.Log(LogLevel(level), msg, args...)
}

// run must be single threaded and is therefore hidden to prevent accidental capture
// of other go routines.
func (s *serializer) run() (exitErr error) {
	sm := &statemachine.StateMachine{
		Logger: logAdapter{Logger: s.myConfig.Logger},
	}

	defer func() {
		s.exitMutex.Lock()
		defer s.exitMutex.Unlock()
		close(s.errC)
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				s.exitErr = errors.Wrapf(err, "serializer caught panic")
			} else {
				s.exitErr = errors.Errorf("panic in statemachine: %v", r)
			}
		} else {
			s.exitErr = exitErr
		}
		s.exitStatus = sm.Status()
	}()

	actions := &Actions{}
	clientActions := &ClientActions{}

	applyEvent := func(stateEvent *state.Event) error {
		if s.myConfig.EventInterceptor != nil {
			err := s.myConfig.EventInterceptor.Intercept(stateEvent)
			if err != nil {
				return errors.WithMessage(err, "event interceptor error")
			}
		}

		newActions, newClientActions := toActions(sm.ApplyEvent(stateEvent))

		actions.concat(newActions)
		clientActions.concat(newClientActions)
		return nil
	}

	err := applyEvent(&state.Event{
		Type: &state.Event_Initialize{
			Initialize: &state.EventInitialParameters{
				Id:                   s.myConfig.ID,
				BatchSize:            s.myConfig.BatchSize,
				HeartbeatTicks:       s.myConfig.HeartbeatTicks,
				SuspectTicks:         s.myConfig.SuspectTicks,
				NewEpochTimeoutTicks: s.myConfig.NewEpochTimeoutTicks,
				BufferSize:           s.myConfig.BufferSize,
			},
		},
	})
	if err != nil {
		return err
	}

	err = s.walStorage.LoadAll(func(i uint64, p *msgs.Persistent) {
		if _, ok := s.walStorage.(*dummyWAL); ok {
			// This was our own startup/bootstrap WAL,
			// we need to get these entries persisted into the real one.
			actions.persist(i, p)
		}

		applyEvent(&state.Event{
			Type: &state.Event_LoadEntry{
				LoadEntry: &state.EventPersistedEntry{
					Index: i,
					Data:  p,
				},
			},
		})
	})

	if err != nil {
		return errors.WithMessage(err, "failed to load persisted from WALStorage")
	}

	err = applyEvent(&state.Event{
		Type: &state.Event_CompleteInitialization{
			CompleteInitialization: &state.EventLoadCompleted{},
		},
	})
	if err != nil {
		return err
	}

	var actionsC chan<- Actions
	var clientActionsC chan<- ClientActions
	for {
		var err error
		select {
		case step := <-s.stepC:
			err = applyEvent(&state.Event{
				Type: step,
			})
		case actionsC <- *actions:
			actions.clear()
			actionsC = nil
			err = applyEvent(&state.Event{
				Type: &state.Event_ActionsReceived{
					ActionsReceived: &state.EventReady{},
				},
			})
		case clientActionsC <- *clientActions:
			clientActions.clear()
			clientActionsC = nil
			err = applyEvent(&state.Event{
				Type: &state.Event_ClientActionsReceived{
					ClientActionsReceived: &state.EventReady{},
				},
			})
		case transfer := <-s.transferC:
			err = applyEvent(&state.Event{
				Type: transfer,
			})
		case results := <-s.resultsC:
			err = applyEvent(&state.Event{
				Type: &state.Event_AddResults{
					AddResults: results,
				},
			})
		case clientResults := <-s.clientResultsC:
			err = applyEvent(&state.Event{
				Type: &state.Event_AddClientResults{
					AddClientResults: clientResults,
				},
			})
		case statusReq := <-s.statusC:
			select {
			case statusReq <- sm.Status():
			case <-s.doneC:
			}
		case <-s.tickC:
			err = applyEvent(&state.Event{
				Type: &state.Event_Tick{
					Tick: &state.EventTickElapsed{},
				},
			})
		case <-s.doneC:
			return ErrStopped
		}

		if !actions.isEmpty() {
			actionsC = s.actionsC
		}

		if !clientActions.isEmpty() {
			clientActionsC = s.clientActionsC
		}

		if err != nil {
			return err
		}
	}
}
