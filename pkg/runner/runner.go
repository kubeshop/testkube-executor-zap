package runner

import (
	"github.com/kubeshop/testkube/pkg/api/v1/testkube"
	"github.com/kubeshop/testkube/pkg/executor/output"
)

func NewRunner() *ExampleRunner {
	return &ExampleRunner{}
}

// ExampleRunner for template - change me to some valid runner
type ExampleRunner struct {
}

func (r *ExampleRunner) Run(execution testkube.Execution) (result testkube.ExecutionResult, err error) {

	if execution.Content.IsFile() {
		output.PrintEvent("using file", execution)
		// TODO implement file based test content for string, git-file, file-uri
		//      or remove if not used
	}

	if execution.Content.IsDir() {
		output.PrintEvent("using dir", execution)
		// TODO implement file based test content for git-dir
		//      or remove if not used
	}

	// TODO run executor here

	// error result should be returned if something is not ok
	// return result.Err(fmt.Errorf("some test execution related error occured"))

	// TODO return ExecutionResult
	return testkube.ExecutionResult{
		Status: testkube.StatusPtr(testkube.PASSED_ExecutionStatus),
		Output: "exmaple test output",
	}, nil
}
