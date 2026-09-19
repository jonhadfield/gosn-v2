package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// defaultKeyring is where the session functions read and write the session
// when they are passed a nil keyring. When it is nil, they use the system
// keyring.
var defaultKeyring keyring.Keyring

// SetDefaultKeyring sets the store that session functions use when they are
// passed a nil keyring, including GetSession and the refresh it performs.
// Passing nil switches back to the system keyring.
func SetDefaultKeyring(k keyring.Keyring) {
	defaultKeyring = k
}

func resolveKeyring(k keyring.Keyring) keyring.Keyring {
	if k != nil {
		return k
	}

	if defaultKeyring != nil {
		return defaultKeyring
	}

	return systemKeyring{}
}

// systemKeyring uses the operating system's keyring, such as the macOS
// Keychain or a Secret Service provider on Linux.
type systemKeyring struct{}

func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (systemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (systemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

func (systemKeyring) DeleteAll(service string) error {
	return keyring.DeleteAll(service)
}

// FileKeyring stores the session in a file rather than the system keyring,
// for machines without a keyring service, such as headless servers.
//
// The file holds a single secret, so the service and user arguments are
// ignored. It is written with 0600 permissions. To protect it further, encrypt
// the session with a session key.
type FileKeyring struct {
	Path string
}

var _ keyring.Keyring = (*FileKeyring)(nil)

// NewFileKeyring returns a FileKeyring that stores the session at path.
func NewFileKeyring(path string) *FileKeyring {
	return &FileKeyring{Path: path}
}

// Get returns the stored session, or keyring.ErrNotFound if the file does not
// exist or is empty.
func (f *FileKeyring) Get(_, _ string) (string, error) {
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", keyring.ErrNotFound
	}

	if err != nil {
		return "", fmt.Errorf("failed to read session file: %w", err)
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", keyring.ErrNotFound
	}

	return s, nil
}

// Set writes the session to the file, creating its directory if needed. The
// write goes to a temporary file that is then renamed over the old one, so an
// interrupted write cannot leave a truncated session behind.
func (f *FileKeyring) Set(_, _, secret string) error {
	dir := filepath.Dir(f.Path)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create session file directory: %w", err)
	}

	// CreateTemp creates the file with 0600 permissions
	tmp, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err = tmp.WriteString(secret); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("failed to write session file: %w", err)
	}

	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	if err = os.Rename(tmp.Name(), f.Path); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Delete removes the session file, or returns keyring.ErrNotFound if it does
// not exist.
func (f *FileKeyring) Delete(_, _ string) error {
	err := os.Remove(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return keyring.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("failed to remove session file: %w", err)
	}

	return nil
}

// DeleteAll removes the session file.
func (f *FileKeyring) DeleteAll(_ string) error {
	return f.Delete("", "")
}
