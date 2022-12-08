package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubeshop/testkube/pkg/api/v1/testkube"
	"github.com/kubeshop/testkube/pkg/executor"
	"github.com/kubeshop/testkube/pkg/executor/output"
	"github.com/kubeshop/testkube/pkg/executor/secret"
)

type Params struct {
	Datadir string // RUNNER_DATADIR
	Zaphome string // ZAP_HOME
}

func NewRunner() *ZapRunner {
	return &ZapRunner{
		params: Params{
			Datadir: os.Getenv("RUNNER_DATADIR"),
			Zaphome: os.Getenv("ZAP_HOME"),
		},
	}
}

type ZapRunner struct {
	params Params
}

func (r *ZapRunner) Run(execution testkube.Execution) (result testkube.ExecutionResult, err error) {
	// check that the datadir exists
	_, err = os.Stat(r.params.Datadir)
	if errors.Is(err, os.ErrNotExist) {
		return result, err
	}

	var directory string
	var zapConfig string

	if execution.Content.IsFile() {
		// use the given file config as ZAP config YAML
		directory = r.params.Datadir
		zapConfig = filepath.Join(r.params.Datadir, "test-content")
	} else if len(execution.Args) > 0 {
		// assume the ZAP config YAML has been passed as test argument
		directory = filepath.Join(r.params.Datadir, "repo")
		zapConfig = filepath.Join(directory, execution.Args[len(execution.Args)-1])
	}

	options := Options{}
	err = options.UnmarshalYAML(zapConfig)
	if err != nil {
		return result.WithErrors(err), nil
	}

	// determine the actual ZAP script and args to run
	scanType := strings.Split(execution.TestType, "/")[1]
	reportFile := fmt.Sprintf("%s-report.html", execution.TestName)
	scriptName := zapScript(scanType)
	args := zapArgs(scanType, options, reportFile)

	envManager := secret.NewEnvManagerWithVars(execution.Variables)
	envManager.GetVars(execution.Variables)
	// simply set the ENVs to use during Maven execution
	for _, env := range execution.Variables {
		os.Setenv(env.Name, env.Value)
	}

	// convert executor env variables to runner env variables
	for key, value := range execution.Envs {
		os.Setenv(key, value)
	}

	// when using file based ZAP parameters it expects a /zap/wrk directory
	// we simply symlink the directory
	os.Symlink(directory, filepath.Join(r.params.Zaphome, "wrk"))

	output.PrintEvent("Running", r.params.Zaphome, scriptName, args)
	output, err := executor.Run(r.params.Zaphome, scriptName, envManager, args...)
	output = envManager.Obfuscate(output)

	if err == nil {
		result.Status = testkube.ExecutionStatusPassed
	} else {
		result.Status = testkube.ExecutionStatusFailed
		result.ErrorMessage = err.Error()
		if strings.Contains(result.ErrorMessage, "exit status 1") || strings.Contains(result.ErrorMessage, "exit status 2") {
			result.ErrorMessage = "security issues found during scan"
		} else {
			// ZAP was unable to run at all, wrong args?
			return result, nil
		}
	}

	result.Output = string(output)
	result.OutputType = "text/plain"

	// prepare step results based on output
	result.Steps = []testkube.ExecutionStepResult{}
	lines := strings.Split(result.Output, "\n")
	for _, line := range lines {
		if strings.Index(line, "PASS") == 0 || strings.Index(line, "INFO") == 0 {
			result.Steps = append(result.Steps, testkube.ExecutionStepResult{
				Name: stepName(line),
				// always success
				Status: string(testkube.PASSED_ExecutionStatus),
			})
		} else if strings.Index(line, "WARN") == 0 {
			result.Steps = append(result.Steps, testkube.ExecutionStepResult{
				Name: stepName(line),
				// depends on the options if WARN will fail or not
				Status: warnStatus(scanType, options),
			})
		} else if strings.Index(line, "FAIL") == 0 {
			result.Steps = append(result.Steps, testkube.ExecutionStepResult{
				Name: stepName(line),
				// always error
				Status: string(testkube.FAILED_ExecutionStatus),
			})
		}
	}

	// TODO maybe upload the report file as artifact

	return result, err
}

const API = "api"
const BASELINE = "baseline"
const FULL = "full"

func zapScript(scanType string) string {
	switch {
	case scanType == BASELINE:
		return "./zap-baseline.py"
	default:
		return fmt.Sprintf("./zap-%s-scan.py", scanType)
	}
}

func zapArgs(scanType string, options Options, reportFile string) (args []string) {
	switch {
	case scanType == API:
		args = options.ToApiScanArgs(reportFile)
	case scanType == BASELINE:
		args = options.ToBaselineScanArgs(reportFile)
	case scanType == FULL:
		args = options.ToFullScanArgs(reportFile)
	}
	return args
}

func stepName(line string) string {
	return strings.TrimSpace(strings.SplitAfter(line, ":")[1])
}

func warnStatus(scanType string, options Options) string {
	var fail bool

	switch {
	case scanType == API:
		fail = options.API.FailOnWarn
	case scanType == BASELINE:
		fail = options.Baseline.FailOnWarn
	case scanType == FULL:
		fail = options.Full.FailOnWarn
	}

	if fail {
		return string(testkube.FAILED_ExecutionStatus)
	} else {
		return string(testkube.PASSED_ExecutionStatus)
	}
}
