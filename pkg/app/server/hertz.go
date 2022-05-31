/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package server

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/route"
)

// Hertz is the core struct of hertz.
type Hertz struct {
	*route.Engine
}

// New creates a hertz instance without any default config.
func New(opts ...config.Option) *Hertz {
	options := config.NewOptions(opts)
	h := &Hertz{
		Engine: route.NewEngine(options),
	}
	return h
}

// Default creates a hertz instance with default middlewares.
func Default(opts ...config.Option) *Hertz {
	h := New(opts...)
	h.Use(recovery.Recovery())

	return h
}

// Spin runs the server until catching os.Signal.
// SIGTERM triggers immediately close.
// SIGHUP|SIGINT triggers graceful shutdown.
func (h *Hertz) Spin() {
	errCh := make(chan error)
	go func() {
		errCh <- h.Run()
	}()

	if err := waitSignal(errCh); err != nil {
		hlog.Errorf("HERTZ: Receive close signal: error=%v", err)
		if err := h.Engine.Close(); err != nil {
			hlog.Errorf("HERTZ: Close error=%v", err)
		}
		return
	}

	hlog.Infof("HERTZ: Begin graceful shutdown, wait at most num=%d seconds...", h.GetOptions().ExitWaitTimeout/time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), h.GetOptions().ExitWaitTimeout)
	defer cancel()

	if err := h.Shutdown(ctx); err != nil {
		hlog.Errorf("HERTZ: Shutdown error=%v", err)
	}
}

func waitSignal(errCh chan error) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGTERM:
			// force exit
			return errors.New(sig.String()) // nolint
		case syscall.SIGHUP, syscall.SIGINT:
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		return err
	}

	return nil
}
