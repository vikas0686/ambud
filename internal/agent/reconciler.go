// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vikas0686/ambud/internal/apitypes"
	"github.com/vikas0686/ambud/internal/runtime"
)

// Reconciler registers this node with a control plane (once, persisting
// the resulting credential) and then, on a fixed interval, reports
// current state and reconciles local containerd toward whatever
// desired state the control plane returns — see docs/ROADMAP.md's
// Phase 3 note on the agent becoming "a local controller reconciling
// toward desired state" instead of a dumb command executor.
type Reconciler struct {
	cp              controlPlaneAPI
	rt              runtime.Runtime
	collector       *ResourceCollector
	credentialsPath string
	joinToken       string
	nodeName        string
	interval        time.Duration
	logger          *slog.Logger
}

// NewReconciler builds a Reconciler. joinToken is only used the first
// time this node registers (when no credential file exists yet at
// credentialsPath) — on every subsequent start, the saved credential
// is used instead and joinToken is ignored.
func NewReconciler(
	cp controlPlaneAPI, rt runtime.Runtime, collector *ResourceCollector,
	credentialsPath, joinToken, nodeName string, interval time.Duration, logger *slog.Logger,
) *Reconciler {
	return &Reconciler{
		cp: cp, rt: rt, collector: collector,
		credentialsPath: credentialsPath, joinToken: joinToken, nodeName: nodeName,
		interval: interval, logger: logger,
	}
}

// Run registers (if needed) and then heartbeats on Reconciler's
// interval until ctx is cancelled. A heartbeat or reconcile failure is
// logged and retried on the next tick — a control plane outage must
// not stop the agent from serving its local API or crash it; see
// docs/ARCHITECTURE.md's "fail static, not fail empty" design
// principle.
func (r *Reconciler) Run(ctx context.Context) error {
	nodeID, credential, err := r.ensureRegistered(ctx)
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}
	r.logger.Info("registered with control plane", "node_id", nodeID)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.heartbeatOnce(ctx, nodeID, credential)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.heartbeatOnce(ctx, nodeID, credential)
		}
	}
}

func (r *Reconciler) ensureRegistered(ctx context.Context) (nodeID, credential string, err error) {
	cred, found, err := LoadCredential(r.credentialsPath)
	if err != nil {
		return "", "", err
	}
	if found {
		return cred.NodeID, cred.Credential, nil
	}

	if r.joinToken == "" {
		return "", "", errors.New("no saved credential at " + r.credentialsPath + " and no join token provided")
	}

	nodeID, credential, err = r.cp.Register(ctx, r.joinToken, r.nodeName)
	if err != nil {
		return "", "", err
	}
	if saveErr := SaveCredential(r.credentialsPath, NodeCredential{NodeID: nodeID, Credential: credential}); saveErr != nil {
		return "", "", fmt.Errorf("save credential: %w", saveErr)
	}
	return nodeID, credential, nil
}

func (r *Reconciler) heartbeatOnce(ctx context.Context, nodeID, credential string) {
	containers, err := r.rt.List(ctx)
	if err != nil {
		r.logger.Error("list local containers for heartbeat failed", "error", err)
		return
	}

	req := apitypes.HeartbeatRequest{
		Resources:  r.collector.Latest(),
		Containers: toAPIStatuses(containers),
	}

	resp, err := r.cp.Heartbeat(ctx, nodeID, credential, req)
	if err != nil {
		r.logger.Error("heartbeat failed", "error", err)
		return
	}

	r.reconcile(ctx, containers, resp.Workloads)
}

// reconcile starts any desired workload that isn't currently running
// locally — including one that crashed: containerd keeps a container's
// record around (state Stopped or Unknown) after its process exits
// until something deletes it, so "present in current" is not the same
// as "running," and checking presence alone would silently leave a
// crashed workload dead until something else touched it. It does not
// stop or remove containers outside desired — Phase 3 has no
// workload-deletion API yet, so nothing should ever be "extra" in
// normal operation; see docs/ROADMAP.md's Phase 3.
func (r *Reconciler) reconcile(ctx context.Context, current []runtime.ContainerStatus, desired []apitypes.WorkloadSpec) {
	byName := make(map[string]runtime.ContainerStatus, len(current))
	for _, c := range current {
		byName[c.Name] = c
	}

	for _, wl := range desired {
		existing, exists := byName[wl.Name]
		if exists && existing.State == runtime.StateRunning {
			continue
		}

		if exists {
			// A stopped/crashed container's record still occupies the
			// name (and its snapshot) in containerd — Run would fail
			// with ErrAlreadyExists without clearing it first. Stop is
			// safe to call on an already-non-running container; it
			// just removes the stale record. See internal/runtime's
			// Containerd.Stop and its stopTask helper.
			r.logger.Info("reconcile: clearing stale container before restart", "name", wl.Name, "state", existing.State)
			if err := r.rt.Stop(ctx, wl.Name); err != nil && !errors.Is(err, runtime.ErrNotFound) {
				r.logger.Error("reconcile: clear stale container failed", "name", wl.Name, "error", err)
				continue
			}
		}

		r.logger.Info("reconcile: starting desired workload", "name", wl.Name, "image", wl.Image)
		if err := r.rt.Run(ctx, wl.Name, wl.Image, fromAPIPorts(wl.Ports)); err != nil {
			r.logger.Error("reconcile: start workload failed", "name", wl.Name, "error", err)
		}
	}
}
