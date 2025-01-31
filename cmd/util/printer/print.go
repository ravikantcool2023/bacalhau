package printer

import (
	"math"
	"os"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/mitchellh/go-wordwrap"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/term"

	"github.com/bacalhau-project/bacalhau/pkg/models"
	"github.com/bacalhau-project/bacalhau/pkg/util/idgen"
)

var (
	none  = color.New(color.Reset)
	red   = color.New(color.FgRed)
	green = color.New(color.FgGreen)
)

const (
	errorPrefix = "Error: "
	hintPrefix  = "Hint: "
)

var terminalWidth int

func getTerminalWidth(cmd *cobra.Command) uint {
	if terminalWidth == 0 {
		var err error
		terminalWidth, _, err = term.GetSize(int(os.Stderr.Fd()))
		if err != nil || terminalWidth <= 0 {
			log.Ctx(cmd.Context()).Debug().Err(err).Msg("Failed to get terminal size")
			terminalWidth = math.MaxInt8
		}
	}
	return uint(terminalWidth)
}

func PrintEvent(cmd *cobra.Command, event models.Event) {
	printIndentedString(cmd, errorPrefix, event.Message, red, 0)
	if event.Details != nil && event.Details[models.DetailsKeyHint] != "" {
		printIndentedString(cmd, hintPrefix, event.Details[models.DetailsKeyHint], green, uint(len(errorPrefix)))
	}
}

func PrintError(cmd *cobra.Command, err error) {
	printIndentedString(cmd, errorPrefix, err.Error(), red, 0)
}

// Groups the executions in the job state, returning a map of printable messages
// to node(s) that generated that message.
func SummariseExecutions(executions []*models.Execution) map[string][]string {
	results := make(map[string][]string, len(executions))
	for _, execution := range executions {
		var message string
		if execution.RunOutput != nil {
			if execution.RunOutput.ErrorMsg != "" {
				message = execution.RunOutput.ErrorMsg
			} else if execution.RunOutput.ExitCode > 0 {
				message = execution.RunOutput.STDERR
			} else {
				message = execution.RunOutput.STDOUT
			}
		} else if execution.IsDiscarded() {
			message = execution.ComputeState.Message
		}

		if message != "" {
			results[message] = append(results[message], idgen.ShortNodeID(execution.NodeID))
		}
	}
	return results
}

func SummariseHistoryEvents(history []*models.JobHistory) []models.Event {
	slices.SortFunc(history, func(a, b *models.JobHistory) int {
		return a.Occurred().Compare(b.Occurred())
	})

	events := make(map[string]models.Event, len(history))
	for _, entry := range history {
		hasDetails := entry.Event.Details != nil
		failsExecution := hasDetails && entry.Event.Details[models.DetailsKeyFailsExecution] == "true"
		if failsExecution && entry.Event.Message != "" {
			events[entry.Event.Message] = entry.Event
		}
	}

	return maps.Values(events)
}

func printIndentedString(cmd *cobra.Command, prefix, msg string, prefixColor *color.Color, startIndent uint) {
	maxWidth := getTerminalWidth(cmd)
	blockIndent := int(startIndent) + len(prefix)
	blockTextWidth := maxWidth - startIndent - uint(len(prefix))

	cmd.PrintErrln()
	cmd.PrintErr(strings.Repeat(" ", int(startIndent)))
	prefixColor.Fprintf(cmd.ErrOrStderr(), "%s", prefix)
	for i, line := range strings.Split(wordwrap.WrapString(msg, blockTextWidth), "\n") {
		if i > 0 {
			cmd.PrintErr(strings.Repeat(" ", blockIndent))
		}
		cmd.PrintErrln(line)
	}
}
