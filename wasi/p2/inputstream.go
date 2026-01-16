package p2

import (
	"io"
	"math"
	"sync"
)

type InputStream interface {
	Read(n uint64) ([]byte, error)
	BlockingRead(n uint64) ([]byte, error)
	Skip(n uint64) (uint64, error)
	BlockingSkip(n uint64) (uint64, error)
	Subscribe(func())
}

type buffer interface {
	read(n int) ([]byte, bool, error)
	getWriteBuffer() *[]byte
}

type bytesBuffer struct {
	data   []byte
	offset int
}

func (b *bytesBuffer) read(n int) ([]byte, bool, error) {
	remaining := len(b.data) - b.offset
	if remaining == 0 {
		return nil, false, nil
	}

	toRead := n
	if remaining < n {
		toRead = remaining
	}

	start := b.offset
	b.offset += toRead
	remaining -= toRead
	return b.data[start : start+toRead], remaining > 0, nil
}

func (b *bytesBuffer) getWriteBuffer() *[]byte {
	b.offset = 0
	return &b.data
}

type errorBuffer struct {
	err error
}

func (b *errorBuffer) read(n int) ([]byte, bool, error) {
	return nil, false, b.err
}

func (b *errorBuffer) getWriteBuffer() *[]byte {
	return nil
}

type bufferPool struct {
	written    chan buffer
	free       chan buffer
	bufferSize int
}

func newBufferPool(numBuffers, bufferSize int) *bufferPool {
	free := make(chan buffer, numBuffers)
	for range numBuffers {
		free <- &bytesBuffer{data: make([]byte, bufferSize)}
	}
	return &bufferPool{
		written:    make(chan buffer, numBuffers),
		free:       free,
		bufferSize: bufferSize,
	}
}

type ReaderInputStream struct {
	r               io.Reader
	err             error
	bufferPool      *bufferPool
	currentBuffer   buffer
	maxReadSize     int
	done            chan struct{}
	subscriptionsMu sync.Mutex
	subscriptions   []func()
}

func NewReaderInputStream(r io.Reader, maxReadSize, bufferSize, nBuffers int) *ReaderInputStream {
	pool := newBufferPool(nBuffers, bufferSize)
	stream := &ReaderInputStream{
		r:           r,
		bufferPool:  pool,
		maxReadSize: maxReadSize,
		done:        make(chan struct{}),
	}
	go func() {
		defer close(pool.written)
		for {
			select {
			case <-stream.done:
				return
			case buf := <-pool.free:
				slcp := buf.getWriteBuffer()
				if slcp == nil {
					continue
				}
				*slcp = (*slcp)[0:bufferSize:bufferSize]
				n, err := r.Read(*slcp)
				if err != nil {
					select {
					case <-stream.done:
					case pool.written <- &errorBuffer{err: err}:
						stream.notifySubscriptions()
					}
					return
				}
				*slcp = (*slcp)[0:n]
				select {
				case <-stream.done:
					return
				case pool.written <- buf:
					stream.notifySubscriptions()
				}
			}
		}
	}()
	return stream
}

func (s *ReaderInputStream) Read(n uint64) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	toRead := n
	if n > uint64(s.maxReadSize) {
		toRead = uint64(s.maxReadSize)
	}

	outBuf := make([]byte, 0, toRead)
	remaining := toRead

	for remaining > 0 {
		if s.currentBuffer == nil {
			select {
			case b := <-s.bufferPool.written:
				s.currentBuffer = b
			default:
				return outBuf, nil
			}
		}
		data, hasMore, err := s.currentBuffer.read(int(remaining))
		if err != nil {
			s.closeOnErr(err)
			if len(outBuf) == 0 {
				return nil, err
			}
			return outBuf, nil
		}

		outBuf = append(outBuf, data...)
		remaining -= uint64(len(data))
		if !hasMore {
			s.bufferPool.free <- s.currentBuffer
			s.currentBuffer = nil
		}
		if remaining == 0 {
			break
		}
	}
	return outBuf, nil
}

func (s *ReaderInputStream) BlockingRead(n uint64) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	if s.currentBuffer == nil {
		s.currentBuffer = <-s.bufferPool.written
	}

	return s.Read(n)
}

func (s *ReaderInputStream) Skip(n uint64) (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}

	remaining := n
	for {
		if s.currentBuffer == nil {
			select {
			case b := <-s.bufferPool.written:
				s.currentBuffer = b
			default:
				return n - remaining, nil
			}
		}

		toRead := int(remaining)
		if remaining > math.MaxInt {
			toRead = math.MaxInt
		}

		b, hasMore, err := s.currentBuffer.read(toRead)
		if err != nil {
			s.closeOnErr(err)
			return n - remaining, err
		}
		remaining -= uint64(len(b))

		if !hasMore {
			s.bufferPool.free <- s.currentBuffer
			s.currentBuffer = nil
		}

		if remaining == 0 {
			return n, nil
		}
	}
}

func (s *ReaderInputStream) BlockingSkip(n uint64) (uint64, error) {
	if s.currentBuffer == nil {
		s.currentBuffer = <-s.bufferPool.written
	}

	return s.Skip(n)
}

func (s *ReaderInputStream) Subscribe(fn func()) {
	if s.currentBuffer != nil {
		fn()
		return
	}

	select {
	case b := <-s.bufferPool.written:
		s.currentBuffer = b
		fn()
	default:
		s.subscriptionsMu.Lock()
		defer s.subscriptionsMu.Unlock()
		s.subscriptions = append(s.subscriptions, fn)
	}
}

func (s *ReaderInputStream) Close() error {
	s.closeOnErr(io.EOF)
	if closer, ok := s.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s *ReaderInputStream) closeOnErr(err error) {
	if s.err != nil {
		return
	}
	s.err = err
	close(s.done)
	for _, fn := range s.subscriptions {
		fn()
	}
	close(s.bufferPool.free)
}

func (s *ReaderInputStream) notifySubscriptions() {
	s.subscriptionsMu.Lock()
	notifySubs := s.subscriptions
	s.subscriptions = nil
	s.subscriptionsMu.Unlock()

	for _, sub := range notifySubs {
		sub()
	}
}
