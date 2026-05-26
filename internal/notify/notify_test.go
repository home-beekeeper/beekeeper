package notify

import (
	"errors"
	"testing"
)

func TestNotifyDisabled(t *testing.T) {
	called := false
	orig := notifyFunc
	notifyFunc = func(title, message string, icon any) error {
		called = true
		return nil
	}
	defer func() { notifyFunc = orig }()

	Notify(Config{Enabled: false}, "title", "message")
	if called {
		t.Fatal("notifyFunc should not be called when Enabled=false")
	}
}

func TestNotifyBestEffort(t *testing.T) {
	orig := notifyFunc
	notifyFunc = func(title, message string, icon any) error {
		return errors.New("notification backend error")
	}
	defer func() { notifyFunc = orig }()

	// Should not panic and should not propagate the error.
	Notify(Config{Enabled: true}, "title", "message")
}
