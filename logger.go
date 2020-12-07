package main

import "os"

// PluginLogger prefixes the log output with 'plugin-lntxbot'
// and writes to stderr
type PluginLogger struct{}

func (pl PluginLogger) Write(p []byte) (n int, err error) {
	_, err = os.Stderr.Write([]byte("\x1B[01;46mlntxbot\x1B[0m " + string(p)))
	return len(p), err
}
