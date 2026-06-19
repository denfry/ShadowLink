// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"fmt"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
)

// Instance is a running (or constructed) embedded sing-box.
type Instance struct {
	b   *box.Box
	ctx context.Context
}

// New constructs a sing-box instance from rendered JSON. It populates the
// mandatory DI context (include.Context) required since sing-box v1.11 and parses
// with sing-box's context-aware JSON (NOT encoding/json).
func New(configJSON []byte) (*Instance, error) {
	ctx := include.Context(context.Background())
	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, configJSON)
	if err != nil {
		return nil, fmt.Errorf("parse sing-box config: %w", err)
	}
	b, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		return nil, fmt.Errorf("construct sing-box: %w", err)
	}
	return &Instance{b: b, ctx: ctx}, nil
}

// Start brings the tunnel up (privileged: creates TUN, programs routes).
func (i *Instance) Start() error {
	if i.b == nil {
		return fmt.Errorf("instance not constructed")
	}
	return i.b.Start()
}

// Close stops the instance and releases resources.
func (i *Instance) Close() error {
	if i.b == nil {
		return nil
	}
	return i.b.Close()
}
