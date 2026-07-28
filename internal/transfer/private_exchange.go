package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	networkprivacy "ardents/internal/messaging"
)

const privateExchangeQueueSize = 32

type PrivateExchange struct {
	refreshMu       sync.Mutex
	mu              sync.Mutex
	channel         *networkprivacy.Channel
	carrier         networkprivacy.LiveCarrier
	runContext      context.Context
	runCancel       context.CancelFunc
	receiveCancel   context.CancelFunc
	generation      uint64
	waiters         map[string]chan []byte
	replicaWaiters  map[string]chan ReplicaControlMessage
	requests        chan []byte
	replicaRequests chan ReplicaControlMessage
	failures        chan error
}

func NewPrivateExchange(channel *networkprivacy.Channel, carrier networkprivacy.LiveCarrier) *PrivateExchange {
	return &PrivateExchange{
		channel: channel, carrier: carrier, waiters: make(map[string]chan []byte),
		replicaWaiters:  make(map[string]chan ReplicaControlMessage),
		requests:        make(chan []byte, privateExchangeQueueSize),
		replicaRequests: make(chan ReplicaControlMessage, privateExchangeQueueSize),
		failures:        make(chan error, privateExchangeQueueSize),
	}
}

func (e *PrivateExchange) Start(ctx context.Context) error {
	if e == nil || e.channel == nil || e.carrier == nil {
		return networkprivacy.ChannelGrantUnavailable()
	}
	e.mu.Lock()
	if e.runContext != nil {
		e.mu.Unlock()
		return fmt.Errorf("private exchange is already started")
	}
	e.runContext, e.runCancel = context.WithCancel(ctx)
	e.mu.Unlock()
	if err := e.RefreshPrivateSubscriptions(ctx); err != nil {
		e.mu.Lock()
		e.runCancel()
		e.runContext, e.runCancel = nil, nil
		e.mu.Unlock()
		return err
	}
	return nil
}

// RefreshPrivateSubscriptions atomically adopts the channel's current and
// receive-only previous topics. Existing subscriptions remain live if any new
// subscription cannot be established.
func (e *PrivateExchange) RefreshPrivateSubscriptions(_ context.Context) error {
	if e == nil || e.carrier == nil {
		return networkprivacy.ChannelGrantUnavailable()
	}
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()
	e.mu.Lock()
	runContext := e.runContext
	e.mu.Unlock()
	if runContext == nil {
		return nil
	}
	if e.channel == nil {
		return networkprivacy.ChannelGrantUnavailable()
	}
	if err := runContext.Err(); err != nil {
		return err
	}
	topics, err := e.channel.ContentTopics()
	if err != nil {
		return err
	}
	receiveContext, cancel := context.WithCancel(runContext)
	subscriptions := make([]<-chan networkprivacy.SealedEnvelope, 0, len(topics))
	for _, topic := range topics {
		envelopes, subscribeErr := e.carrier.SubscribePrivateEnvelopes(receiveContext, topic)
		if subscribeErr != nil {
			cancel()
			return subscribeErr
		}
		subscriptions = append(subscriptions, envelopes)
	}
	e.mu.Lock()
	previousCancel := e.receiveCancel
	e.receiveCancel = cancel
	e.generation++
	generation := e.generation
	e.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	for _, envelopes := range subscriptions {
		go e.receive(receiveContext, generation, envelopes)
	}
	return nil
}

func (e *PrivateExchange) Requests() <-chan []byte                       { return e.requests }
func (e *PrivateExchange) Failures() <-chan error                        { return e.failures }
func (e *PrivateExchange) ReplicaRequests() <-chan ReplicaControlMessage { return e.replicaRequests }

type ReplicaControlMessage struct {
	OperationID string
	Action      string
	Sender      string
	Payload     []byte
}

func (e *PrivateExchange) RegisterReplicaResponses(operationID string) (<-chan ReplicaControlMessage, func(), error) {
	if e == nil || operationID == "" {
		return nil, nil, fmt.Errorf("replica response registration is incomplete")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.replicaWaiters[operationID]; exists {
		return nil, nil, fmt.Errorf("replica response operation is already registered")
	}
	responses := make(chan ReplicaControlMessage, 4)
	e.replicaWaiters[operationID] = responses
	return responses, func() { e.unregisterReplica(operationID, responses) }, nil
}

func (e *PrivateExchange) RegisterResponse(requestID string) (<-chan []byte, func(), error) {
	if e == nil || requestID == "" {
		return nil, nil, fmt.Errorf("private response registration is incomplete")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.waiters[requestID]; exists {
		return nil, nil, fmt.Errorf("private response request is already registered")
	}
	responses := make(chan []byte, 4)
	e.waiters[requestID] = responses
	return responses, func() { e.unregister(requestID, responses) }, nil
}

func (e *PrivateExchange) Publish(ctx context.Context, class networkprivacy.MessageClass, payload []byte) error {
	if e == nil || e.channel == nil || e.carrier == nil {
		return networkprivacy.ChannelGrantUnavailable()
	}
	envelope, err := e.channel.Seal(class, 1, payload)
	if err != nil {
		return err
	}
	return e.carrier.PublishPrivateEnvelope(ctx, envelope)
}

func (e *PrivateExchange) receive(ctx context.Context, generation uint64, envelopes <-chan networkprivacy.SealedEnvelope) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-envelopes:
			if !ok {
				return
			}
			if !e.current(generation) {
				return
			}
			e.receiveEnvelope(ctx, envelope)
		}
	}
}

func (e *PrivateExchange) receiveEnvelope(ctx context.Context, envelope networkprivacy.SealedEnvelope) {
	opened, err := e.channel.Open(envelope)
	if err != nil {
		e.reportFailure(err)
		return
	}
	switch opened.Class {
	case networkprivacy.MessageClassBlobFetchRequest:
		e.deliver(ctx, e.requests, opened.Payload)
	case networkprivacy.MessageClassBlobFetchResponse:
		e.dispatchResponse(ctx, opened.Payload)
	case networkprivacy.MessageClassBlobReplicaControl:
		e.dispatchReplicaControl(ctx, opened)
	}
}

func (e *PrivateExchange) dispatchReplicaControl(ctx context.Context, opened networkprivacy.OpenedMessage) {
	var route struct {
		OperationID string `json:"operation_id"`
		Action      string `json:"action"`
	}
	if err := json.Unmarshal(opened.Payload, &route); err != nil || route.OperationID == "" || route.Action == "" {
		e.reportFailure(fmt.Errorf("private replica control routing metadata is invalid"))
		return
	}
	message := ReplicaControlMessage{OperationID: route.OperationID, Action: route.Action, Sender: opened.Sender, Payload: append([]byte(nil), opened.Payload...)}
	if isReplicaResponse(route.Action) {
		e.mu.Lock()
		responses := e.replicaWaiters[route.OperationID]
		e.mu.Unlock()
		if responses != nil {
			e.deliverReplica(ctx, responses, message)
		}
		return
	}
	e.deliverReplica(ctx, e.replicaRequests, message)
}

func isReplicaResponse(action string) bool {
	return action == "reserve_result" || action == "commit_result" || action == "capacity_result" || action == "health_result"
}

func (e *PrivateExchange) deliverReplica(ctx context.Context, target chan ReplicaControlMessage, message ReplicaControlMessage) {
	select {
	case <-ctx.Done():
	case target <- message:
	}
}

func (e *PrivateExchange) dispatchResponse(ctx context.Context, payload []byte) {
	var route struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(payload, &route); err != nil || route.RequestID == "" {
		e.reportFailure(fmt.Errorf("private blob response routing metadata is invalid"))
		return
	}
	e.mu.Lock()
	responses := e.waiters[route.RequestID]
	e.mu.Unlock()
	if responses != nil {
		e.deliver(ctx, responses, payload)
	}
}

func (e *PrivateExchange) deliver(ctx context.Context, target chan []byte, payload []byte) {
	select {
	case <-ctx.Done():
	case target <- append([]byte(nil), payload...):
	}
}

func (e *PrivateExchange) reportFailure(err error) {
	select {
	case e.failures <- err:
	default:
	}
}

func (e *PrivateExchange) unregister(requestID string, responses chan []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.waiters[requestID] == responses {
		delete(e.waiters, requestID)
	}
}

func (e *PrivateExchange) unregisterReplica(operationID string, responses chan ReplicaControlMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.replicaWaiters[operationID] == responses {
		delete(e.replicaWaiters, operationID)
	}
}

func (e *PrivateExchange) current(generation uint64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.generation == generation
}
