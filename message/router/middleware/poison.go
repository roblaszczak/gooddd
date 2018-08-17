package middleware

import (
	"github.com/pkg/errors"
	"github.com/roblaszczak/gooddd/message"
)

type poisonQueueHook func(message message.Message, err error)

var ErrInvalidPoisonQueueTopic = errors.New("invalid poison queue topic")

type PoisonQueue struct {
	topic      string
	pub        message.Publisher
	Middleware message.HandlerMiddleware
}

var ReasonForPoisonedKey = "reason_poisoned"

func NewPoisonQueue(pub message.Publisher, topic string) (PoisonQueue, error) {
	if topic == "" {
		return PoisonQueue{}, ErrInvalidPoisonQueueTopic
	}

	pq := PoisonQueue{
		topic: topic,
		pub:   pub,
	}

	pq.Middleware = func(h message.HandlerFunc) message.HandlerFunc {
		return func(msg message.ConsumedMessage) ([]message.ProducedMessage, error) {
			events, err := h(msg)
			if err != nil {
				// republish the original message on the poisoned topic
				var payload message.Payload
				msg.UnmarshalPayload(&payload)
				producedMsg := message.NewDefault(msg.UUID(), payload)

				// add context why it was poisoned
				producedMsg.SetMetadata(ReasonForPoisonedKey, err.Error())

				err := pq.pub.Publish(pq.topic, []message.ProducedMessage{producedMsg})

				// handle publishing errors
				if r := recover(); r != nil {
					var ok bool
					err, ok = r.(error)
					if !ok {
						err = errors.Errorf("recovered panic with %+v", err)
					}
				}
				if err != nil {
					return nil, err
				}
			}

			return events, nil
		}
	}
	return pq, nil
}
