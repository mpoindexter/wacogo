package host

type Flags[T FlagsImpl] map[string]bool

type FlagsImpl interface {
	~map[string]bool
	FlagsValueProvider
}

type FlagsValueProvider interface {
	FlagsValues() []string
}
