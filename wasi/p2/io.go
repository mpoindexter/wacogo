package p2

import (
	"errors"
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

type StreamError host.Variant[StreamError]

func (StreamError) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCaseValue(StreamErrorLastOperationFailed),
		host.VariantCase[StreamError](StreamErrorClosed),
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

func StreamErrorLastOperationFailed(e host.Own[IOError]) StreamError {
	return host.VariantConstructValue[StreamError](
		"last-operation-failed",
		e,
	)
}

func (v StreamError) LastOperationFailed() (host.Own[IOError], bool) {
	return host.VariantCast[host.Own[IOError]](v, "last-operation-failed")
}

func CreateErrorInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("error", host.ResourceTypeFor[IOError](hi, hi))

	hi.AddFunction("[method]error.to-debug-string", func(self host.Borrow[IOError]) componentmodel.String {
		return componentmodel.String(self.Resource().DebugString)
	})
	return hi
}

func CreatePollInstance() *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, hi))
	hi.AddFunction("[method]pollable.ready", func(self host.Borrow[Pollable]) componentmodel.Bool {
		return componentmodel.Bool(self.Resource().isReady())
	})
	hi.AddFunction("[method]pollable.block", func(self host.Borrow[Pollable]) {
		self.Resource().block()
	})
	hi.AddFunction("poll", func(pollables []host.Borrow[Pollable]) []componentmodel.U32 {
		result := make([]componentmodel.U32, 0, len(pollables))
		for i := range pollables {
			if pollables[i].Resource().isReady() {
				result = append(result, componentmodel.U32(i))
			}
		}
		return result
	})
	return hi
}

func toByteArray(data []byte, err error) (componentmodel.ByteArray, error) {
	if err != nil {
		return componentmodel.ByteArray{}, err
	}
	return componentmodel.ByteArray(data), nil
}

func toU64(n uint64, err error) (componentmodel.U64, error) {
	if err != nil {
		return 0, err
	}
	return componentmodel.U64(n), nil
}

func translateIOResponse[T any](data T, err error) Result[T, StreamError] {
	if err != nil {
		if errors.Is(err, io.EOF) {
			return ResultErr[T](
				StreamErrorClosed(),
			)
		}
		return ResultErr[T](
			StreamErrorLastOperationFailed(
				host.NewOwn[IOError](IOError{DebugString: err.Error()}),
			),
		)
	}
	return ResultOk[StreamError](data)
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

	hi.AddFunction("[method]input-stream.read", func(self host.Borrow[InputStream], len componentmodel.U64) Result[componentmodel.ByteArray, StreamError] {
		return translateIOResponse(toByteArray(self.Resource().Read(uint64(len))))
	})
	hi.AddFunction("[method]input-stream.blocking-read", func(self host.Borrow[InputStream], len componentmodel.U64) Result[componentmodel.ByteArray, StreamError] {
		return translateIOResponse(toByteArray(self.Resource().BlockingRead(uint64(len))))
	})
	hi.AddFunction("[method]input-stream.skip", func(self host.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return translateIOResponse(toU64(self.Resource().Skip(uint64(n))))
	})
	hi.AddFunction("[method]input-stream.blocking-skip", func(self host.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return translateIOResponse(toU64(self.Resource().BlockingSkip(uint64(n))))
	})
	hi.AddFunction("[method]input-stream.subscribe", func(self host.Borrow[InputStream]) host.Own[Pollable] {
		ch := make(chan struct{})
		p := NewChanPollable(ch)
		self.Resource().Subscribe(func() {
			close(ch)
		})
		return host.NewOwn[Pollable](p)
	})

	hi.AddFunction("[method]output-stream.check-write", func(self host.Borrow[OutputStream]) Result[componentmodel.U64, StreamError] {
		return translateIOResponse(toU64(self.Resource().CheckWrite()))
	})

	hi.AddFunction("[method]output-stream.write", func(self host.Borrow[OutputStream], contents componentmodel.ByteArray) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().Write(contents))
	})

	hi.AddFunction("[method]output-stream.blocking-write-and-flush", func(self host.Borrow[OutputStream], contents componentmodel.ByteArray) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().BlockingWriteAndFlush(contents))
	})

	hi.AddFunction("[method]output-stream.flush", func(self host.Borrow[OutputStream]) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().Flush())
	})

	hi.AddFunction("[method]output-stream.blocking-flush", func(self host.Borrow[OutputStream]) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().BlockingFlush())
	})

	hi.AddFunction("[method]output-stream.subscribe", func(self host.Borrow[OutputStream]) host.Own[Pollable] {
		ch := make(chan struct{})
		p := NewChanPollable(ch)
		self.Resource().Subscribe(func() {
			close(ch)
		})
		return host.NewOwn[Pollable](p)
	})

	hi.AddFunction("[method]output-stream.write-zeroes", func(self host.Borrow[OutputStream], n componentmodel.U64) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().WriteZeroes(uint64(n)))
	})

	hi.AddFunction("[method]output-stream.blocking-write-zeroes-and-flush", func(self host.Borrow[OutputStream], n componentmodel.U64) Result[Void, StreamError] {
		return translateIOResponse(Void{}, self.Resource().BlockingWriteZeroesAndFlush(uint64(n)))
	})

	hi.AddFunction("[method]output-stream.splice", func(self host.Borrow[OutputStream], src host.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return translateIOResponse(toU64(self.Resource().Splice(src.Resource(), uint64(n))))
	})

	hi.AddFunction("[method]output-stream.blocking-splice", func(self host.Borrow[OutputStream], src host.Borrow[InputStream], n componentmodel.U64) Result[componentmodel.U64, StreamError] {
		return translateIOResponse(toU64(self.Resource().BlockingSplice(src.Resource(), uint64(n))))
	})

	return hi
}
