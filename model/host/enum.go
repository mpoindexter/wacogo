package host

type Enum[T EnumImpl] string

type EnumImpl interface {
	~string
	EnumValueProvider
}

type EnumValueProvider interface {
	EnumValues() []string
}
