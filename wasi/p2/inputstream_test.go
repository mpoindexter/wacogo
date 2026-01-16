package p2

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/synctest"
	"time"
)

type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// blockingReader blocks on Read until unblocked
type blockingReader struct {
	unblock chan struct{}
	data    []byte
	offset  int
}

func newBlockingReader(data []byte) *blockingReader {
	return &blockingReader{
		unblock: make(chan struct{}),
		data:    data,
	}
}

func (b *blockingReader) Read(p []byte) (n int, err error) {
	<-b.unblock
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *blockingReader) Unblock() {
	close(b.unblock)
}

func TestReaderInputStream_Read_Simple(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := []byte("hello world")
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		synctest.Wait()

		result, err := stream.Read(5)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		if string(result) != "hello" {
			t.Errorf("Read() = %q, want %q", result, "hello")
		}
	})
}

func TestReaderInputStream_Read_MaxReadSize(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := make([]byte, 2000)
		for i := range data {
			data[i] = byte(i % 256)
		}
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 100, 512, 2)
		defer stream.Close()

		// Request more than maxReadSize
		result, err := stream.Read(500)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		// Should only get maxReadSize bytes
		if len(result) > 100 {
			t.Errorf("Read() returned %d bytes, want at most 100", len(result))
		}
	})
}

func TestReaderInputStream_Read_NoDataAvailable(t *testing.T) {
	// Create a reader that will block
	r := newBlockingReader([]byte("data"))
	stream := NewReaderInputStream(r, 1024, 512, 2)
	defer stream.Close()

	// Try to read immediately - should return empty since no data is buffered yet
	result, err := stream.Read(10)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Read() = %d bytes, want 0 (no data available yet)", len(result))
	}
}

func TestReaderInputStream_BlockingRead(t *testing.T) {
	data := []byte("hello world")
	r := bytes.NewReader(data)
	stream := NewReaderInputStream(r, 1024, 512, 2)
	defer stream.Close()

	// BlockingRead should wait for data
	result, err := stream.BlockingRead(5)
	if err != nil {
		t.Fatalf("BlockingRead failed: %v", err)
	}

	if len(result) == 0 {
		t.Error("BlockingRead returned empty result")
	}
}

func TestReaderInputStream_Read_AfterError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expectedErr := errors.New("read error")
		r := &errorReader{err: expectedErr}
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		synctest.Wait()

		_, err := stream.Read(10)
		if err == nil {
			t.Error("Read should return error after underlying reader fails")
		}
	})
}

func TestReaderInputStream_Read_MultipleReads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := []byte("hello world from test")
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		synctest.Wait()

		// Read in chunks
		result1, err := stream.Read(5)
		if err != nil {
			t.Fatalf("First Read failed: %v", err)
		}

		synctest.Wait()

		result2, err := stream.Read(6)
		if err != nil {
			t.Fatalf("Second Read failed: %v", err)
		}

		combined := string(result1) + string(result2)
		expected := "hello world"
		if combined != expected {
			t.Errorf("Combined reads = %q, want %q", combined, expected)
		}
	})
}

func TestReaderInputStream_Skip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := []byte("0123456789abcdef")
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		synctest.Wait()

		// Skip first 10 bytes
		skipped, err := stream.Skip(10)
		if err != nil {
			t.Fatalf("Skip failed: %v", err)
		}

		if skipped != 10 {
			t.Errorf("Skip() = %d, want 10", skipped)
		}

		synctest.Wait()

		// Read remaining data
		result, err := stream.Read(6)
		if err != nil {
			t.Fatalf("Read after Skip failed: %v", err)
		}

		if string(result) != "abcdef" {
			t.Errorf("Read after Skip = %q, want %q", result, "abcdef")
		}
	})
}

func TestReaderInputStream_BlockingSkip(t *testing.T) {
	data := []byte("0123456789abcdef")
	r := bytes.NewReader(data)
	stream := NewReaderInputStream(r, 1024, 512, 2)
	defer stream.Close()

	// BlockingSkip should wait for data
	skipped, err := stream.BlockingSkip(5)
	if err != nil {
		t.Fatalf("BlockingSkip failed: %v", err)
	}

	if skipped != 5 {
		t.Errorf("BlockingSkip() = %d, want 5", skipped)
	}
}

func TestReaderInputStream_Subscribe(t *testing.T) {
	data := []byte("hello world")
	r := newBlockingReader(data)
	stream := NewReaderInputStream(r, 1024, 512, 2)
	defer stream.Close()

	notified := make(chan struct{})
	stream.Subscribe(func() {
		close(notified)
	})

	// Unblock the reader so data becomes available
	r.Unblock()

	// Wait for notification
	select {
	case <-notified:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Subscribe callback was not called")
	}
}

func TestReaderInputStream_Subscribe_DataAlreadyAvailable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := []byte("hello world")
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		// Wait for data to be buffered
		synctest.Wait()

		notified := make(chan struct{})
		stream.Subscribe(func() {
			close(notified)
		})

		// Should be notified immediately
		select {
		case <-notified:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Error("Subscribe should call callback immediately when data is available")
		}
	})
}

func TestReaderInputStream_Close(t *testing.T) {
	data := []byte("hello world")
	r := bytes.NewReader(data)
	stream := NewReaderInputStream(r, 1024, 512, 2)

	err := stream.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Subsequent reads should fail
	_, err = stream.Read(5)
	if err == nil {
		t.Error("Read after Close should return error")
	}
}

func TestReaderInputStream_Close_WithCloser(t *testing.T) {
	type readCloser struct {
		io.Reader
		closed bool
	}

	rc := &readCloser{Reader: bytes.NewReader([]byte("test"))}
	closer := func(rc *readCloser) io.ReadCloser {
		return struct {
			io.Reader
			io.Closer
		}{
			Reader: rc.Reader,
			Closer: closerFunc(func() error {
				rc.closed = true
				return nil
			}),
		}
	}(rc)

	stream := NewReaderInputStream(closer, 1024, 512, 2)
	stream.Close()

	if !rc.closed {
		t.Error("Close did not call underlying reader's Close method")
	}
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}

func TestReaderInputStream_BufferPoolExhaustion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Slow reader with small buffer pool
		data := make([]byte, 10000)
		r := bytes.NewReader(data)
		stream := NewReaderInputStream(r, 1024, 100, 1)
		defer stream.Close()

		synctest.Wait()
		// Try to read more than buffer can hold at once
		result, err := stream.Read(500)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		// Should get partial data or empty
		if len(result) > 100 {
			t.Errorf("Read returned %d bytes with buffer size 100", len(result))
		}
	})
}

func TestReaderInputStream_EmptyReader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		stream := NewReaderInputStream(r, 1024, 512, 2)
		defer stream.Close()

		synctest.Wait()

		result, err := stream.Read(10)
		if err != nil && err != io.EOF {
			t.Fatalf("Read failed: %v", err)
		}

		if len(result) != 0 {
			t.Errorf("Read from empty reader returned %d bytes, want 0", len(result))
		}
	})
}

func TestBytesBuffer_Read(t *testing.T) {
	data := []byte("hello world")
	buf := &bytesBuffer{data: data, offset: 0}

	// Read partial data
	result, hasMore, err := buf.read(5)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(result) != "hello" {
		t.Errorf("read() = %q, want %q", result, "hello")
	}

	if !hasMore {
		t.Error("hasMore should be true")
	}

	// Read remaining data
	result, hasMore, err = buf.read(100)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(result) != " world" {
		t.Errorf("read() = %q, want %q", result, " world")
	}

	if hasMore {
		t.Error("hasMore should be false")
	}

	// Read from exhausted buffer
	result, hasMore, err = buf.read(10)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("read() from exhausted buffer = %d bytes, want 0", len(result))
	}

	if hasMore {
		t.Error("hasMore should be false for exhausted buffer")
	}
}

func TestBytesBuffer_GetWriteBuffer(t *testing.T) {
	data := []byte("hello world")
	buf := &bytesBuffer{data: data, offset: 5}

	writeBuffer := buf.getWriteBuffer()
	if writeBuffer == nil {
		t.Fatal("getWriteBuffer returned nil")
	}

	if buf.offset != 0 {
		t.Errorf("offset = %d, want 0 after getWriteBuffer", buf.offset)
	}

	if &buf.data != writeBuffer {
		t.Error("getWriteBuffer should return pointer to internal data")
	}
}

func TestErrorBuffer_Read(t *testing.T) {
	expectedErr := errors.New("test error")
	buf := &errorBuffer{err: expectedErr}

	result, hasMore, err := buf.read(10)
	if err != expectedErr {
		t.Errorf("read() error = %v, want %v", err, expectedErr)
	}

	if result != nil {
		t.Errorf("read() = %v, want nil", result)
	}

	if hasMore {
		t.Error("hasMore should be false for error buffer")
	}
}

func TestErrorBuffer_GetWriteBuffer(t *testing.T) {
	buf := &errorBuffer{err: errors.New("test")}

	writeBuffer := buf.getWriteBuffer()
	if writeBuffer != nil {
		t.Errorf("getWriteBuffer() = %v, want nil", writeBuffer)
	}
}
