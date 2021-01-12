package app

import (
	"context"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type LifeCycle interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type Hook struct {
	Start func(context.Context) error
	Stop  func(context.Context) error
}

type options struct {
	startTimeOut time.Duration
	stopTimeOut  time.Duration
	sigs         []os.Signal
	siFn         func(*App, os.Signal)
}

type Option func(o *options)

type App struct {
	opts   options
	start_before []Hook
	hooks  []Hook
	cancel func()
}

func SetStartTimeOut(t time.Duration) Option {
	return func(o *options) {
		o.startTimeOut = t
	}
}

func New(opts ...Option) *App {
	options := options{
		startTimeOut: time.Second * 30,
		stopTimeOut:  time.Second * 30,
		sigs: []os.Signal{
			syscall.SIGTERM,
			syscall.SIGQUIT,
			syscall.SIGINT,
		},
		siFn: func(app *App, signal os.Signal) {
			switch signal {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				app.Stop()
			default:

			}
		},
	}
	for _, o := range opts {
		o(&options)
	}
	return &App{
		opts: options,
	}
}

func (a *App) Run() error {
	// 建立监控
	var ctx context.Context
	ctx, a.cancel = context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	// append to start before
	for _, hook := range a.start_before {
		hook := hook
		if hook.Stop != nil {
			g.Go(func() error {
				<-ctx.Done()
				stopCtx, cancel := context.WithTimeout(ctx, a.opts.stopTimeOut)
				defer cancel()
				return hook.Stop(stopCtx)
			})
		}
		if hook.Start != nil {
			g.Go(func() error {
				starCtx, cancel := context.WithTimeout(ctx, a.opts.startTimeOut)
				defer cancel()
				return hook.Start(starCtx)
			})
		}
	}


	for _, hook := range a.hooks {
		hook := hook
		if hook.Stop != nil {
			g.Go(func() error {
				<-ctx.Done()
				stopCtx, cancel := context.WithTimeout(ctx, a.opts.stopTimeOut)
				defer cancel()
				return hook.Stop(stopCtx)
			})
		}
		if hook.Start != nil {
			g.Go(func() error {
				starCtx, cancel := context.WithTimeout(ctx, a.opts.startTimeOut)
				defer cancel()
				return hook.Start(starCtx)
			})
		}
	}
	// 没有设置信号，就关闭掉
	if len(a.opts.sigs) == 0 {
		return g.Wait()
	}
	c := make(chan os.Signal, len(a.opts.sigs))
	signal.Notify(c, a.opts.sigs...)
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case sig := <-c:
				if a.opts.siFn != nil {
					a.opts.siFn(a, sig)
				}
			}

		}
	})

	return g.Wait()
}

func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) AppendStartBefore(lc LifeCycle){
	a.start_before = append(a.start_before, Hook{
		Start: func(ctx context.Context) error {
			return lc.Start(ctx)
		},
		Stop: func(ctx context.Context) error {
			return lc.Stop(ctx)
		},
	})
}

func (a *App) Append(lc LifeCycle) {
	a.hooks = append(a.hooks, Hook{
		Start: func(ctx context.Context) error {
			return lc.Start(ctx)
		},
		Stop: func(ctx context.Context) error {
			return lc.Stop(ctx)
		},
	})
}

func (a *App) AppendHook(hook Hook) {
	a.hooks = append(a.hooks, hook)
}
