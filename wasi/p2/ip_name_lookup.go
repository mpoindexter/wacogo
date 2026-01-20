package p2

import (
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type ResolveAddressStream struct {
}

func CreateIpNameLookupInstance(
	networkInstance *host.Instance,
	pollInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("resolve-address-stream", host.ResourceTypeFor[ResolveAddressStream](hi, hi))
	hi.AddTypeExport("network", host.ResourceTypeFor[Network](hi, networkInstance))
	hi.AddTypeExport("error-code", host.ValueTypeFor[NetworkErrorCode](hi))
	hi.AddTypeExport("ip-address", host.ValueTypeFor[IpAddress](hi))
	hi.AddTypeExport("pollable", host.ResourceTypeFor[Pollable](hi, pollInstance))

	hi.MustAddFunction("resolve-addresses", func(
		network host.Borrow[Network],
		name string,
	) Result[host.Own[ResolveAddressStream], NetworkErrorCode] {
		return ResultErr[host.Own[ResolveAddressStream]](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]resolve-address-stream.resolve-next-address", func(
		self host.Borrow[ResolveAddressStream],
	) Result[Option[IpAddress], NetworkErrorCode] {
		return ResultErr[Option[IpAddress]](NetworkErrorCode("not-supported"))
	})

	hi.MustAddFunction("[method]resolve-address-stream.subscribe", func(
		self host.Borrow[ResolveAddressStream],
	) host.Own[Pollable] {
		return host.NewOwn(Pollable(AlwaysReadyPollable{}))
	})

	return hi
}
