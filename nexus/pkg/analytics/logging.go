package analytics

import "golang.org/x/exp/slog"

type NexusLogger struct {
	Logger *slog.Logger
}

func NewNexusLogger(logger *slog.Logger) *NexusLogger {
	return &NexusLogger{Logger: logger}
}

func limitLength(s string) string {
	// sentry has a limit of 200 characters for tag values
	maxLen := 197
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// tagsFromArgs constructs a map of tags from the args
func tagsFromArgs(args ...interface{}) map[string]string {
	tags := make(map[string]string)
	for i := 0; i < len(args); i += 2 {
		// skip "err":
		if args[i] == "err" {
			continue
		}
		key := args[i].(string)
		value, ok := args[i+1].(string)
		if ok {
			tags[key] = limitLength(value)
		}
	}
	return tags
}

// errFromArgs returns the first error found in the args
func errFromArgs(args ...interface{}) error {
	var err error
	for i := 0; i < len(args)-1; i++ {
		// check if the current argument is "err" and the next one is of type error
		if argStr, ok := args[i].(string); ok && argStr == "err" {
			if errVal, ok := args[i+1].(error); ok {
				err = errVal
				return err
			}
		}
	}
	return nil
}

func (nl *NexusLogger) Debug(msg string, args ...interface{}) {
	nl.Logger.Debug(msg, args...)
}

func (nl *NexusLogger) Error(msg string, args ...interface{}) {
	nl.Logger.Error(msg, args...)
	// convert args to tags to pass to sentry:
	tags := tagsFromArgs(args...)
	// look for an error in the args:
	err := errFromArgs(args...)
	if err != nil {
		// send error to sentry:
		CaptureException(err, tags)
	}
}

func (nl *NexusLogger) Info(msg string, args ...interface{}) {
	nl.Logger.Info(msg, args...)
}

func (nl *NexusLogger) Warn(msg string, args ...interface{}) {
	nl.Logger.Warn(msg, args...)

	tags := tagsFromArgs(args...)
	// send message to sentry:
	CaptureMessage(msg, tags)
}
