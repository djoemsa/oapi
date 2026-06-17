package proxy

import (
	"testing"
)

func TestLogWriter(t *testing.T) {
	lw := NewLogWriter(5)
	msg := []byte("hello world\n")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("expected write length %d, got %d", len(msg), n)
	}

	select {
	case line := <-lw.Subscribe():
		if line != "hello world\n" {
			t.Errorf("expected 'hello world\\n', got %q", line)
		}
	default:
		t.Error("expected message on channel")
	}
}

func TestLogWriterNonBlocking(t *testing.T) {
	lw := NewLogWriter(2)
	// Write 3 messages, the third should be dropped since buffer is 2
	_, _ = lw.Write([]byte("msg1\n"))
	_, _ = lw.Write([]byte("msg2\n"))
	_, _ = lw.Write([]byte("msg3\n"))

	ch := lw.Subscribe()
	select {
	case m := <-ch:
		if m != "msg1\n" {
			t.Errorf("expected 'msg1\\n', got %q", m)
		}
	default:
		t.Fatal("expected message 1")
	}

	select {
	case m := <-ch:
		if m != "msg2\n" {
			t.Errorf("expected 'msg2\\n', got %q", m)
		}
	default:
		t.Fatal("expected message 2")
	}

	select {
	case m := <-ch:
		t.Fatalf("unexpected message 3 received: %q", m)
	default:
		// success: msg3 dropped as buffer was full
	}
}
