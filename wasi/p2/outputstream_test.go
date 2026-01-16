package p2

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// mockWriter captures writes and can simulate errors
type mockWriter struct {
	buf       bytes.Buffer
	writeErr  error
	writeChan chan []byte // for synchronization in tests
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writeChan: make(chan []byte, 10),
	}
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	n, err = m.buf.Write(p)
	select {
	case m.writeChan <- append([]byte(nil), p...):
	default:
	}
	return n, err
}

func (m *mockWriter) String() string {
	return m.buf.String()
}

// closableWriter wraps a writer with Close method
type closableWriter struct {
	*mockWriter
	closed atomic.Bool
}

func (c *closableWriter) Close() error {
	c.closed.Store(true)
	return nil
}

// errorWriter always returns an error on write
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, e.err
}

func TestWriterOutputStream_CheckWrite(t *testing.T) {
	t.Run("returns buffer size when available", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		size, err := stream.CheckWrite()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != maxWriteSize {
			t.Errorf("expected size %d, got %d", maxWriteSize, size)
		}
	})

	t.Run("returns same buffer when already allocated", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		// Allocate buffer
		size1, err := stream.CheckWrite()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Second call should return the same buffer size (buffer is still allocated)
		size2, err := stream.CheckWrite()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size1 != size2 {
			t.Errorf("expected same size %d, got %d", size1, size2)
		}
		if size1 != maxWriteSize {
			t.Errorf("expected size %d, got %d", maxWriteSize, size1)
		}
	})

	t.Run("returns error after stream error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testErr := errors.New("test error")
			w := &errorWriter{err: testErr}

			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// Trigger write that will fail
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data := []byte("test")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Wait for background goroutine to process error
			synctest.Wait()

			// Now CheckWrite should return error
			_, err = stream.CheckWrite()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, testErr) {
				t.Errorf("expected error %v, got %v", testErr, err)
			}
		})
	})
}

func TestWriterOutputStream_Write(t *testing.T) {
	t.Run("writes data successfully", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// Allocate buffer
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			// Write data
			data := []byte("hello world")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Wait for background write
			synctest.Wait()

			if got := w.String(); got != "hello world" {
				t.Errorf("expected 'hello world', got %q", got)
			}
		})
	})

	t.Run("panics when no buffer allocated", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()

		stream.Write([]byte("test"))
	})

	t.Run("returns error when data exceeds buffer size", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		_, err := stream.CheckWrite()
		if err != nil {
			t.Fatalf("CheckWrite failed: %v", err)
		}

		// Try to write more than maxWriteSize
		data := make([]byte, maxWriteSize+1)
		err = stream.Write(data)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error after stream error", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		stream.closeOnErr(io.ErrUnexpectedEOF)

		_, err := stream.CheckWrite()
		if err != io.ErrUnexpectedEOF {
			t.Errorf("expected ErrUnexpectedEOF, got %v", err)
		}

		// Write should also return error without allocating buffer
		err = stream.Write([]byte("test"))
		if err != io.ErrUnexpectedEOF {
			t.Errorf("expected ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("multiple sequential writes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			for i := 0; i < 5; i++ {
				_, err := stream.CheckWrite()
				if err != nil {
					t.Fatalf("CheckWrite failed: %v", err)
				}

				data := []byte("test")
				if err := stream.Write(data); err != nil {
					t.Fatalf("Write failed: %v", err)
				}

				synctest.Wait()
			}

			expected := strings.Repeat("test", 5)
			if got := w.String(); got != expected {
				t.Errorf("expected %q, got %q", expected, got)
			}
		})
	})
}

func TestWriterOutputStream_BlockingWriteAndFlush(t *testing.T) {
	t.Run("writes and flushes data", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			data := []byte("test data")
			err := stream.BlockingWriteAndFlush(data)
			if err != nil {
				t.Fatalf("BlockingWriteAndFlush failed: %v", err)
			}

			if got := w.String(); got != "test data" {
				t.Errorf("expected 'test data', got %q", got)
			}
		})
	})

	t.Run("returns error when exceeds max size", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		data := make([]byte, maxWriteSize+1)
		err := stream.BlockingWriteAndFlush(data)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("handles write error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testErr := errors.New("write failed")
			w := &errorWriter{err: testErr}

			stream := NewWriterOutputStream(w)
			defer stream.Close()

			data := []byte("test")
			err := stream.BlockingWriteAndFlush(data)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, testErr) {
				t.Errorf("expected error %v, got %v", testErr, err)
			}
		})
	})

	t.Run("multiple sequential blocking writes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			for i := 0; i < 3; i++ {
				data := []byte("block")
				err := stream.BlockingWriteAndFlush(data)
				if err != nil {
					t.Fatalf("BlockingWriteAndFlush failed: %v", err)
				}
			}

			expected := strings.Repeat("block", 3)
			if got := w.String(); got != expected {
				t.Errorf("expected %q, got %q", expected, got)
			}
		})
	})
}

func TestWriterOutputStream_Flush(t *testing.T) {
	t.Run("non-blocking flush returns immediately", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		err := stream.Flush()
		if err != nil {
			t.Fatalf("Flush failed: %v", err)
		}
	})

	t.Run("returns error after stream error", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		testErr := errors.New("test error")
		stream.closeOnErr(testErr)

		err := stream.Flush()
		if !errors.Is(err, testErr) {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})
}

func TestWriterOutputStream_BlockingFlush(t *testing.T) {
	t.Run("waits for pending writes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// Write some data
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			data := []byte("test")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// BlockingFlush should wait for the write to complete
			err = stream.BlockingFlush()
			if err != nil {
				t.Fatalf("BlockingFlush failed: %v", err)
			}

			if got := w.String(); got != "test" {
				t.Errorf("expected 'test', got %q", got)
			}
		})
	})

	t.Run("returns error after stream error", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		testErr := errors.New("test error")
		stream.closeOnErr(testErr)

		err := stream.BlockingFlush()
		if !errors.Is(err, testErr) {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})
}

func TestWriterOutputStream_Subscribe(t *testing.T) {
	t.Run("calls callback when buffer available", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// Allocate the buffer
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			// Write to free the buffer
			data := []byte("test")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Subscribe should be called when we write and buffer becomes free
			called := atomic.Bool{}
			stream.Subscribe(func() {
				called.Store(true)
			})

			// Subscription should not have been called yet (buffer still allocated)
			if called.Load() {
				t.Error("subscription called too early")
			}

			// Wait for background processing
			synctest.Wait()

			// Now subscription should have been called
			if !called.Load() {
				t.Error("subscription was not called")
			}
		})
	})

	t.Run("calls callback immediately if buffer available", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		called := atomic.Bool{}
		stream.Subscribe(func() {
			called.Store(true)
		})

		if !called.Load() {
			t.Error("subscription was not called immediately")
		}
	})

	t.Run("calls callback immediately on error", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		stream.closeOnErr(io.EOF)

		called := atomic.Bool{}
		stream.Subscribe(func() {
			called.Store(true)
		})

		if !called.Load() {
			t.Error("subscription was not called on error")
		}
	})

	t.Run("multiple subscriptions", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// Allocate buffer
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			// Add multiple subscriptions
			count := atomic.Int32{}
			for i := 0; i < 3; i++ {
				stream.Subscribe(func() {
					count.Add(1)
				})
			}

			// Write to trigger notifications
			data := []byte("test")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			synctest.Wait()

			if got := count.Load(); got != 3 {
				t.Errorf("expected 3 calls, got %d", got)
			}
		})
	})

	t.Run("subscription after buffer allocated", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		// Allocate buffer
		_, err := stream.CheckWrite()
		if err != nil {
			t.Fatalf("CheckWrite failed: %v", err)
		}

		// Subscribe should be called immediately since buffer is allocated
		called := atomic.Bool{}
		stream.Subscribe(func() {
			called.Store(true)
		})

		if !called.Load() {
			t.Error("subscription was not called immediately")
		}
	})
}

func TestWriterOutputStream_WriteZeroes(t *testing.T) {
	t.Run("writes zeros successfully", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			n := uint64(100)
			err = stream.WriteZeroes(n)
			if err != nil {
				t.Fatalf("WriteZeroes failed: %v", err)
			}

			synctest.Wait()

			if w.buf.Len() != 100 {
				t.Errorf("expected 100 bytes, got %d", w.buf.Len())
			}

			// Verify all zeros
			data := w.buf.Bytes()
			for i, b := range data {
				if b != 0 {
					t.Errorf("byte at index %d is not zero: %d", i, b)
					break
				}
			}
		})
	})

	t.Run("returns error when exceeds max size", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		err := stream.WriteZeroes(maxWriteSize + 1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriterOutputStream_BlockingWriteZeroesAndFlush(t *testing.T) {
	t.Run("writes zeros and flushes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			n := uint64(50)
			err := stream.BlockingWriteZeroesAndFlush(n)
			if err != nil {
				t.Fatalf("BlockingWriteZeroesAndFlush failed: %v", err)
			}

			if w.buf.Len() != 50 {
				t.Errorf("expected 50 bytes, got %d", w.buf.Len())
			}

			// Verify all zeros
			data := w.buf.Bytes()
			for i, b := range data {
				if b != 0 {
					t.Errorf("byte at index %d is not zero: %d", i, b)
					break
				}
			}
		})
	})

	t.Run("returns error when exceeds max size", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)
		defer stream.Close()

		err := stream.BlockingWriteZeroesAndFlush(maxWriteSize + 1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriterOutputStream_Splice(t *testing.T) {
	t.Run("splices data from input stream", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			input := []byte("splice test data")
			inputStream := NewReaderInputStream(bytes.NewReader(input), 1024, 512, 2)
			defer inputStream.Close()
			w := newMockWriter()
			outputStream := NewWriterOutputStream(w)
			defer outputStream.Close()

			// Wait for input stream to buffer data
			synctest.Wait()

			n, err := outputStream.Splice(inputStream, uint64(len(input)))
			if err != nil {
				t.Fatalf("Splice failed: %v", err)
			}

			if n != uint64(len(input)) {
				t.Errorf("expected %d bytes spliced, got %d", len(input), n)
			}

			synctest.Wait()

			if got := w.String(); got != "splice test data" {
				t.Errorf("expected 'splice test data', got %q", got)
			}
		})
	})

	t.Run("limits splice to available buffer size", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			input := make([]byte, maxWriteSize*2)
			for i := range input {
				input[i] = byte(i % 256)
			}
			inputStream := NewReaderInputStream(bytes.NewReader(input), maxWriteSize*2, maxWriteSize*2, 2)
			defer inputStream.Close()
			w := newMockWriter()
			outputStream := NewWriterOutputStream(w)
			defer outputStream.Close()

			// Wait for input stream to buffer data
			synctest.Wait()

			n, err := outputStream.Splice(inputStream, uint64(len(input)))
			if err != nil {
				t.Fatalf("Splice failed: %v", err)
			}

			if n > maxWriteSize {
				t.Errorf("expected at most %d bytes spliced, got %d", maxWriteSize, n)
			}

			synctest.Wait()
		})
	})

	t.Run("returns error from input stream", func(t *testing.T) {
		testErr := errors.New("read error")
		inputStream := &errorInputStream{err: testErr}
		w := newMockWriter()

		outputStream := NewWriterOutputStream(w)
		defer outputStream.Close()

		_, err := outputStream.Splice(inputStream, 100)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, testErr) {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})
}

func TestWriterOutputStream_BlockingSplice(t *testing.T) {
	t.Run("splices data with blocking", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			input := []byte("blocking splice")
			inputStream := NewReaderInputStream(bytes.NewReader(input), 1024, 512, 2)
			defer inputStream.Close()
			w := newMockWriter()
			outputStream := NewWriterOutputStream(w)
			defer outputStream.Close()

			// Wait for input stream to buffer data
			synctest.Wait()

			n, err := outputStream.BlockingSplice(inputStream, uint64(len(input)))
			if err != nil {
				t.Fatalf("BlockingSplice failed: %v", err)
			}

			if n != uint64(len(input)) {
				t.Errorf("expected %d bytes spliced, got %d", len(input), n)
			}

			if got := w.String(); got != "blocking splice" {
				t.Errorf("expected 'blocking splice', got %q", got)
			}
		})
	})

	t.Run("limits to maxWriteSize", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			input := make([]byte, maxWriteSize*2)
			inputStream := NewReaderInputStream(bytes.NewReader(input), maxWriteSize*2, maxWriteSize*2, 2)
			defer inputStream.Close()
			w := newMockWriter()
			outputStream := NewWriterOutputStream(w)
			defer outputStream.Close()

			// Wait for input stream to buffer data
			synctest.Wait()

			n, err := outputStream.BlockingSplice(inputStream, uint64(len(input)))
			if err != nil {
				t.Fatalf("BlockingSplice failed: %v", err)
			}

			if n != maxWriteSize {
				t.Errorf("expected %d bytes spliced, got %d", maxWriteSize, n)
			}
		})
	})
}

func TestWriterOutputStream_Close(t *testing.T) {
	t.Run("closes underlying writer", func(t *testing.T) {
		cw := &closableWriter{mockWriter: newMockWriter()}
		stream := NewWriterOutputStream(cw)

		err := stream.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		if !cw.closed.Load() {
			t.Error("underlying writer was not closed")
		}
	})

	t.Run("does not error on non-closable writer", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)

		err := stream.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	t.Run("sets EOF error", func(t *testing.T) {
		w := newMockWriter()
		stream := NewWriterOutputStream(w)

		stream.Close()

		_, err := stream.CheckWrite()
		if !errors.Is(err, io.EOF) {
			t.Errorf("expected EOF error, got %v", err)
		}
	})

	t.Run("notifies subscribers on close", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			w := newMockWriter()
			stream := NewWriterOutputStream(w)

			// Allocate buffer so subscription doesn't fire immediately
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			called := atomic.Bool{}
			stream.Subscribe(func() {
				called.Store(true)
			})

			stream.Close()
			synctest.Wait()

			if !called.Load() {
				t.Error("subscription was not called on close")
			}
		})
	})
}

func TestWriterOutputStream_ErrorHandling(t *testing.T) {
	t.Run("propagates write error to subsequent operations", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			testErr := errors.New("write failed")
			w := &errorWriter{err: testErr}
			stream := NewWriterOutputStream(w)
			defer stream.Close()

			// First write will fail in background
			_, err := stream.CheckWrite()
			if err != nil {
				t.Fatalf("CheckWrite failed: %v", err)
			}

			data := []byte("test")
			if err := stream.Write(data); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			synctest.Wait()

			// Subsequent operations should return the error
			_, err = stream.CheckWrite()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, testErr) {
				t.Errorf("expected error %v, got %v", testErr, err)
			}

			err = stream.Flush()
			if !errors.Is(err, testErr) {
				t.Errorf("expected error %v, got %v", testErr, err)
			}

			err = stream.BlockingFlush()
			if !errors.Is(err, testErr) {
				t.Errorf("expected error %v, got %v", testErr, err)
			}
		})
	})
}

// Helper types for testing

type errorInputStream struct {
	err error
}

func (e *errorInputStream) Read(n uint64) ([]byte, error) {
	return nil, e.err
}

func (e *errorInputStream) BlockingRead(n uint64) ([]byte, error) {
	return nil, e.err
}

func (e *errorInputStream) Skip(n uint64) (uint64, error) {
	return 0, e.err
}

func (e *errorInputStream) BlockingSkip(n uint64) (uint64, error) {
	return 0, e.err
}

func (e *errorInputStream) Subscribe(fn func()) {
	fn()
}
