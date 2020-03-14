package network

import (
	"context"
	"encoding/hex"
	"net"
	"strconv"

	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/golang/protobuf/proto"
	"github.com/perlin-network/noise"
	"go.uber.org/zap"
)

type Transport struct {
	logger *zap.SugaredLogger

	id        uint64
	id2addr   map[uint64]string
	pubkey2id map[noise.PublicKey]uint64

	handler Handler
	node    *noise.Node
}

type Handler func(id uint64, data []byte)

func NewTransport(logger *zap.Logger, config *Config) (*Transport, error) {
	id2addr := make(map[uint64]string)
	pubkey2id := make(map[noise.PublicKey]uint64)
	for _, p := range config.Peers {
		pubkeyBytes, err := hex.DecodeString(p.PublicKey)
		if err != nil {
			return nil, err
		}

		var pubkey noise.PublicKey
		copy(pubkey[:], pubkeyBytes)

		id2addr[p.ID] = p.Address
		pubkey2id[pubkey] = p.ID
	}

	key, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, err
	}
	var privkey noise.PrivateKey
	copy(privkey[:], key)

	addr, port, err := net.SplitHostPort(config.ListenAddress)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(addr)

	p, err := strconv.ParseInt(port, 0, 16)
	if err != nil {
		return nil, err
	}

	id := noise.ID{
		ID:   privkey.Public(),
		Host: ip,
		Port: uint16(p),
		//Address: config.ListenAddress,
	}
	node, err := noise.NewNode(
		noise.WithNodePrivateKey(privkey),
		noise.WithNodeID(id),
		noise.WithNodeBindPort(uint16(p)),
	)
	if err != nil {
		return nil, err
	}

	return &Transport{
		logger:    logger.Sugar(),
		id:        config.ID,
		id2addr:   id2addr,
		pubkey2id: pubkey2id,
		node:      node,
	}, nil
}

func (t *Transport) Handle(h Handler) {
	t.node.Handle(func(ctx noise.HandlerContext) error {
		id, ok := t.pubkey2id[ctx.ID().ID]
		if !ok {
			t.logger.Fatal("Unknown remote: %s", ctx.ID().Address)
		}

		h(id, ctx.Data())
		return nil
	})
	t.handler = h
}

func (t *Transport) Start() error {
	t.logger.Infof("Start listening on %s...", t.node.Addr())
	return t.node.Listen()
}

func (t *Transport) Close() {
	t.logger.Infof("Closing transport")
	t.node.Close()
}

func (t *Transport) Send(dest uint64, msg *pb.Msg) {
	data, err := proto.Marshal(msg)
	if err != nil {
		panic("Failed to marshal outbound message")
	}

	// local message
	if dest == t.id {
		t.handler(dest, data)
		return
	}

	addr, ok := t.id2addr[dest]
	if !ok {
		panic("Unknown remote")
	}

	err = t.node.Send(context.TODO(), addr, data)
	if err != nil {
		t.logger.Debugf("Failed to send to %s: %s", addr, err)
	}
}
