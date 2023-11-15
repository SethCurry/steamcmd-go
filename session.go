package steamcmd

import (
	"context"
	"fmt"
)

func NewSessionFactory(username string) *SessionFactory {
	if username == "" {
		username = "anonymous"
	}

	return &SessionFactory{
		Username: username,
	}
}

type SessionFactory struct {
	Username string
}

func (s SessionFactory) New(ctx context.Context) (*Session, error) {
	return NewSession(ctx, s.Username)
}

func NewSession(ctx context.Context, username string) (*Session, error) {
	ioSession, err := NewSessionIO(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session IO: %w", err)
	}

	return &Session{
		IO:       ioSession,
		Username: username,
	}, nil
}

type Session struct {
	IO       *SessionIO
	Username string
}

func (s *Session) Login() error {
	_, err := s.IO.Exec("login " + s.Username)
	if err != nil {
		return fmt.Errorf("failed to execute login command: %w", err)
	}

	return nil
}

func (s *Session) ForceInstallDir(installDir string) error {
	_, err := s.IO.Exec("force_install_dir " + installDir)
	if err != nil {
		return fmt.Errorf("failed to force install dir %s: %w", installDir, err)
	}

	return nil
}

func (s *Session) AppUpdate(appID int, validate bool) error {
	cmd := fmt.Sprintf("app_update %d", appID)
	if validate {
		cmd += " validate"
	}

	if _, err := s.IO.Exec(cmd); err != nil {
		return fmt.Errorf("failed to execute app update command: %w", err)
	}

	return nil
}

func (s *Session) InstallMod(appID int, modID int) error {
	cmd := fmt.Sprintf("workshop_download_item %d %d", appID, modID)

	if _, err := s.IO.Exec(cmd); err != nil {
		return fmt.Errorf("failed to execute install mod command: %w", err)
	}

	return nil
}

func (s *Session) Close() error {
	return s.IO.Close()
}
