package svnode

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-zeromq/zmq4"
)

type ZMQClient struct {
	blockURL string
	txURL    string
	logger   *slog.Logger

	blocks chan BlockNotification
	txs    chan TxNotification

	mu       sync.Mutex
	blockSub zmq4.Socket
	txSub    zmq4.Socket
	closed   bool
}

type BlockNotification struct {
	Hash     [32]byte
	Header   []byte
	Sequence uint32
}

type TxNotification struct {
	TxID     [32]byte
	RawTx    []byte
	Sequence uint32
}

func NewZMQClient(blockURL, txURL string, logger *slog.Logger) (*ZMQClient, error) {
	if blockURL == "" && txURL == "" {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ZMQClient{
		blockURL: blockURL,
		txURL:    txURL,
		logger:   logger,
		blocks:   make(chan BlockNotification, 64),
		txs:      make(chan TxNotification, 256),
	}, nil
}

func (z *ZMQClient) Connect(ctx context.Context) error {
	if z.blockURL != "" {
		s := zmq4.NewSub(ctx)
		if err := s.Dial(z.blockURL); err != nil {
			return fmt.Errorf("svnode zmq dial blocks: %w", err)
		}
		if err := s.SetOption(zmq4.OptionSubscribe, "hashblock"); err != nil {
			return fmt.Errorf("svnode zmq subscribe hashblock: %w", err)
		}
		z.mu.Lock()
		z.blockSub = s
		z.mu.Unlock()
		go z.pumpBlocks()
	}
	if z.txURL != "" {
		s := zmq4.NewSub(ctx)
		if err := s.Dial(z.txURL); err != nil {
			return fmt.Errorf("svnode zmq dial tx: %w", err)
		}
		if err := s.SetOption(zmq4.OptionSubscribe, "rawtx"); err != nil {
			return fmt.Errorf("svnode zmq subscribe rawtx: %w", err)
		}
		z.mu.Lock()
		z.txSub = s
		z.mu.Unlock()
		go z.pumpTxs()
	}
	return nil
}

func (z *ZMQClient) Close() error {
	z.mu.Lock()
	z.closed = true
	if z.blockSub != nil {
		_ = z.blockSub.Close()
	}
	if z.txSub != nil {
		_ = z.txSub.Close()
	}
	z.mu.Unlock()
	return nil
}

func (z *ZMQClient) Blocks() <-chan BlockNotification { return z.blocks }
func (z *ZMQClient) Txs() <-chan TxNotification       { return z.txs }

func (z *ZMQClient) pumpBlocks() {
	for {
		msg, err := z.blockSub.Recv()
		z.mu.Lock()
		closed := z.closed
		z.mu.Unlock()
		if closed {
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			z.logger.Warn("zmq recv block", "err", err)
			return
		}
		if len(msg.Frames) < 3 {
			continue
		}
		var b BlockNotification
		copy(b.Hash[:], msg.Frames[1])
		b.Sequence = binary.LittleEndian.Uint32(msg.Frames[2])
		select {
		case z.blocks <- b:
		default:
			z.logger.Warn("zmq blocks channel full; dropping")
		}
	}
}

func (z *ZMQClient) pumpTxs() {
	for {
		msg, err := z.txSub.Recv()
		z.mu.Lock()
		closed := z.closed
		z.mu.Unlock()
		if closed {
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			z.logger.Warn("zmq recv tx", "err", err)
			return
		}
		if len(msg.Frames) < 3 {
			continue
		}
		var t TxNotification
		t.RawTx = append([]byte(nil), msg.Frames[1]...)
		t.Sequence = binary.LittleEndian.Uint32(msg.Frames[2])
		select {
		case z.txs <- t:
		default:
			z.logger.Warn("zmq txs channel full; dropping")
		}
	}
}
