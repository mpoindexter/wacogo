package p2

import (
	"io"

	"github.com/partite-ai/wacogo/model"
	"github.com/partite-ai/wacogo/model/host"
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

func (s OutputStream) write(contents model.ByteArray) Result[Void, StreamError] {
	data := make([]byte, len(contents))
	for i := range contents {
		data[i] = byte(contents[i])
	}

	_, err := s.w.Write(data)
	if err != nil {
		return ResultErr[Void](
			StreamErrorLastOperationFailed(
				model.Own[IOError]{
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

func (s InputStream) read(n uint64) Result[model.ByteArray, StreamError] {
	buf := make([]byte, n)
	bytesRead, err := s.r.Read(buf)
	if err != nil {
		return ResultErr[model.ByteArray](
			StreamErrorLastOperationFailed(
				model.Own[IOError]{
					Resource: IOError{DebugString: err.Error()},
				},
			),
		)
	}
	return ResultOk[StreamError](model.ByteArray(buf[:bytesRead]))
}

func (s InputStream) skip(n uint64) Result[model.U64, StreamError] {
	bytesRead, err := io.CopyN(io.Discard, s.r, int64(n))
	if err != nil {
		return ResultErr[model.U64](
			StreamErrorLastOperationFailed(
				model.Own[IOError]{
					Resource: IOError{DebugString: err.Error()},
				},
			),
		)
	}
	return ResultOk[StreamError](model.U64(uint64(bytesRead)))
}

func (s InputStream) close() {
	if closer, ok := s.r.(io.Closer); ok {
		closer.Close()
	}
}

type StreamError host.Variant[StreamError]

func (StreamError) ValueType(inst *host.Instance) model.ValueType {
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

func StreamErrorLastOperationFailed(e model.Own[IOError]) StreamError {
	return host.VariantConstructValue[StreamError](
		"last-operation-failed",
		e,
	)
}

func (v StreamError) LastOperationFailed() (model.Own[IOError], bool) {
	return host.VariantCast[model.Own[IOError]](v, "last-operation-failed")
}

func CreateErrorInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("error", host.ResourceTypeFor[IOError](hi, hi))

	hi.AddFunction("[method]error.to-debug-string", func(self model.Borrow[IOError]) model.String {
		return model.String(self.Resource.DebugString)
	})
	return hi
}

func CreatePollInstance() *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, hi))
	hi.AddFunction("[method]pollable.ready", func(self model.Borrow[Pollable]) model.Bool {
		return model.Bool(self.Resource.isReady())
	})
	hi.AddFunction("[method]pollable.block", func(self model.Borrow[Pollable]) {
		self.Resource.block()
	})
	hi.AddFunction("poll", func(pollables []model.Borrow[Pollable]) []model.U32 {
		result := make([]model.U32, 0, len(pollables))
		for i := range pollables {
			if pollables[i].Resource.isReady() {
				result = append(result, model.U32(i))
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

	hi.AddFunction("[method]input-stream.read", func(self model.Borrow[InputStream], len model.U64) Result[model.ByteArray, StreamError] {
		return self.Resource.read(uint64(len))
	})
	hi.AddFunction("[method]input-stream.blocking-read", func(self model.Borrow[InputStream], len model.U64) Result[model.ByteArray, StreamError] {
		return self.Resource.read(uint64(len))
	})
	hi.AddFunction("[method]input-stream.skip", func(self model.Borrow[InputStream], n model.U64) Result[model.U64, StreamError] {
		return self.Resource.skip(uint64(n))
	})
	hi.AddFunction("[method]input-stream.blocking-skip", func(self model.Borrow[InputStream], n model.U64) Result[model.U64, StreamError] {
		return self.Resource.skip(uint64(n))
	})
	hi.AddFunction("[method]input-stream.subscribe", func(self model.Borrow[InputStream]) model.Own[Pollable] {
		return model.Own[Pollable]{
			Resource: AlwaysReadyPollable{},
		}
	})

	hi.AddFunction("[method]output-stream.check-write", func(self model.Borrow[OutputStream]) Result[model.U64, StreamError] {
		return ResultOk[StreamError](model.U64(4096))
	})

	hi.AddFunction("[method]output-stream.write", func(self model.Borrow[OutputStream], contents model.ByteArray) Result[Void, StreamError] {
		return self.Resource.write(contents)
	})

	hi.AddFunction("[method]output-stream.blocking-write-and-flush", func(self model.Borrow[OutputStream], contents model.ByteArray) Result[Void, StreamError] {
		return self.Resource.write(contents)
	})

	hi.AddFunction("[method]output-stream.flush", func(self model.Borrow[OutputStream]) Result[Void, StreamError] {
		return ResultOk[StreamError](Void{})
	})

	hi.AddFunction("[method]output-stream.blocking-flush", func(self model.Borrow[OutputStream]) Result[Void, StreamError] {
		return ResultOk[StreamError](Void{})
	})

	hi.AddFunction("[method]output-stream.subscribe", func(self model.Borrow[OutputStream]) model.Own[Pollable] {
		return model.Own[Pollable]{
			Resource: AlwaysReadyPollable{},
		}
	})

	hi.AddFunction("[method]output-stream.write-zeroes", func(self model.Borrow[OutputStream], n model.U64) Result[Void, StreamError] {
		zeroes := make(model.ByteArray, n)
		return self.Resource.write(zeroes)
	})

	hi.AddFunction("[method]output-stream.blocking-write-zeroes-and-flush", func(self model.Borrow[OutputStream], n model.U64) Result[Void, StreamError] {
		zeroes := make(model.ByteArray, n)
		return self.Resource.write(zeroes)
	})

	hi.AddFunction("[method]output-stream.splice", func(self model.Borrow[OutputStream], src model.Borrow[InputStream], n model.U64) Result[model.U64, StreamError] {
		copied, err := io.CopyN(self.Resource.w, src.Resource.r, int64(n))
		if err != nil {
			return ResultErr[model.U64](
				StreamErrorLastOperationFailed(
					model.Own[IOError]{
						Resource: IOError{DebugString: err.Error()},
					},
				),
			)
		}
		return ResultOk[StreamError](model.U64(uint64(copied)))
	})

	hi.AddFunction("[method]output-stream.blocking-splice", func(self model.Borrow[OutputStream], src model.Borrow[InputStream], n model.U64) Result[model.U64, StreamError] {
		copied, err := io.CopyN(self.Resource.w, src.Resource.r, int64(n))
		if err != nil {
			return ResultErr[model.U64](
				StreamErrorLastOperationFailed(
					model.Own[IOError]{
						Resource: IOError{DebugString: err.Error()},
					},
				),
			)
		}
		return ResultOk[StreamError](model.U64(uint64(copied)))
	})

	return hi
}
