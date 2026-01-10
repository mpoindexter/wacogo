package p2

import (
	"io"

	"github.com/partite-ai/wacogo/model"
	"github.com/partite-ai/wacogo/model/host"
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

	hi.MustAddFunction("get-environment", func() []Tuple2[model.String, model.String] {
		tuples := make([]Tuple2[model.String, model.String], len(vars))
		for i, v := range vars {
			tuples[i] = NewTuple2(model.String(v.Key), model.String(v.Value))
		}
		return tuples
	})
	hi.MustAddFunction("get-arguments", func() []model.String {
		modelArgs := make([]model.String, len(args))
		for i, arg := range args {
			modelArgs[i] = model.String(arg)
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
	hi.MustAddFunction("get-stdout", func() model.Own[OutputStream] {
		return model.Own[OutputStream]{Resource: OutputStream{w: w}}
	})
	return hi
}

func CreateStderrInstance(
	w io.Writer,
	streamsInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("output-stream", host.ResourceTypeFor[OutputStream](hi, streamsInstance))

	hi.MustAddFunction("get-stderr", func() model.Own[OutputStream] {
		return model.Own[OutputStream]{Resource: OutputStream{w: w}}
	})
	return hi
}

func CreateStdinInstance(
	in io.Reader,
	streamsInstance *host.Instance,
) *host.Instance {
	hi := host.NewInstance()
	hi.AddTypeExport("input-stream", host.ResourceTypeFor[InputStream](hi, streamsInstance))

	hi.MustAddFunction("get-stdin", func() model.Own[InputStream] {
		return model.Own[InputStream]{Resource: InputStream{r: in}}
	})

	return hi
}
