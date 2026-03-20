package routes

import (
	"path/filepath"
	"testing"
)

func TestIsSafeUploadPath_Subdirectory(t *testing.T) {
	t.Parallel()

	safe, abs := isSafeUploadPath("uploads")
	if !safe {
		t.Fatal("expected uploads to be safe subdirectory")
	}
	if filepath.Base(abs) != "uploads" {
		t.Fatalf("abs path = %s, expected basename uploads", abs)
	}
}

func TestIsSafeUploadPath_CurrentDirIsUnsafe(t *testing.T) {
	t.Parallel()

	safe, abs := isSafeUploadPath(".")
	if safe {
		t.Fatalf("expected current dir to be unsafe, abs=%s", abs)
	}
}

func TestIsSafeUploadPath_ParentDirIsUnsafe(t *testing.T) {
	t.Parallel()

	safe, abs := isSafeUploadPath("..")
	if safe {
		t.Fatalf("expected parent dir to be unsafe, abs=%s", abs)
	}
}

func TestIsSafeUploadPath_AbsoluteRootIsUnsafe(t *testing.T) {
	t.Parallel()

	safe, abs := isSafeUploadPath("/")
	if safe {
		t.Fatalf("expected root path to be unsafe, abs=%s", abs)
	}
}
