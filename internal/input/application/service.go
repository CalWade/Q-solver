package application

import (
	"Q-Solver/internal/input/domain"
	"errors"
	"fmt"
	"sync"
)

type Service struct {
	changeMu     sync.Mutex
	mu           sync.Mutex
	driver       Driver
	commands     Commands
	observer     Observer
	engine       *domain.Engine
	profile      domain.Policy
	filterPolicy domain.Policy
	plan         NativePlan
	caps         Capabilities
	revision     uint64
	editing      bool
	running      bool
	failure      string
	driverFailed bool
	queue        chan string
	notify       chan struct{}
	done         chan struct{}
	workers      sync.WaitGroup
}

func NewService(driver Driver, commands Commands, observer Observer) *Service {
	return &Service{driver: driver, commands: commands, observer: observer}
}

func (s *Service) Start(profile domain.Policy) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	return s.start(profile)
}

func (s *Service) start(profile domain.Policy) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	initial := profile.Clone()
	initial.Suspended = true
	initial.Bindings = nil
	engine, err := domain.NewEngine(initial)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.engine, s.profile = engine, profile.Clone()
	s.filterPolicy, s.plan, s.editing = initial, NativePlan{}, false
	s.caps = Capabilities{}
	s.revision++
	s.queue, s.notify, s.done = make(chan string, 128), make(chan struct{}, 1), make(chan struct{})
	s.running, s.failure, s.driverFailed = true, "", false
	s.workers.Add(1)
	go s.work()
	s.mu.Unlock()
	caps, err := s.driver.Start(Callbacks{Event: s.handle, Shortcut: s.shortcut, Seed: s.seed, Failed: s.failed})
	s.mu.Lock()
	s.caps = caps
	if s.driverFailed {
		s.caps.Filtering = false
		s.caps.Reason = s.failure
	}
	s.mu.Unlock()
	if err == nil {
		err = s.configure(profile, nil)
	}
	if err != nil {
		s.mu.Lock()
		s.failure = err.Error()
		s.mu.Unlock()
	}
	s.signal()
	return err
}

// Configure applies a complete revision. persist runs outside the event lock.
// A failed native update or save restores the previous registration plan.
func (s *Service) Configure(profile domain.Policy, persist func() error) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	return s.configure(profile, persist)
}

func (s *Service) configure(profile domain.Policy, persist func() error) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return errors.New("键盘服务尚未启动")
	}
	caps, previous, editing, revision := s.caps, s.plan, s.editing, s.revision+1
	s.mu.Unlock()
	if profile.Protection.Enabled && !caps.Filtering && !editing {
		return fmt.Errorf("键盘防护尚未生效：%s", caps.Reason)
	}
	filtered := profile.Clone()
	filtered.Bindings = nil
	filtered.Suspended = editing
	plan := NativePlan{Revision: revision}
	for _, binding := range profile.Bindings {
		if editing && !caps.Filtering {
			continue
		}
		protected := false
		for _, key := range binding.Keys {
			protected = protected || domain.Family(key) == profile.Modifier
		}
		if !(profile.Protection.Enabled && protected) && s.driver.CanRegister(binding) {
			if !editing {
				plan.Bindings = append(plan.Bindings, binding)
			}
		} else {
			if !caps.Filtering {
				return fmt.Errorf("快捷键 %s 需要系统输入权限", binding.Action)
			}
			filtered.Bindings = append(filtered.Bindings, binding)
		}
	}
	if err := s.driver.Apply(plan); err != nil {
		return err
	}
	if persist != nil {
		if err := persist(); err != nil {
			if rollback := s.driver.Apply(previous); rollback != nil {
				s.failed(fmt.Errorf("保存失败且快捷键恢复失败：%w", rollback))
			}
			return err
		}
	}
	s.mu.Lock()
	s.profile, s.filterPolicy, s.plan, s.revision = profile.Clone(), filtered, plan, revision
	s.failure = ""
	err := s.engine.Configure(filtered)
	s.mu.Unlock()
	s.signal()
	return err
}

func (s *Service) SetEditing(editing bool) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	return s.setEditing(editing)
}

func (s *Service) setEditing(editing bool) error {
	s.mu.Lock()
	previous, profile := s.editing, s.profile.Clone()
	if previous == editing {
		s.mu.Unlock()
		return nil
	}
	s.editing = editing
	s.mu.Unlock()
	if err := s.configure(profile, nil); err != nil {
		s.mu.Lock()
		s.editing = previous
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Service) handle(event domain.Event) domain.Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.engine == nil {
		return domain.Decision{}
	}
	// A stale DOM focus flag must not pause filtering in another application.
	suspended := s.editing && event.OwnWindow
	if s.filterPolicy.Suspended != suspended {
		s.filterPolicy.Suspended = suspended
		s.engine.Configure(s.filterPolicy)
		s.signal()
	}
	wasPending := s.engine.Pending()
	decision := s.engine.Process(event)
	if decision.Action != "" {
		s.enqueueLocked(decision.Action)
	}
	if wasPending != s.engine.Pending() {
		s.signal()
	}
	return decision
}

func (s *Service) shortcut(revision uint64, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.editing || revision != s.revision || s.engine.Pending() {
		return
	}
	s.enqueueLocked(action)
}

func (s *Service) seed(keys []domain.Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.engine != nil {
		s.engine.Seed(keys)
	}
}

func (s *Service) failed(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.failure = err.Error()
	s.driverFailed = true
	s.caps.Filtering = false
	s.caps.Reason = err.Error()
	s.mu.Unlock()
	s.signal()
}

func (s *Service) enqueueLocked(action string) {
	select {
	case s.queue <- action:
	default:
		s.failure = "命令队列已满，请稍后重试"
		s.signal()
	}
}

func (s *Service) signal() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *Service) work() {
	defer s.workers.Done()
	for {
		select {
		case <-s.done:
			return
		case action := <-s.queue:
			if s.commands != nil {
				s.commands.Execute(action)
			}
		case <-s.notify:
			if s.observer != nil {
				s.observer.InputStatusChanged(s.Status())
			}
		}
	}
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{State: "off", Protection: s.profile.Protection, Capabilities: s.caps, Revision: s.revision, Error: s.failure}
	switch {
	case !s.running:
		status.State = "stopped"
	case s.failure != "":
		status.State = "unavailable"
	case s.profile.Protection.Enabled && !s.caps.Filtering:
		status.State = "unavailable"
		status.Error = s.caps.Reason
	case s.engine.Pending():
		status.State = "waiting-release"
	case s.filterPolicy.Suspended:
		status.State = "editing"
	case s.profile.Protection.Enabled:
		status.State = "active"
	}
	return status
}

func (s *Service) Stop() error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	return s.stop()
}

// Restart preserves editing intent when the user retries permissions from a
// settings window; restarting must not arm shortcuts underneath that editor.
func (s *Service) Restart(profile domain.Policy) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	s.mu.Lock()
	editing := s.editing
	s.mu.Unlock()
	if err := s.stop(); err != nil {
		return err
	}
	err := s.start(profile)
	if editing {
		err = errors.Join(err, s.setEditing(true))
	}
	return err
}

func (s *Service) stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	err := s.driver.Stop()
	s.mu.Lock()
	s.running = false
	close(s.done)
	s.mu.Unlock()
	s.workers.Wait()
	return err
}
