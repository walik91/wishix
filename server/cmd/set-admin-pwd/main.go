// set-admin-pwd reads a password from stdin and updates only ADMIN_PASSWORD_HASH.
package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"wishix/server/internal/auth"
)

func main() {
	if err := run(os.Args[1:], os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string, in io.Reader) error {
	if len(args) != 1 {
		return errors.New("usage: set-admin-pwd /path/to/.env (password on stdin)")
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	password := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if !utf8.ValidString(password) {
		return errors.New("password must be valid UTF-8")
	}
	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("cannot read existing .env file")
	}
	contents := string(data)
	newline := "\n"
	if strings.Contains(contents, "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	found := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ADMIN_PASSWORD_HASH=") || strings.HasPrefix(trimmed, "export ADMIN_PASSWORD_HASH=") {
			if found >= 0 {
				return errors.New("duplicate ADMIN_PASSWORD_HASH entries")
			}
			found = i
		}
	}
	entry := "ADMIN_PASSWORD_HASH=" + base64.RawURLEncoding.EncodeToString([]byte(auth.HashPassword(password)))
	if found >= 0 {
		lines[found] = entry
	} else {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, entry, "")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".admin-env-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(info.Mode().Perm()); err == nil {
		_, err = tmp.WriteString(strings.Join(lines, newline))
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	fmt.Println("Admin password hash updated. Recreate the server to apply it.")
	return nil
}
