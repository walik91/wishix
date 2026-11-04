package main

import (
	"context"
	"io"
	"testing"
)

func TestCommandArguments(t *testing.T) {
	for _, name := range []string{"up", "up-by-one", "down", "redo", "reset", "status", "version"} {
		if _, err := parseCommand([]string{name}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, args := range [][]string{{"up-to", "1"}, {"down-to", "1"}, {"down-to", "0"}} {
		if _, err := parseCommand(args); err != nil {
			t.Error(err)
		}
	}
	for _, args := range [][]string{nil, {"unknown"}, {"up", "extra"}, {"down", "1"}, {"up-to"}, {"down-to", "-1"}, {"up-to", "0"}, {"up-to", "text"}, {"up-to", "9223372036854775808"}} {
		if _, err := parseCommand(args); err == nil {
			t.Errorf("invalid command accepted: %v", args)
		}
	}
}

func TestInvalidCommandDoesNotConnectOrMigrate(t *testing.T) {
	t.Setenv("DB_NAME", "")
	err := run(context.Background(), []string{"down", "unexpected"}, io.Discard)
	if err == nil || err.Error() != usage {
		t.Fatal("CLI must reject arguments before inspecting DB settings")
	}
}
