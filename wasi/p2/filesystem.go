package p2

import (
	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type Descriptor struct {
}

type DirectoryEntryStream struct {
}

type Filesize uint64
type LinkCount uint64

type ErrorCode host.Enum[ErrorCode]

func (ErrorCode) EnumValues() []string {
	return []string{
		"access",
		"would-block",
		"already",
		"bad-descriptor",
		"busy",
		"deadlock",
		"quota",
		"exist",
		"file-too-large",
		"illegal-byte-sequence",
		"in-progress",
		"interrupted",
		"invalid",
		"io",
		"is-directory",
		"loop",
		"too-many-links",
		"message-size",
		"name-too-long",
		"no-device",
		"no-entry",
		"no-lock",
		"insufficient-memory",
		"insufficient-space",
		"not-directory",
		"not-empty",
		"not-recoverable",
		"unsupported",
		"no-tty",
		"no-such-device",
		"overflow",
		"not-permitted",
		"pipe",
		"read-only",
		"invalid-seek",
		"text-file-busy",
		"cross-device",
	}
}

type Advice host.Enum[Advice]

func (Advice) EnumValues() []string {
	return []string{
		"normal",
		"sequential",
		"random",
		"will-need",
		"dont-need",
		"no-reuse",
	}
}

type DescriptorFlags host.Flags[DescriptorFlags]

func (DescriptorFlags) FlagsValues() []string {
	return []string{
		"read",
		"write",
		"file-integrity-sync",
		"data-integrity-sync",
		"requested-write-sync",
		"mutate-directory",
	}
}

type DescriptorType host.Enum[DescriptorType]

func (DescriptorType) EnumValues() []string {
	return []string{
		"unknown",
		"block-device",
		"character-device",
		"directory",
		"fifo",
		"symbolic-link",
		"regular-file",
		"socket",
	}
}

type NewTimestamp host.Variant[NewTimestamp]

func (NewTimestamp) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.VariantType(
		inst,
		host.VariantCase(NewTimestampNoChange),
		host.VariantCase(NewTimestampNow),
		host.VariantCaseValue(NewTimestampTimestamp),
	)
}

func NewTimestampNoChange() NewTimestamp {
	return host.VariantConstruct[NewTimestamp](
		"no-change",
	)
}

func (v NewTimestamp) NoChange() bool {
	return host.VariantTest(v, "no-change")
}

func NewTimestampNow() NewTimestamp {
	return host.VariantConstruct[NewTimestamp](
		"now",
	)
}

func (v NewTimestamp) Now() bool {
	return host.VariantTest(v, "now")
}

func NewTimestampTimestamp(timestamp DateTime) NewTimestamp {
	return host.VariantConstructValue[NewTimestamp](
		"timestamp",
		timestamp,
	)
}

func (v NewTimestamp) Timestamp() (DateTime, bool) {
	return host.VariantCast[DateTime](v, "timestamp")
}

type DescriptorStat host.Record[DescriptorStat]

func NewDescriptorStat(
	typ DescriptorType,
	linkCount LinkCount,
	size Filesize,
	dataAccessTimestamp Option[DateTime],
	dataModificationTimestamp Option[DateTime],
	statusChangeTimestamp Option[DateTime],
) DescriptorStat {
	return host.RecordConstruct[DescriptorStat](
		host.RecordField("type", typ),
		host.RecordField("link-count", linkCount),
		host.RecordField("size", size),
		host.RecordField("data-access-timestamp", dataAccessTimestamp),
		host.RecordField("data-modification-timestamp", dataModificationTimestamp),
		host.RecordField("status-change-timestamp", statusChangeTimestamp),
	)
}

func (ds DescriptorStat) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[DescriptorStat](
		inst,
		NewDescriptorStat,
	)
}

func (ds DescriptorStat) Type() DescriptorType {
	return host.RecordFieldGetIndex[DescriptorType](ds, 0)
}

func (ds DescriptorStat) LinkCount() LinkCount {
	return host.RecordFieldGetIndex[LinkCount](ds, 1)
}

func (ds DescriptorStat) Size() Filesize {
	return host.RecordFieldGetIndex[Filesize](ds, 2)
}

func (ds DescriptorStat) DataAccessTimestamp() DateTime {
	return host.RecordFieldGetIndex[DateTime](ds, 3)
}

func (ds DescriptorStat) DataModificationTimestamp() DateTime {
	return host.RecordFieldGetIndex[DateTime](ds, 4)
}

func (ds DescriptorStat) StatusChangeTimestamp() DateTime {
	return host.RecordFieldGetIndex[DateTime](ds, 5)
}

type PathFlags host.Flags[PathFlags]

func (PathFlags) FlagsValues() []string {
	return []string{
		"symlink-follow",
	}
}

type OpenFlags host.Flags[OpenFlags]

func (OpenFlags) FlagsValues() []string {
	return []string{
		"create",
		"directory",
		"exclusive",
		"truncate",
	}
}

type DirectoryEntry host.Record[DirectoryEntry]

func NewDirectoryEntry(entryType DescriptorType, name string) DirectoryEntry {
	return host.RecordConstruct[DirectoryEntry](
		host.RecordField("type", entryType),
		host.RecordField("name", name),
	)
}
func (de DirectoryEntry) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[DirectoryEntry](
		inst,
		NewDirectoryEntry,
	)
}

type MetadataHashValue host.Record[MetadataHashValue]

func NewMetadataHashValue(lower componentmodel.U64, upper componentmodel.U64) MetadataHashValue {
	return host.RecordConstruct[MetadataHashValue](
		host.RecordField("lower", lower),
		host.RecordField("upper", upper),
	)
}

func (mhv MetadataHashValue) ValueType(inst *host.Instance) componentmodel.ValueType {
	return host.RecordType[MetadataHashValue](
		inst,
		NewMetadataHashValue,
	)
}

func CreateFilesystemTypesInstance(
	streamsInstance *host.Instance,
	errorInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("input-stream", host.ResourceTypeFor[InputStream](hi, streamsInstance))
	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, streamsInstance))
	hi.AddTypeExport("error", host.ResourceTypeFor[IOError](hi, errorInstance))
	hi.AddTypeExport("datetime", host.ValueTypeFor[DateTime](hi))
	hi.AddTypeExport("filesize", host.ValueTypeFor[Filesize](hi))
	hi.AddTypeExport("descriptor-flags", host.ValueTypeFor[DescriptorFlags](hi))
	hi.AddTypeExport("link-count", host.ValueTypeFor[LinkCount](hi))
	hi.AddTypeExport("descriptor-stat", host.ValueTypeFor[DescriptorStat](hi))
	hi.AddTypeExport("descriptor-type", host.ValueTypeFor[DescriptorType](hi))
	hi.AddTypeExport("path-flags", host.ValueTypeFor[PathFlags](hi))
	hi.AddTypeExport("open-flags", host.ValueTypeFor[OpenFlags](hi))
	hi.AddTypeExport("new-timestamp", host.ValueTypeFor[NewTimestamp](hi))
	hi.AddTypeExport("directory-entry", host.ValueTypeFor[DirectoryEntry](hi))
	hi.AddTypeExport("error-code", host.ValueTypeFor[ErrorCode](hi))
	hi.AddTypeExport("advice", host.ValueTypeFor[Advice](hi))
	hi.AddTypeExport("metadata-hash-value", host.ValueTypeFor[MetadataHashValue](hi))
	hi.AddTypeExport("descriptor", host.ResourceTypeFor[*Descriptor](hi, hi))
	hi.AddTypeExport("directory-entry-stream", host.ResourceTypeFor[*DirectoryEntryStream](hi, hi))

	/*
		hc.RegisterFunction("[method]descriptor.", func(self *Descriptor){

		})
	*/

	hi.AddFunction("[method]descriptor.read-via-stream", func(self host.Borrow[*Descriptor], offset Filesize) Result[host.Own[InputStream], ErrorCode] {
		return ResultErr[host.Own[InputStream]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.write-via-stream", func(self host.Borrow[*Descriptor], offset Filesize) Result[host.Own[OutputStream], ErrorCode] {
		return ResultErr[host.Own[OutputStream]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.append-via-stream", func(self host.Borrow[*Descriptor]) Result[host.Own[OutputStream], ErrorCode] {
		return ResultErr[host.Own[OutputStream]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.advise", func(self host.Borrow[*Descriptor], offset Filesize, length Filesize, advice Advice) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.sync-data", func(self host.Borrow[*Descriptor]) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.get-flags", func(self host.Borrow[*Descriptor]) Result[DescriptorFlags, ErrorCode] {
		return ResultErr[DescriptorFlags](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.get-type", func(self host.Borrow[*Descriptor]) Result[DescriptorType, ErrorCode] {
		return ResultErr[DescriptorType](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.set-size", func(self host.Borrow[*Descriptor], size Filesize) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.set-times", func(self host.Borrow[*Descriptor], dataAccessTimestamp NewTimestamp, dataModificationTimestamp NewTimestamp) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.read", func(self host.Borrow[*Descriptor], length componentmodel.U64, offset componentmodel.U64) Result[Tuple2[componentmodel.ByteArray, componentmodel.Bool], ErrorCode] {
		return ResultErr[Tuple2[componentmodel.ByteArray, componentmodel.Bool]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.write", func(self host.Borrow[*Descriptor], buffer componentmodel.ByteArray, offset componentmodel.U64) Result[componentmodel.U64, ErrorCode] {
		return ResultErr[componentmodel.U64](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.read-directory", func(self host.Borrow[*Descriptor]) Result[host.Own[*DirectoryEntryStream], ErrorCode] {
		return ResultErr[host.Own[*DirectoryEntryStream]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.sync", func(self host.Borrow[*Descriptor]) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.create-directory-at", func(self host.Borrow[*Descriptor], path componentmodel.String) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.stat", func(self host.Borrow[*Descriptor]) Result[DescriptorStat, ErrorCode] {
		return ResultErr[DescriptorStat](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.stat-at", func(self host.Borrow[*Descriptor], pathFlags PathFlags, path componentmodel.String) Result[DescriptorStat, ErrorCode] {
		return ResultErr[DescriptorStat](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.set-times-at", func(self host.Borrow[*Descriptor], pathFlags PathFlags, path componentmodel.String, dataAccessTimestamp NewTimestamp, dataModificationTimestamp NewTimestamp) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.link-at", func(self host.Borrow[*Descriptor], oldPathFlags PathFlags, oldPath componentmodel.String, newDescriptor host.Borrow[*Descriptor], newPath componentmodel.String) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.open-at", func(self host.Borrow[*Descriptor], pathFlags PathFlags, path componentmodel.String, openFlags OpenFlags, flags DescriptorFlags) Result[host.Own[*Descriptor], ErrorCode] {
		return ResultErr[host.Own[*Descriptor]](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.readlink-at", func(self host.Borrow[*Descriptor], path componentmodel.String) Result[componentmodel.String, ErrorCode] {
		return ResultErr[componentmodel.String](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.remove-directory-at", func(self host.Borrow[*Descriptor], path componentmodel.String) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.rename-at", func(self host.Borrow[*Descriptor], oldPath string, newDescriptor host.Borrow[*Descriptor], newPath string) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.symlink-at", func(self host.Borrow[*Descriptor], oldPath string, newPath string) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.unlink-file-at", func(self host.Borrow[*Descriptor], path string) Result[Void, ErrorCode] {
		return ResultErr[Void](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.is-same-object", func(self host.Borrow[*Descriptor], other host.Borrow[*Descriptor]) componentmodel.Bool {
		return self.Resource() == other.Resource()
	})

	hi.AddFunction("[method]descriptor.metadata-hash", func(self host.Borrow[*Descriptor]) Result[MetadataHashValue, ErrorCode] {
		return ResultErr[MetadataHashValue](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]descriptor.metadata-hash-at", func(self host.Borrow[*Descriptor], pathFlags PathFlags, path componentmodel.String) Result[MetadataHashValue, ErrorCode] {
		return ResultErr[MetadataHashValue](ErrorCode("unsupported"))
	})

	hi.AddFunction("[method]directory-entry-stream.read-directory-entry", func(self host.Borrow[*DirectoryEntryStream]) Result[Option[DirectoryEntry], ErrorCode] {
		return ResultErr[Option[DirectoryEntry]](ErrorCode("unsupported"))
	})

	hi.AddFunction("filesystem-error-code", func(err host.Borrow[IOError]) Option[ErrorCode] {
		return OptionNone[ErrorCode]()
	})

	return hi
}

func CreateFilesystemPreopensInstance(
	typesInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("descriptor", host.ResourceTypeFor[*Descriptor](hi, typesInstance))
	hi.AddFunction("get-directories", func() []Tuple2[host.Own[*Descriptor], string] {
		return nil
	})
	return hi
}
