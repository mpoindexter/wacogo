package p2

import (
	"fmt"
	"io"
	"sync"
)

type OutputStream interface {
	CheckWrite() (uint64, error)
	Write(contents []byte) error
	BlockingWriteAndFlush(contents []byte) error
	Flush() error
	BlockingFlush() error
	Subscribe(func())
	WriteZeroes(n uint64) error
	BlockingWriteZeroesAndFlush(n uint64) error
	Splice(src InputStream, n uint64) (uint64, error)
	BlockingSplice(src InputStream, n uint64) (uint64, error)
}

const maxWriteSize = 4 * 1024 // 4KB

type WriterOutputStream struct {
	w               io.Writer
	bufferPool      *bufferPool
	err             error
	allocatedBuffer buffer
	subscriptionsMu sync.Mutex
	subscriptions   []func()
	done            chan struct{}
}

func NewWriterOutputStream(w io.Writer) *WriterOutputStream {
	stream := &WriterOutputStream{
		w:          w,
		bufferPool: newBufferPool(1, maxWriteSize),
		done:       make(chan struct{}),
	}

	go func() {
		defer close(stream.bufferPool.free)
		for {
			select {
			case <-stream.done:
				return
			case buf, ok := <-stream.bufferPool.written:
				if !ok {
					return
				}
				b, _, _ := buf.read(maxWriteSize)
				_, err := w.Write(b)
				if err != nil {
					select {
					case <-stream.done:
					case stream.bufferPool.free <- &errorBuffer{err: err}:
						stream.notifySubscribers()
					}
					return
				}
				select {
				case <-stream.done:
					return
				case stream.bufferPool.free <- buf:
					stream.notifySubscribers()
				}
			}
		}
	}()

	return stream
}

func (s *WriterOutputStream) CheckWrite() (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}

	b, ok, err := s.getWriteBuffer(false)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}

	return uint64(cap(*b)), nil
}

func (s *WriterOutputStream) Write(contents []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.allocatedBuffer == nil {
		panic("no allocated buffer")
	}

	if len(contents) > cap(*s.allocatedBuffer.getWriteBuffer()) {
		return fmt.Errorf("write exceeds allocated buffer size")
	}

	b := s.allocatedBuffer.getWriteBuffer()
	*b = (*b)[:len(contents)]
	copy(*b, contents)
	s.bufferPool.written <- s.allocatedBuffer
	s.allocatedBuffer = nil
	return nil
}

func (s *WriterOutputStream) BlockingWriteAndFlush(contents []byte) error {
	if len(contents) > maxWriteSize {
		return fmt.Errorf("write exceeds max size")
	}

	b, _, err := s.getWriteBuffer(true)
	if err != nil {
		return err
	}

	*b = (*b)[:len(contents)]
	copy(*b, contents)
	s.bufferPool.written <- s.allocatedBuffer
	s.allocatedBuffer = nil
	_, _, err = s.getWriteBuffer(true)
	return err
}

func (s *WriterOutputStream) Flush() error {
	// Non-blocking flush is a no-op since writes are handled asynchronously
	return s.err
}

func (s *WriterOutputStream) BlockingFlush() error {
	if s.err != nil {
		return s.err
	}

	_, _, err := s.getWriteBuffer(true)
	return err
}

func (s *WriterOutputStream) Subscribe(fn func()) {
	if s.err != nil {
		fn()
		return
	}

	_, ok, _ := s.getWriteBuffer(false)
	if ok {
		fn()
		return
	}

	s.subscriptionsMu.Lock()
	s.subscriptions = append(s.subscriptions, fn)
	s.subscriptionsMu.Unlock()
}

func (s *WriterOutputStream) WriteZeroes(n uint64) error {
	if n > maxWriteSize {
		return fmt.Errorf("write exceeds max size")
	}

	bytes := make([]byte, n)
	return s.Write(bytes)
}

func (s *WriterOutputStream) BlockingWriteZeroesAndFlush(n uint64) error {
	if n > maxWriteSize {
		return fmt.Errorf("write exceeds max size")
	}

	bytes := make([]byte, n)
	return s.BlockingWriteAndFlush(bytes)
}

func (s *WriterOutputStream) Splice(src InputStream, n uint64) (uint64, error) {
	writable, err := s.CheckWrite()
	if err != nil {
		return 0, err
	}
	if writable < n {
		n = writable
	}

	data, err := src.Read(n)
	if err != nil {
		return 0, err
	}

	err = s.Write(data)
	if err != nil {
		return 0, err
	}

	return uint64(len(data)), nil
}

func (s *WriterOutputStream) BlockingSplice(src InputStream, n uint64) (uint64, error) {
	if n > maxWriteSize {
		n = maxWriteSize
	}

	data, err := src.BlockingRead(n)
	if err != nil {
		return 0, err
	}

	err = s.BlockingWriteAndFlush(data)
	if err != nil {
		return 0, err
	}

	return uint64(len(data)), nil
}

func (s *WriterOutputStream) Close() error {
	s.closeOnErr(io.EOF)
	if closer, ok := s.w.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s *WriterOutputStream) notifySubscribers() {
	s.subscriptionsMu.Lock()
	subs := s.subscriptions
	s.subscriptions = nil
	s.subscriptionsMu.Unlock()

	for _, fn := range subs {
		fn()
	}
}

func (s *WriterOutputStream) getWriteBuffer(blocking bool) (*[]byte, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}

	if s.allocatedBuffer == nil {
		if blocking {
			s.allocatedBuffer = <-s.bufferPool.free
		} else {
			select {
			case buf := <-s.bufferPool.free:
				s.allocatedBuffer = buf
			default:
				return nil, false, nil
			}
		}
	}

	b := s.allocatedBuffer.getWriteBuffer()
	if b == nil {
		_, _, err := s.allocatedBuffer.read(0)
		if err != nil {
			s.closeOnErr(err)
			return nil, false, s.err
		}
	}
	*b = (*b)[:cap(*b):cap(*b)]
	return b, true, nil
}

func (s *WriterOutputStream) closeOnErr(err error) {
	if s.err != nil {
		return
	}
	s.err = err
	s.notifySubscribers()
	close(s.done)
	close(s.bufferPool.written)
}
