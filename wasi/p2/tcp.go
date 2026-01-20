package p2

import "github.com/partite-ai/wacogo/componentmodel/host"

type ShutdownType host.Enum[ShutdownType]

func (ShutdownType) EnumValues() []string {
	return []string{
		"receive",
		"send",
		"both",
	}
}

type TcpSocket struct {
}

func CreateTcpInstance(
	streamsInstance *host.Instance,
	pollInstance *host.Instance,
	networkInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("tcp-socket", host.ResourceTypeFor[TcpSocket](hi, hi))
	hi.AddTypeExport("network", host.ResourceTypeFor[Network](hi, networkInstance))
	hi.AddTypeExport("error-code", host.ValueTypeFor[NetworkErrorCode](hi))
	hi.AddTypeExport("input-stream", host.ResourceTypeFor[InputStream](hi, streamsInstance))
	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, streamsInstance))
	hi.AddTypeExport("ip-socket-address", host.ValueTypeFor[IpSocketAddress](hi))
	hi.AddTypeExport("ip-address-family", host.ValueTypeFor[IpAddressFamily](hi))
	hi.AddTypeExport("duration", host.ValueTypeFor[Duration](hi))
	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, pollInstance))
	hi.AddTypeExport("shutdown-type", host.ValueTypeFor[ShutdownType](hi))

	hi.MustAddFunction("[method]tcp-socket.start-bind", func(self host.Borrow[TcpSocket], network host.Borrow[Network], localAddress IpSocketAddress) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.finish-bind", func(self host.Borrow[TcpSocket]) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.start-connect", func(self host.Borrow[TcpSocket], network host.Borrow[Network], remoteAddress IpSocketAddress) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.finish-connect", func(self host.Borrow[TcpSocket]) Result[Tuple2[host.Own[InputStream], host.Own[OutputStream]], NetworkErrorCode] {
		return ResultErr[Tuple2[host.Own[InputStream], host.Own[OutputStream]]](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.start-listen", func(self host.Borrow[TcpSocket]) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.finish-listen", func(self host.Borrow[TcpSocket]) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.accept", func(self host.Borrow[TcpSocket]) Result[Tuple3[host.Own[TcpSocket], host.Own[InputStream], host.Own[OutputStream]], NetworkErrorCode] {
		return ResultErr[Tuple3[host.Own[TcpSocket], host.Own[InputStream], host.Own[OutputStream]]](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.local-address", func(self host.Borrow[TcpSocket]) Result[IpSocketAddress, NetworkErrorCode] {
		return ResultErr[IpSocketAddress](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.remote-address", func(self host.Borrow[TcpSocket]) Result[IpSocketAddress, NetworkErrorCode] {
		return ResultErr[IpSocketAddress](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.is-listening", func(self host.Borrow[TcpSocket]) bool {
		return false
	})

	hi.MustAddFunction("[method]tcp-socket.address-family", func(self host.Borrow[TcpSocket]) IpAddressFamily {
		return IpAddressFamily("ipv4")
	})

	hi.MustAddFunction("[method]tcp-socket.set-listen-backlog-size", func(self host.Borrow[TcpSocket], size uint64) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.keep-alive-enabled", func(self host.Borrow[TcpSocket]) Result[bool, NetworkErrorCode] {
		return ResultErr[bool](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-keep-alive-enabled", func(self host.Borrow[TcpSocket], value bool) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.keep-alive-idle-time", func(self host.Borrow[TcpSocket]) Result[Duration, NetworkErrorCode] {
		return ResultErr[Duration](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-keep-alive-idle-time", func(self host.Borrow[TcpSocket], value Duration) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.keep-alive-interval", func(self host.Borrow[TcpSocket]) Result[Duration, NetworkErrorCode] {
		return ResultErr[Duration](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-keep-alive-interval", func(self host.Borrow[TcpSocket], value Duration) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.keep-alive-count", func(self host.Borrow[TcpSocket]) Result[uint32, NetworkErrorCode] {
		return ResultErr[uint32](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-keep-alive-count", func(self host.Borrow[TcpSocket], value uint32) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.hop-limit", func(self host.Borrow[TcpSocket]) Result[uint8, NetworkErrorCode] {
		return ResultErr[uint8](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-hop-limit", func(self host.Borrow[TcpSocket], value uint8) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.receive-buffer-size", func(self host.Borrow[TcpSocket]) Result[uint64, NetworkErrorCode] {
		return ResultErr[uint64](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-receive-buffer-size", func(self host.Borrow[TcpSocket], value uint64) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.send-buffer-size", func(self host.Borrow[TcpSocket]) Result[uint64, NetworkErrorCode] {
		return ResultErr[uint64](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.set-send-buffer-size", func(self host.Borrow[TcpSocket], value uint64) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]tcp-socket.subscribe", func(self host.Borrow[TcpSocket]) host.Own[Pollable] {
		return host.NewOwn[Pollable](AlwaysReadyPollable{})
	})

	hi.MustAddFunction("[method]tcp-socket.shutdown", func(self host.Borrow[TcpSocket], shutdownType ShutdownType) Result[Void, NetworkErrorCode] {
		return ResultErr[Void](NetworkErrorCode("not-supported"))
	})

	return hi
}

func CreateTcpCreateSocketInstance(
	networkInstance *host.Instance,
	tcpInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("network", host.ResourceTypeFor[Network](hi, networkInstance))
	hi.AddTypeExport("tcp-socket", host.ResourceTypeFor[TcpSocket](hi, tcpInstance))
	hi.AddTypeExport("ip-address-family", host.ValueTypeFor[IpAddressFamily](hi))
	hi.AddTypeExport("error-code", host.ValueTypeFor[NetworkErrorCode](hi))

	hi.MustAddFunction("create-tcp-socket", func(addressFamily IpAddressFamily) Result[host.Own[TcpSocket], NetworkErrorCode] {
		return ResultErr[host.Own[TcpSocket]](NetworkErrorCode("not-supported"))
	})
	return hi
}
