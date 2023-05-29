package election

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var (
	ErrAlreadyRunning = errors.New("elector is already running")
	ErrNotRunning     = errors.New("elector is not running")
)

type LeaderChecker interface {
	IsLeader() bool
}

type LeaderGetter interface {
	GetLeader() string
}

type Status interface {
	LeaderGetter
	LeaderChecker
}

type Config struct {
	LeaseName      string
	LeaseNamespace string
	MemberID       string
	LeaseDuration  time.Duration
	RenewDeadline  time.Duration
	RetryPeriod    time.Duration
}

type Elector struct {
	elector *leaderelection.LeaderElector

	mu           sync.RWMutex
	runCtx       context.Context
	cancelRunCtx func()
	electorDone  chan struct{}
}

func New(cfg Config, k8sClient kubernetes.Interface, callbacks leaderelection.LeaderCallbacks, reg prometheus.Registerer) (*Elector, error) {
	leaderelection.SetProvider(metricsProvider(func() leaderelection.SwitchMetric {
		return newLeaderMetrics(reg)
	}))

	le, err := leaderelection.NewLeaderElector(
		leaderelection.LeaderElectionConfig{
			Lock: &resourcelock.LeaseLock{
				LeaseMeta: metav1.ObjectMeta{
					Name:      cfg.LeaseName,
					Namespace: cfg.LeaseNamespace,
				},
				Client: k8sClient.CoordinationV1(),
				LockConfig: resourcelock.ResourceLockConfig{
					Identity: cfg.MemberID,
				},
			},
			Name:            cfg.MemberID, // required to properly set election metrics.
			ReleaseOnCancel: true,
			LeaseDuration:   cfg.LeaseDuration,
			RenewDeadline:   cfg.RenewDeadline,
			RetryPeriod:     cfg.RetryPeriod,
			Callbacks:       callbacks,
		},
	)

	if err != nil {
		return nil, err
	}

	return &Elector{elector: le}, nil
}

func (e *Elector) Status() Status { return e.elector }

func (e *Elector) Start(ctx context.Context) error {
	e.mu.RLock()
	currCtx := e.runCtx
	e.mu.RUnlock()

	if currCtx != nil {
		return ErrAlreadyRunning
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.runCtx != nil {
		return ErrAlreadyRunning
	}

	e.runCtx, e.cancelRunCtx = context.WithCancel(ctx)
	e.electorDone = make(chan struct{})

	go func() {
		e.elector.Run(e.runCtx)
		close(e.electorDone)
	}()

	return nil
}

func (e *Elector) Stop(ctx context.Context) error {
	e.mu.RLock()
	currCtx := e.runCtx
	e.mu.RUnlock()

	if currCtx == nil {
		return ErrNotRunning
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.runCtx == nil {
		return ErrNotRunning
	}

	e.cancelRunCtx()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-e.electorDone:
	}

	e.runCtx = nil
	e.cancelRunCtx = nil
	e.electorDone = nil

	return nil
}
