package partypubsub

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// BufSize is the number of incoming messages to buffer for each topic.
const BufSize = 128

// SignersParty represents a subsspiption to a single PubSub topic. Messages
// can be published to the topic with SignersParty.Publish, and received
// messages are pushed to the Messages channel.
type SignersParty struct {
	// Messages is a channel of messages received from other peers in the chat room
	Messages chan *PartyMessage

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	self peer.ID
	name string
}

// PartyMessage gets converted to/from JSON and sent in the body of pubsub messages.
// @TODO: this should be a proto from TSS-lib
type PartyMessage struct {
	Message  string
	SenderID string
}

// JoinSignersParty tries to subscribe to the PubSub topic, returning
// a SignersParty on success.
func JoinSignersPartyPS(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, name string) (*SignersParty, error) {
	// join the pubsub topic
	topic, err := ps.Join(topicName(name))
	if err != nil {
		return nil, err
	}

	// and subsspibe to it
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	sp := &SignersParty{
		ctx:      ctx,
		ps:       ps,
		topic:    topic,
		sub:      sub,
		self:     selfID,
		name:     name,
		Messages: make(chan *PartyMessage, BufSize),
	}

	// start reading messages from the subsspiption in a loop
	go sp.readLoop()
	return sp, nil
}

// Publish sends a message to the pubsub topic.
// @TODO: this should be a protobuf encoded message courtesy of tss-lib
func (sp *SignersParty) Publish(message string) error {
	m := PartyMessage{
		Message:  message,
		SenderID: sp.self.String(),
	}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return sp.topic.Publish(sp.ctx, msgBytes)
}

func (sp *SignersParty) ListPeers() []peer.ID {
	return sp.ps.ListPeers(topicName(sp.name))
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (sp *SignersParty) readLoop() {
	for {
		msg, err := sp.sub.Next(sp.ctx)
		if err != nil {
			close(sp.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == sp.self {
			continue
		}
		m := new(PartyMessage)
		err = json.Unmarshal(msg.Data, m)
		if err != nil {
			continue
		}
		// send valid messages onto the Messages channel
		sp.Messages <- m
	}
}

func topicName(name string) string {
	return "tss-party:" + name
}
