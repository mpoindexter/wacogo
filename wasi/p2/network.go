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

type IpV4Address host.Record[IpV4Address]

func NewIpV4Address(addr net.IP) IpV4Address {
	if len(addr) != net.IPv4len {
		addr = make([]byte, net.IPv4len)
	}
	return host.RecordConstruct[IpV4Address](
		host.RecordField("", addr[0]),
		host.RecordField("", addr[1]),
		host.RecordField("", addr[2]),
		host.RecordField("", addr[3]),
	)
}

func (a IpV4Address) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[IpV4Address](
		inst,
		NewIpV4Address,
	)
}

func (a IpV4Address) ToNetIP() net.IP {
	return net.IPv4(
		host.RecordFieldGetIndex[uint8](a, 0),
		host.RecordFieldGetIndex[uint8](a, 1),
		host.RecordFieldGetIndex[uint8](a, 2),
		host.RecordFieldGetIndex[uint8](a, 3),
	)
}

type IpV6Address host.Record[IpV6Address]

func NewIpV6Address(addr net.IP) IpV6Address {
	if len(addr) != net.IPv6len {
		addr = make([]byte, net.IPv6len)
	}
	return host.RecordConstruct[IpV6Address](
		host.RecordField("", uint16(addr[0])<<8|uint16(addr[1])),
		host.RecordField("", uint16(addr[2])<<8|uint16(addr[3])),
		host.RecordField("", uint16(addr[4])<<8|uint16(addr[5])),
		host.RecordField("", uint16(addr[6])<<8|uint16(addr[7])),
		host.RecordField("", uint16(addr[8])<<8|uint16(addr[9])),
		host.RecordField("", uint16(addr[10])<<8|uint16(addr[11])),
		host.RecordField("", uint16(addr[12])<<8|uint16(addr[13])),
		host.RecordField("", uint16(addr[14])<<8|uint16(addr[15])),
	)
}

func (a IpV6Address) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[IpV6Address](
		inst,
		NewIpV6Address,
	)
}

func (a IpV6Address) ToNetIP() net.IP {
	return net.IP{
		byte(host.RecordFieldGetIndex[uint16](a, 0) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 0) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 1) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 1) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 2) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 2) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 3) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 3) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 4) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 4) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 5) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 5) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 6) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 6) & 0xff),
		byte(host.RecordFieldGetIndex[uint16](a, 7) >> 8),
		byte(host.RecordFieldGetIndex[uint16](a, 7) & 0xff),
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

type IpV4SocketAddress host.Record[IpV4SocketAddress]

func NewIpV4SocketAddress(port uint16, address IpV4Address) IpV4SocketAddress {
	return host.RecordConstruct[IpV4SocketAddress](
		host.RecordField("port", port),
		host.RecordField("address", address),
	)
}

func (a IpV4SocketAddress) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[IpV4SocketAddress](
		inst,
		NewIpV4SocketAddress,
	)
}

func (a IpV4SocketAddress) Port() uint16 {
	return host.RecordFieldGetIndex[uint16](a, 0)
}

func (a IpV4SocketAddress) Address() IpV4Address {
	return host.RecordFieldGetIndex[IpV4Address](a, 1)
}

type IpV6SocketAddress host.Record[IpV6SocketAddress]

func NewIpV6SocketAddress(port uint16, flowInfo uint32, address IpV6Address, scopeID uint32) IpV6SocketAddress {
	return host.RecordConstruct[IpV6SocketAddress](
		host.RecordField("port", port),
		host.RecordField("flow-info", flowInfo),
		host.RecordField("address", address),
		host.RecordField("scope-id", scopeID),
	)
}

func (a IpV6SocketAddress) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[IpV6SocketAddress](
		inst,
		NewIpV6SocketAddress,
	)
}

func (a IpV6SocketAddress) Port() uint16 {
	return host.RecordFieldGetIndex[uint16](a, 0)
}

func (a IpV6SocketAddress) FlowInfo() uint32 {
	return host.RecordFieldGetIndex[uint32](a, 1)
}

func (a IpV6SocketAddress) Address() IpV6Address {
	return host.RecordFieldGetIndex[IpV6Address](a, 2)
}

func (a IpV6SocketAddress) ScopeID() uint32 {
	return host.RecordFieldGetIndex[uint32](a, 3)
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
