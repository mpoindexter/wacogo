package p2

import (
	"io"

	"github.com/partite-ai/wacogo/componentmodel"
	"github.com/partite-ai/wacogo/componentmodel/host"
)

type EnvVar struct {
	Key   string
	Value string
}

func CreateEnvironmentInstance(
	vars []*EnvVar,
	args []string,
	initialCwd string,
) *host.Instance {
	hi := host.NewInstance()

	hi.MustAddFunction("get-environment", func() []Tuple2[componentmodel.String, componentmodel.String] {
		tuples := make([]Tuple2[componentmodel.String, componentmodel.String], len(vars))
		for i, v := range vars {
			tuples[i] = NewTuple2(componentmodel.String(v.Key), componentmodel.String(v.Value))
		}
		return tuples
	})
	hi.MustAddFunction("get-arguments", func() []componentmodel.String {
		modelArgs := make([]componentmodel.String, len(args))
		for i, arg := range args {
			modelArgs[i] = componentmodel.String(arg)
		}
		return modelArgs
	})
	hi.MustAddFunction("initial-cwd", func() Option[string] {
		if initialCwd == "" {
			return OptionNone[string]()
		}
		return OptionSome(initialCwd)
	})
	return hi
}

func CreateStdoutInstance(
	w io.Writer,
	streamsInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()

	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, streamsInstance))
	hi.MustAddFunction("get-stdout", func() host.Own[OutputStream] {
		return host.NewOwn(OutputStream(NewWriterOutputStream(w)))
	})
	return hi
}

func CreateStderrInstance(
	w io.Writer,
	streamsInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, streamsInstance))

	hi.MustAddFunction("get-stderr", func() host.Own[OutputStream] {
		return host.NewOwn(OutputStream(NewWriterOutputStream(w)))
	})
	return hi
}

func CreateStdinInstance(
	in io.Reader,
	streamsInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("input-stream", host.ResourceTypeFor[InputStream](hi, streamsInstance))

	hi.MustAddFunction("get-stdin", func() host.Own[InputStream] {
		return host.NewOwn[InputStream](NewReaderInputStream(in, 32768, 1024, 32))
	})

	return hi
}

type TerminalInput struct{}

func CreateTerminalInputInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("terminal-input", host.ResourceTypeFor[TerminalInput](hi, hi))
	return hi
}

type TerminalOutput struct{}

func CreateTerminalOutputInstance() *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("terminal-output", host.ResourceTypeFor[TerminalOutput](hi, hi))
	return hi
}

func CreateTerminalStdinInstance(
	terminalInputInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("terminal-input", host.ResourceTypeFor[TerminalInput](hi, terminalInputInstance))

	hi.MustAddFunction("get-terminal-stdin", func() Option[host.Own[TerminalInput]] {
		return OptionNone[host.Own[TerminalInput]]()
	})
	return hi
}

func CreateTerminalStdoutInstance(
	terminalOutputInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("terminal-output", host.ResourceTypeFor[TerminalOutput](hi, terminalOutputInstance))

	hi.MustAddFunction("get-terminal-stdout", func() Option[host.Own[TerminalOutput]] {
		return OptionNone[host.Own[TerminalOutput]]()
	})
	return hi
}

func CreateTerminalStderrInstance(
	terminalOutputInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("terminal-output", host.ResourceTypeFor[TerminalOutput](hi, terminalOutputInstance))

	hi.MustAddFunction("get-terminal-stderr", func() Option[host.Own[TerminalOutput]] {
		return OptionNone[host.Own[TerminalOutput]]()
	})
	return hi
}
