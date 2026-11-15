package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"github.com/seifreed/tupinambacli/internal/browserauth"
)

var version = "dev"
var exit = os.Exit
var newClient = newConfiguredClient
var cookiesFromBrowser = browserauth.CookiesFromBrowser

func main() { exit(run(os.Args[1:])) }

func run(args []string) (exitCode int) {
	logger, closeLogs, loggerErr := configuredLogger(os.Stderr)
	defer closeLogs()
	if correlationID, err := newCorrelationID(); err == nil {
		logger = logger.With("correlation_id", correlationID)
	}
	slog.SetDefault(logger)
	defer func() {
		if recovered := recover(); recovered != nil {
			command := ""
			if len(args) > 0 {
				command = args[0]
			}
			logger.Error("unhandled command panic", "component", "cli", "operation", "cli_command", "event_name", "command.panic", "command", command, "outcome", "fatal", "error_type", fmt.Sprintf("%T", recovered), "stack_trace", string(debug.Stack()))
			exitCode = 2
		}
	}()
	if loggerErr != nil {
		logger.Warn("log destination unavailable; using stderr", "component", "cli", "operation", "logger_init", "event_name", "logging.destination.fallback", "error_type", fmt.Sprintf("%T", loggerErr))
	}
	if len(args) == 0 {
		logger.Warn("command missing", "component", "cli", "operation", "command_validation", "event_name", "command.missing", "outcome", "usage")
		usage()
		return 2
	}
	started := time.Now()
	command := args[0]
	var err error
	switch args[0] {
	case "search":
		err = cmdSearch(args[1:])
	case "product":
		err = cmdProduct(args[1:])
	case "batch":
		err = cmdBatch(args[1:])
	case "total":
		err = cmdTotal(args[1:])
	case "collections", "categories":
		err = cmdCollections(args[1:])
	case "login":
		err = cmdLogin(args[1:])
	case "set-cookie":
		err = cmdSetCookie(args[1:])
	case "import-har":
		err = cmdImportHAR(args[1:])
	case "whoami":
		err = cmdWhoami(args[1:])
	case "cart":
		err = cmdCart(args[1:])
	case "checkout":
		err = cmdCheckout(args[1:])
	case "mcp":
		err = cmdMCP(args[1:])
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		logger.Warn("unknown command", "component", "cli", "operation", "command_validation", "event_name", "command.unknown", "outcome", "usage")
		fmt.Fprintln(os.Stderr, "unknown command")
		usage()
		return 2
	}
	if err != nil {
		logger.Error("command failed", "component", "cli", "operation", "cli_command", "event_name", "command.failed", "command", command, "duration_ms", time.Since(started).Milliseconds(), "error_type", fmt.Sprintf("%T", err))
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	logger.Info("command completed", "component", "cli", "operation", "cli_command", "event_name", "command.completed", "command", command, "duration_ms", time.Since(started).Milliseconds(), "outcome", "success")
	return 0
}
