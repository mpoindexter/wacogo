package p2

import (
	"net"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type Network struct {
}

type NetworkErrorCode host.Enum[NetworkErrorCode]

func (NetworkErrorCode) EnumValues() []string {
	return []string{
		"unknown",
		"access-denied",
		"not-supported",
		"invalid-argument",
		"out-of-memory",
		"timeout",
		"concurrency-conflict",
		"not-in-progress",
		"would-block",
		"invalid-state",
		"new-socket-limit",
		"address-not-bindable",
		"address-in-use",
		"remote-unreachable",
		"connection-refused",
		"connection-reset",
		"connection-aborted",
		"datagram-too-large",
		"name-unresolvable",
		"temporary-resolver-failure",
		"permanent-resolver-failure",
	}
}

type IpAddressFamily host.Enum[IpAddressFamily]

func (IpAddressFamily) EnumValues() []string {
	return []string{
		"ipv4",
		"ipv6",
	}
}

type IpV4Address host.Tuple[struct {
	A host.TupleField[IpV4Address, uint8]
	B host.TupleField[IpV4Address, uint8]
	C host.TupleField[IpV4Address, uint8]
	D host.TupleField[IpV4Address, uint8]
}]

func NewIpV4Address(addr net.IP) IpV4Address {
	if len(addr) != net.IPv4len {
		addr = make([]byte, net.IPv4len)
	}
	tpl := host.NewTuple[IpV4Address]()
	tpl.Fields.A.Set(tpl, uint8(addr[0]))
	tpl.Fields.B.Set(tpl, uint8(addr[1]))
	tpl.Fields.C.Set(tpl, uint8(addr[2]))
	tpl.Fields.D.Set(tpl, uint8(addr[3]))
	return tpl.Tuple()
}

func (a IpV4Address) ToNetIP() net.IP {
	return net.IPv4(
		a.Fields.A.Get(a),
		a.Fields.B.Get(a),
		a.Fields.C.Get(a),
		a.Fields.D.Get(a),
	)
}

type IpV6Address host.Tuple[struct {
	A host.TupleField[IpV6Address, uint16]
	B host.TupleField[IpV6Address, uint16]
	C host.TupleField[IpV6Address, uint16]
	D host.TupleField[IpV6Address, uint16]
	E host.TupleField[IpV6Address, uint16]
	F host.TupleField[IpV6Address, uint16]
	G host.TupleField[IpV6Address, uint16]
	H host.TupleField[IpV6Address, uint16]
}]

func NewIpV6Address(addr net.IP) IpV6Address {
	if len(addr) != net.IPv6len {
		addr = make([]byte, net.IPv6len)
	}
	tpl := host.NewTuple[IpV6Address]()
	tpl.Fields.A.Set(tpl, uint16(addr[0])<<8|uint16(addr[1]))
	tpl.Fields.B.Set(tpl, uint16(addr[2])<<8|uint16(addr[3]))
	tpl.Fields.C.Set(tpl, uint16(addr[4])<<8|uint16(addr[5]))
	tpl.Fields.D.Set(tpl, uint16(addr[6])<<8|uint16(addr[7]))
	tpl.Fields.E.Set(tpl, uint16(addr[8])<<8|uint16(addr[9]))
	tpl.Fields.F.Set(tpl, uint16(addr[10])<<8|uint16(addr[11]))
	tpl.Fields.G.Set(tpl, uint16(addr[12])<<8|uint16(addr[13]))
	tpl.Fields.H.Set(tpl, uint16(addr[14])<<8|uint16(addr[15]))
	return tpl.Tuple()
}

func (a IpV6Address) ToNetIP() net.IP {
	return net.IP{
		byte(a.Fields.A.Get(a) >> 8),
		byte(a.Fields.A.Get(a) & 0xff),
		byte(a.Fields.B.Get(a) >> 8),
		byte(a.Fields.B.Get(a) & 0xff),
		byte(a.Fields.C.Get(a) >> 8),
		byte(a.Fields.C.Get(a) & 0xff),
		byte(a.Fields.D.Get(a) >> 8),
		byte(a.Fields.D.Get(a) & 0xff),
		byte(a.Fields.E.Get(a) >> 8),
		byte(a.Fields.E.Get(a) & 0xff),
		byte(a.Fields.F.Get(a) >> 8),
		byte(a.Fields.F.Get(a) & 0xff),
		byte(a.Fields.G.Get(a) >> 8),
		byte(a.Fields.G.Get(a) & 0xff),
		byte(a.Fields.H.Get(a) >> 8),
		byte(a.Fields.H.Get(a) & 0xff),
	}
}

type IpAddress host.Variant[IpAddress]

func (IpAddress) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCaseValue(IpAddressIPV4),
		host.VariantCaseValue(IpAddressIPV6),
	)
}

func IpAddressIPV4(addr IpV4Address) IpAddress {
	return host.VariantConstructValue[IpAddress](
		"ipv4",
		addr,
	)
}

func IpAddressIPV6(addr IpV6Address) IpAddress {
	return host.VariantConstructValue[IpAddress](
		"ipv6",
		addr,
	)
}

type IpV4SocketAddress host.Record[struct {
	Port    host.RecordField[IpV4SocketAddress, uint16]
	Address host.RecordField[IpV4SocketAddress, IpV4Address]
}]

func NewIpV4SocketAddress(port uint16, address IpV4Address) IpV4SocketAddress {
	rec := host.NewRecord[IpV4SocketAddress]()
	rec.Fields.Port.Set(rec, port)
	rec.Fields.Address.Set(rec, address)
	return rec.Record()
}

type IpV6SocketAddress host.Record[struct {
	Port     host.RecordField[IpV6SocketAddress, uint16]
	FlowInfo host.RecordField[IpV6SocketAddress, uint32] `cm:"flow-info"`
	Address  host.RecordField[IpV6SocketAddress, IpV6Address]
	ScopeID  host.RecordField[IpV6SocketAddress, uint32] `cm:"scope-id"`
}]

func NewIpV6SocketAddress(port uint16, flowInfo uint32, address IpV6Address, scopeID uint32) IpV6SocketAddress {
	rec := host.NewRecord[IpV6SocketAddress]()
	rec.Fields.Port.Set(rec, port)
	rec.Fields.FlowInfo.Set(rec, flowInfo)
	rec.Fields.Address.Set(rec, address)
	rec.Fields.ScopeID.Set(rec, scopeID)
	return rec.Record()
}

type IpSocketAddress host.Variant[IpSocketAddress]

func (IpSocketAddress) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCaseValue(IpSocketAddressIPV4),
		host.VariantCaseValue(IpSocketAddressIPV6),
	)
}

func IpSocketAddressIPV4(addr IpV4SocketAddress) IpSocketAddress {
	return host.VariantConstructValue[IpSocketAddress](
		"ipv4",
		addr,
	)
}

func IpSocketAddressIPV6(addr IpV6SocketAddress) IpSocketAddress {
	return host.VariantConstructValue[IpSocketAddress](
		"ipv6",
		addr,
	)
}

func CreateNetworkInstance() *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("network", host.ResourceTypeFor[Network](hi, hi))
	hi.AddTypeExport("error-code", host.ValueTypeFor[NetworkErrorCode](hi))
	hi.AddTypeExport("ip-address-family", host.ValueTypeFor[IpAddressFamily](hi))
	hi.AddTypeExport("ipv4-address", host.ValueTypeFor[IpV4Address](hi))
	hi.AddTypeExport("ipv6-address", host.ValueTypeFor[IpV6Address](hi))
	hi.AddTypeExport("ip-address", host.ValueTypeFor[IpAddress](hi))
	hi.AddTypeExport("ipv4-socket-address", host.ValueTypeFor[IpV4SocketAddress](hi))
	hi.AddTypeExport("ipv6-socket-address", host.ValueTypeFor[IpV6SocketAddress](hi))
	hi.AddTypeExport("ip-socket-address", host.ValueTypeFor[IpSocketAddress](hi))

	return hi
}

func CreateInstanceNetworkInstance(
	networkInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("network", host.ResourceTypeFor[Network](hi, networkInstance))

	hi.MustAddFunction("instance-network", func() host.Own[Network] {
		return host.NewOwn(Network{})
	})

	return hi
}
