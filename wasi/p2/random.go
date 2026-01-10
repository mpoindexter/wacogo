package p2

import (
	"crypto/rand"
	"encoding/binary"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

func CreateRandomInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddFunction("get-random-bytes", func(n componentmodel.U64) componentmodel.ByteArray {
		buf := make([]byte, n)
		_, err := rand.Read(buf)
		if err != nil {
			panic("failed to read random bytes: " + err.Error())
		}
		return componentmodel.ByteArray(buf)
	})
	hi.AddFunction("get-random-u64", func() componentmodel.U64 {
		var bytes [8]byte
		_, err := rand.Read(bytes[:])
		if err != nil {
			panic("failed to read random u64: " + err.Error())
		}
		return componentmodel.U64(binary.LittleEndian.Uint64(bytes[:]))
	})
	return hi
}

func CreateInsecureRandomInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddFunction("get-insecure-random-bytes", func(n componentmodel.U64) componentmodel.ByteArray {
		buf := make([]byte, n)
		_, err := rand.Read(buf)
		if err != nil {
			panic("failed to read random bytes: " + err.Error())
		}
		return componentmodel.ByteArray(buf)
	})
	hi.AddFunction("get-insecure-random-u64", func() componentmodel.U64 {
		var bytes [8]byte
		_, err := rand.Read(bytes[:])
		if err != nil {
			panic("failed to read random u64: " + err.Error())
		}
		return componentmodel.U64(binary.LittleEndian.Uint64(bytes[:]))
	})
	return hi
}
