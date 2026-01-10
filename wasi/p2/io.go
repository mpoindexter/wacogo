package p2

import (
	"io"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type IOError struct {
	DebugString string
}

type Pollable interface {
	isReady() bool
	block()
}

type AlwaysReadyPollable struct{}

func (AlwaysReadyPollable) isReady() bool {
	return true
}

func (AlwaysReadyPollable) block() {}

type ChanPollable[T any] struct {
	C <-chan T
}

func (p ChanPollable[T]) isReady() bool {
	select {
	case <-p.C:
		return true
	default:
		return false
	}
}

func (p ChanPollable[T]) block() {
	<-p.C
}

func NewChanPollable[T any](ch <-chan T) ChanPollable[T] {
	return ChanPollable[T]{
		C: ch,
	}
}

type OutputStream struct {
	w io.Writer
}

func (OutputStream) Resource() {}

func (s OutputStream) write(contents componentmodel.ByteArray) Result[Void, StreamError] {
	data := make([]byte, len(contents))
	for i := range contents {
		data[i] = byte(contents[i])
	}

	_, err := s.w.Write(data)
	if err != nil {
		return ResultErr[Void](
			StreamErrorLastOperationFailed(
				componentmodel.Own[IOError]{
					Resource: IOError{DebugString: err.Error()},
				},
			),
		)
	}
	return ResultOk[StreamError](Void{})
}

func (s OutputStream) close() {
	if closer, ok := s.w.(io.Closer); ok {
		closer.Close()
	}
}

type InputStream struct {
	r io.Reader
}

func (InputStream) Resource() {}

func (s InputStream) read(n uint64) Result[componentmodel.ByteArray, StreamError] {
	buf := make([]byte, n)
	bytesRead, err := s.r.Read(buf)
	if err != nil {
		return ResultErr[componentmodel.ByteArray](
			StreamErrorLastOperationFailed(
				componentmodel.Own[IOError]{
					Resource: IOError{DebugString: err.Error()},
				},
			),
		)
	}
	return ResultOk[StreamError](componentmodel.ByteArray(buf[:bytesRead]))
}

func (s InputStream) skip(n uint64) Result[componentmodel.U64, StreamError] {
	bytesRead, err := io.CopyN(io.Discard, s.r, int64(n))
	if err != nil {
		return ResultErr[componentmodel.U64](
			StreamErrorLastOperationFailed(
				componentmodel.Own[IOError]{
					Resource: IOError{DebugString: err.Error()},
				},
			),
		)
	}
	return ResultOk[StreamError](componentmodel.U64(uint64(bytesRead)))
}

func (s InputStream) close() {
	if closer, ok := s.r.(io.Closer); ok {
		closer.Close()
	}
}

type StreamError host.Variant[StreamError]

func (StreamError) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCase[StreamError](StreamErrorClosed),
		host.VariantCaseValue(StreamErrorLastOperationFailed),
	)
}

func StreamErrorClosed() StreamError {
	return host.VariantConstruct[StreamError](
		"closed",
	)
}

func (v StreamError) IsClosed() bool {
	return host.VariantTest(v, "closed")
}

func StreamErrorLastOperationFailed(e componentmodel.Own[IOError]) StreamError {
	return host.VariantConstructValue[StreamError](
		"last-operation-failed",
		e,
	)
}

func (v StreamError) LastOperationFailed() (componentmodel.Own[IOError], bool) {
	return host.VariantCast[componentmodel.Own[IOError]](v, "last-operation-failed")
}

func CreateErrorInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("error", host.ResourceTypeFor[IOError](hi, hi))

	hi.AddFunction("[method]error.to-debug-string", func(self componentmodel.Borrow[IOError]) componentmodel.String {
		return componentmodel.String(self.Resource.DebugString)
	})
	return hi
}

func CreatePollInstance() *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, hi))
	hi.AddFunction("[method]pollable.ready", func(self componentmodel.Borrow[Pollable]) componentmodel.Bool {
		return componentmodel.Bool(self.Resource.isReady())
	})
	hi.AddFunction("[method]pollable.block", func(self componentmodel.Borrow[Pollable]) {
		self.Resource.block()
	})
	hi.AddFunction("poll", func(pollables []componentmodel.Borrow[Pollable]) []componentmodel.U32 {
		result := make([]componentmodel.U32, 0, len(pollables))
		for i := range pollables {
			if pollables[i].Resource.isReady() {
				result = append(result, componentmodel.U32(i))
			}
		}
		return result
	})
	return hi
}

func CreateStreamsInstance(
	errorInstance *host.Instance,
	pollInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("error", host.ResourceTypeFor[IOError](hi, errorInstance))
	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, pollInstance))

	hi.AddTypeExport("input-stream", host.ResourceTypeFor[InputStream](hi, hi))
	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, hi))
	hi.AddTypeExport("stream-error", host.ValueTypeFor[StreamError](hi))

	hi.AddFunction("[method]input-stream.read", func(self componentmodel.Borrow[InputStream], len componentmodel.U64) Result[componentmodel.ByteArray, StreamError] {
		return self.Resource.read(uint64(len))
	})
	hi.AddFunction("[method]input-stream.blocking-read", func(self componentmodel.Borrow[InputStream], len componentmodel.U64) Result[componentmodel.ByteArray, StreamError] {
		return self.Resource.read(uint64(len))
	})
	hi.AddFunction("[method]input-stream.skip", func(self componentmodel.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return self.Resource.skip(uint64(n))
	})
	hi.AddFunction("[method]input-stream.blocking-skip", func(self componentmodel.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return self.Resource.skip(uint64(n))
	})
	hi.AddFunction("[method]input-stream.subscribe", func(self componentmodel.Borrow[InputStream]) componentmodel.Own[Pollable] {
		return componentmodel.Own[Pollable]{
			Resource: AlwaysReadyPollable{},
		}
	})

	hi.AddFunction("[method]output-stream.check-write", func(self componentmodel.Borrow[OutputStream]) Result[componentmodel.U64, StreamError] {
		return ResultOk[StreamError](componentmodel.U64(4096))
	})

	hi.AddFunction("[method]output-stream.write", func(self componentmodel.Borrow[OutputStream], contents componentmodel.ByteArray) Result[Void, StreamError] {
		return self.Resource.write(contents)
	})

	hi.AddFunction("[method]output-stream.blocking-write-and-flush", func(self componentmodel.Borrow[OutputStream], contents componentmodel.ByteArray) Result[Void, StreamError] {
		return self.Resource.write(contents)
	})

	hi.AddFunction("[method]output-stream.flush", func(self componentmodel.Borrow[OutputStream]) Result[Void, StreamError] {
		return ResultOk[StreamError](Void{})
	})

	hi.AddFunction("[method]output-stream.blocking-flush", func(self componentmodel.Borrow[OutputStream]) Result[Void, StreamError] {
		return ResultOk[StreamError](Void{})
	})

	hi.AddFunction("[method]output-stream.subscribe", func(self componentmodel.Borrow[OutputStream]) componentmodel.Own[Pollable] {
		return componentmodel.Own[Pollable]{
			Resource: AlwaysReadyPollable{},
		}
	})

	hi.AddFunction("[method]output-stream.write-zeroes", func(self componentmodel.Borrow[OutputStream], n componentmodel.U64) Result[Void, StreamError] {
		zeroes := make(componentmodel.ByteArray, n)
		return self.Resource.write(zeroes)
	})

	hi.AddFunction("[method]output-stream.blocking-write-zeroes-and-flush", func(self componentmodel.Borrow[OutputStream], n componentmodel.U64) Result[Void, StreamError] {
		zeroes := make(componentmodel.ByteArray, n)
		return self.Resource.write(zeroes)
	})

	hi.AddFunction("[method]output-stream.splice", func(self componentmodel.Borrow[OutputStream], src componentmodel.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		copied, err := io.CopyN(self.Resource.w, src.Resource.r, int64(n))
		if err != nil {
			return ResultErr[componentmodel.U64](
				StreamErrorLastOperationFailed(
					componentmodel.Own[IOError]{
						Resource: IOError{DebugString: err.Error()},
					},
				),
			)
		}
		return ResultOk[StreamError](componentmodel.U64(uint64(copied)))
	})

	hi.AddFunction("[method]output-stream.blocking-splice", func(self componentmodel.Borrow[OutputStream], src componentmodel.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		copied, err := io.CopyN(self.Resource.w, src.Resource.r, int64(n))
		if err != nil {
			return ResultErr[componentmodel.U64](
				StreamErrorLastOperationFailed(
					componentmodel.Own[IOError]{
						Resource: IOError{DebugString: err.Error()},
					},
				),
			)
		}
		return ResultOk[StreamError](componentmodel.U64(uint64(copied)))
	})

	return hi
}
