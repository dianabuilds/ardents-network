package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	networkprivacy "ardents/internal/network/privacy"
)

const privateExchangeQueueSize = 32

type PrivateExchange struct {
	mu              sync.Mutex
	channel         *networkprivacy.Channel
	carrier         networkprivacy.LiveCarrier
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
		return networkprivacy.CapabilityUnavailable()
	}
	topic, err := e.channel.ContentTopic()
	if err != nil {
		return err
	}
	envelopes, err := e.carrier.SubscribePrivateEnvelopes(ctx, topic)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.generation++
	generation := e.generation
	e.mu.Unlock()
	go e.receive(ctx, generation, envelopes)
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
		return networkprivacy.CapabilityUnavailable()
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
