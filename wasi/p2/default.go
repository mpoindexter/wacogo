package p2

import (
	"io"

	"github.com/partite-ai/wacogo/componentmodel"
)

func CreateStandardWASIInstances(
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	args []string,
	environment []*EnvVar,
	initialCwd string,
) (map[string]*componentmodel.Instance, error) {
	instances := make(map[string]*componentmodel.Instance)

	cliInstance := CreateEnvironmentInstance(environment, args, initialCwd)
	instances["wasi:cli/environment@0.2.0"] = cliInstance.Instance()

	errorInstance := CreateErrorInstance()
	instances["wasi:io/error@0.2.0"] = errorInstance.Instance()

	pollInstance := CreatePollInstance()
	instances["wasi:io/poll@0.2.0"] = pollInstance.Instance()

	streamsInstance := CreateStreamsInstance(
		errorInstance,
		pollInstance,
	)
	instances["wasi:io/streams@0.2.0"] = streamsInstance.Instance()

	stdinInstance := CreateStdinInstance(stdin, streamsInstance)
	instances["wasi:cli/stdin@0.2.0"] = stdinInstance.Instance()

	stdoutInstance := CreateStdoutInstance(stdout, streamsInstance)
	instances["wasi:cli/stdout@0.2.0"] = stdoutInstance.Instance()

	stderrInstance := CreateStderrInstance(stderr, streamsInstance)
	instances["wasi:cli/stderr@0.2.0"] = stderrInstance.Instance()

	randomInstance := CreateRandomInstance()
	instances["wasi:random/random@0.2.0"] = randomInstance.Instance()

	insecureRandomInstance := CreateInsecureRandomInstance()
	instances["wasi:random/insecure-random@0.2.0"] = insecureRandomInstance.Instance()

	monotonicClockInstance := CreateMonotonicClockInstance(pollInstance)
	instances["wasi:clocks/monotonic-clock@0.2.0"] = monotonicClockInstance.Instance()

	wallClockInstance := CreateWallClockInstance()
	instances["wasi:clocks/wall-clock@0.2.0"] = wallClockInstance.Instance()

	fsTypes := CreateFilesystemTypesInstance(
		streamsInstance,
		errorInstance,
	)
	instances["wasi:filesystem/types@0.2.0"] = fsTypes.Instance()

	preopens := CreateFilesystemPreopensInstance(fsTypes)
	instances["wasi:filesystem/preopens@0.2.0"] = preopens.Instance()

	terminalInput := CreateTerminalInputInstance()
	instances["wasi:cli/terminal-input@0.2.0"] = terminalInput.Instance()

	terminalOutput := CreateTerminalOutputInstance()
	instances["wasi:cli/terminal-output@0.2.0"] = terminalOutput.Instance()

	terminalStdin := CreateTerminalStdinInstance(terminalInput)
	instances["wasi:cli/terminal-stdin@0.2.0"] = terminalStdin.Instance()

	terminalStdout := CreateTerminalStdoutInstance(terminalOutput)
	instances["wasi:cli/terminal-stdout@0.2.0"] = terminalStdout.Instance()

	terminalStderr := CreateTerminalStderrInstance(terminalOutput)
	instances["wasi:cli/terminal-stderr@0.2.0"] = terminalStderr.Instance()

	return instances, nil
}
