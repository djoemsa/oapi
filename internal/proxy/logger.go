package proxy

// LogWriter implements io.Writer and sends written log messages to a channel.
// It is designed to be thread-safe via Go's native channel implementation.
type LogWriter struct {
	ch chan string
}

// NewLogWriter creates a new LogWriter with the specified buffer size.
func NewLogWriter(bufSize int) *LogWriter {
	return &LogWriter{
		ch: make(chan string, bufSize),
	}
}

// Write implements the io.Writer interface. It performs a non-blocking send
// to the channel, dropping messages if the channel buffer is full to avoid
// slowing down the proxy server.
func (lw *LogWriter) Write(p []byte) (int, error) {
	line := string(p)
	select {
	case lw.ch <- line:
	default:
		// Drop message if channel is full to prevent blocking
	}
	return len(p), nil
}

// Subscribe returns the receive-only channel for log messages.
func (lw *LogWriter) Subscribe() <-chan string {
	return lw.ch
}
