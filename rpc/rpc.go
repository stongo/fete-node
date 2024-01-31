package rpc

import (
	jsonrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stongo/fete-node/common"
)

type MessageService struct{}

/* Threshold message signing
 * This is only a stub
 */
func (s *MessageService) Sign(m string) bool {
	common.Logger.Info("received signing request")
	return false
}

func (s *MessageService) Ping() string {
	return "pong"
}

/* Create and return a new jsonrpc server
 * Registers a message service
 */
func NewServer() (*jsonrpc.Server, error) {
	s := jsonrpc.NewServer()
	message := new(MessageService)
	err := s.RegisterName("message", message)
	if err != nil {
		return nil, err
	}
	return s, nil
}
