package gitversion

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"riftsync/internal/config"
	"riftsync/internal/state"
)

const (
	defaultDebounce       = 2 * time.Second
	defaultCommandTimeout = 20 * time.Second
)

type Options struct {
	Debounce       time.Duration
	CommandTimeout time.Duration
}

type Service struct {
	cfg            config.Config
	state          *state.AppState
	debounce       time.Duration
	commandTimeout time.Duration
	notifications  chan revisionRequest
	stopOnce       sync.Once
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

type revisionRequest struct {
	rev         int
	changeCount int
}

func New(cfg config.Config, appState *state.AppState, options Options) *Service {
	debounce := options.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	commandTimeout := options.CommandTimeout
	if commandTimeout <= 0 {
		commandTimeout = defaultCommandTimeout
	}
	return &Service{
		cfg:            cfg,
		state:          appState,
		debounce:       debounce,
		commandTimeout: commandTimeout,
		notifications:  make(chan revisionRequest, 1),
	}
}

func (s *Service) Start(parent context.Context) {
	s.state.AddRevisionListener(s.NotifyRevision)

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	if !s.cfg.GitVersioningEnabled {
		s.state.SetGitState(state.GitState{
			Enabled:  false,
			Status:   state.GitStatusDisabled,
			RepoPath: s.cfg.SyncRootAbs,
		})
		return
	}

	s.state.SetGitState(state.GitState{
		Enabled:  true,
		Status:   state.GitStatusChecking,
		RepoPath: s.cfg.SyncRootAbs,
	})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
}

func (s *Service) Close() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
	s.wg.Wait()
}

func (s *Service) NotifyRevision(event state.RevisionEvent) {
	if !s.cfg.GitVersioningEnabled || event.Rev <= 0 {
		return
	}
	request := revisionRequest{rev: event.Rev, changeCount: event.ChangeCount}
	select {
	case s.notifications <- request:
	default:
		select {
		case <-s.notifications:
		default:
		}
		select {
		case s.notifications <- request:
		default:
		}
	}
}

func (s *Service) run(ctx context.Context) {
	if _, err := s.EnsureRepository(ctx); err != nil {
		s.setError(err)
	}

	var pending *revisionRequest
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case request := <-s.notifications:
			pending = &request
			if timer == nil {
				timer = time.NewTimer(s.debounce)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(s.debounce)
			}
		case <-timerC:
			if pending != nil {
				s.commitRevision(ctx, *pending)
				pending = nil
			}
			timer = nil
			timerC = nil
		}
	}
}

func (s *Service) EnsureRepository(ctx context.Context) (state.GitState, error) {
	gitState := state.GitState{
		Enabled:  s.cfg.GitVersioningEnabled,
		Status:   state.GitStatusChecking,
		RepoPath: s.cfg.SyncRootAbs,
	}

	if !s.cfg.GitVersioningEnabled {
		gitState.Status = state.GitStatusDisabled
		s.state.SetGitState(gitState)
		return gitState, nil
	}
	s.state.SetGitState(gitState)
	if err := os.MkdirAll(s.cfg.SyncRootAbs, 0o755); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	if _, _, err := s.runGit(ctx, "--version"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	gitState.Available = true
	s.state.SetGitState(gitState)

	if _, err := os.Stat(s.gitDir()); errors.Is(err, os.ErrNotExist) {
		gitState.Status = state.GitStatusInitializing
		s.state.SetGitState(gitState)
		if _, _, err := s.runGit(ctx, "init"); err != nil {
			gitState.Status = state.GitStatusError
			gitState.LastError = err.Error()
			s.state.SetGitState(gitState)
			return gitState, err
		}
		gitState.Status = state.GitStatusChecking
		s.state.SetGitState(gitState)
	}

	if err := s.ensureConfig(ctx, "user.name", "RiftSync"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	if err := s.ensureConfig(ctx, "user.email", "riftsync@local"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	if err := s.setConfig(ctx, "core.longpaths", "true"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	if err := s.setConfig(ctx, "core.autocrlf", "false"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}
	if err := s.setConfig(ctx, "core.safecrlf", "false"); err != nil {
		gitState.Status = state.GitStatusError
		gitState.LastError = err.Error()
		s.state.SetGitState(gitState)
		return gitState, err
	}

	if !s.hasHead(ctx) {
		status, _, statusErr := s.runGit(ctx, "status", "--porcelain")
		if statusErr != nil {
			gitState.Status = state.GitStatusError
			gitState.LastError = statusErr.Error()
			s.state.SetGitState(gitState)
			return gitState, statusErr
		}
		if strings.TrimSpace(status) != "" {
			gitState.Status = state.GitStatusInitialCommit
			s.state.SetGitState(gitState)
			if _, _, err := s.runGit(ctx, "add", "-A"); err != nil {
				gitState.Status = state.GitStatusError
				gitState.LastError = err.Error()
				s.state.SetGitState(gitState)
				return gitState, err
			}
			if _, _, err := s.runGit(ctx, "commit", "-m", "RiftSync initial snapshot rev 0"); err != nil {
				gitState.Status = state.GitStatusError
				gitState.LastError = err.Error()
				s.state.SetGitState(gitState)
				return gitState, err
			}
		}
	}

	gitState = s.readHeadState(ctx, gitState)
	gitState.Status = state.GitStatusReady
	s.state.SetGitState(gitState)
	return gitState, nil
}

func (s *Service) commitRevision(ctx context.Context, request revisionRequest) {
	if _, err := s.EnsureRepository(ctx); err != nil {
		s.setError(err)
		return
	}
	current := s.state.GitState()
	current.Status = state.GitStatusCommitting
	current.Enabled = true
	current.RepoPath = s.cfg.SyncRootAbs
	s.state.SetGitState(current)
	if _, _, err := s.runGit(ctx, "add", "-A"); err != nil {
		s.setError(err)
		return
	}
	message := fmt.Sprintf("RiftSync rev %d: %d changes", request.rev, request.changeCount)
	if _, _, err := s.runGit(ctx, "commit", "--allow-empty", "-m", message); err != nil {
		s.setError(err)
		return
	}

	gitState := s.readHeadState(ctx, state.GitState{
		Enabled:   true,
		Available: true,
		Status:    state.GitStatusReady,
		RepoPath:  s.cfg.SyncRootAbs,
	})
	gitState.Status = state.GitStatusReady
	s.state.SetGitState(gitState)
	s.state.UpdateRevisionGitCommit(request.rev, gitState.LastCommit, gitState.LastCommitShort)
}

func (s *Service) ensureConfig(ctx context.Context, key, fallback string) error {
	value, _, err := s.runGit(ctx, "config", "--get", key)
	if err == nil && strings.TrimSpace(value) != "" {
		return nil
	}
	return s.setConfig(ctx, key, fallback)
}

func (s *Service) setConfig(ctx context.Context, key, value string) error {
	_, _, err := s.runGit(ctx, "config", key, value)
	return err
}

func (s *Service) hasHead(ctx context.Context) bool {
	_, _, err := s.runGit(ctx, "rev-parse", "--verify", "HEAD")
	return err == nil
}

func (s *Service) readHeadState(ctx context.Context, current state.GitState) state.GitState {
	commit, _, err := s.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		if isMissingHeadError(err) {
			current.LastCommit = ""
			current.LastCommitShort = ""
			current.LastError = ""
			current.Status = state.GitStatusReady
			return current
		}
		current.Status = state.GitStatusError
		current.LastError = err.Error()
		return current
	}
	current.LastCommit = strings.TrimSpace(commit)
	if len(current.LastCommit) >= 7 {
		current.LastCommitShort = current.LastCommit[:7]
	} else {
		current.LastCommitShort = current.LastCommit
	}
	current.LastError = ""
	current.Status = state.GitStatusReady
	return current
}

func isMissingHeadError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "ambiguous argument 'HEAD'") ||
		strings.Contains(message, "unknown revision or path not in the working tree") ||
		strings.Contains(message, "Needed a single revision")
}

func (s *Service) runGit(parent context.Context, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(parent, s.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	hideCommandWindow(cmd)
	cmd.Dir = s.cfg.SyncRootAbs
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("git %s timed out", strings.Join(args, " "))
	}
	if err != nil {
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = err.Error()
		}
		return stdout.String(), stderr.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), details)
	}
	return stdout.String(), stderr.String(), nil
}

func (s *Service) setError(err error) {
	if err == nil {
		return
	}
	current := s.state.GitState()
	current.Enabled = s.cfg.GitVersioningEnabled
	current.RepoPath = s.cfg.SyncRootAbs
	current.Status = state.GitStatusError
	current.LastError = err.Error()
	s.state.SetGitState(current)
}

func (s *Service) gitDir() string {
	return s.cfg.SyncRootAbs + string(os.PathSeparator) + ".git"
}
