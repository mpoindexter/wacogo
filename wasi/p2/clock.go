package p2

import (
	"time"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type Instant uint64

type Duration uint64

type DateTime host.Record[DateTime]

func NewDateTime(seconds uint64, nanoseconds uint32) DateTime {
	return host.RecordConstruct[DateTime](
		host.RecordField("seconds", seconds),
		host.RecordField("nanoseconds", nanoseconds),
	)
}

func (DateTime) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[DateTime](inst, NewDateTime)
}

func (dt DateTime) Seconds() uint64 {
	return host.RecordFieldGetIndex[uint64](dt, 0)
}

func (dt DateTime) Nanoseconds() uint32 {
	return host.RecordFieldGetIndex[uint32](dt, 1)
}

func CreateMonotonicClockInstance(
	pollInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("instant", host.ValueTypeFor[Instant](hi))
	hi.AddTypeExport("duration", host.ValueTypeFor[Duration](hi))
	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, pollInstance))

	hi.MustAddFunction("now", func() componentmodel.U64 {
		return componentmodel.U64(time.Now().UnixNano())
	})
	hi.MustAddFunction("resolution", func() componentmodel.U64 {
		return componentmodel.U64(1)
	})
	hi.MustAddFunction("subscribe-instant", func(d componentmodel.U64) host.Own[Pollable] {
		target := time.Unix(0, int64(d))
		delta := time.Until(target)
		if delta <= 0 {
			return host.NewOwn[Pollable](AlwaysReadyPollable{})
		}

		return host.NewOwn[Pollable](NewChanPollable(time.After(delta)))
	})

	hi.MustAddFunction("subscribe-duration", func(d componentmodel.U64) host.Own[Pollable] {
		return host.NewOwn[Pollable](NewChanPollable(time.After(time.Duration(d))))
	})

	return hi
}

func CreateWallClockInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("datetime", host.ValueTypeFor[DateTime](hi))

	hi.MustAddFunction("now", func() DateTime {
		now := time.Now()
		return NewDateTime(uint64(now.Unix()), uint32(now.Nanosecond()))
	})

	hi.MustAddFunction("resolution", func() DateTime {
		return NewDateTime(0, 1)
	})

	return hi
}
