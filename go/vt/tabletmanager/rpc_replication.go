// Copyright 2016, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tabletmanager

import (
	"flag"
	"fmt"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/youtube/vitess/go/vt/mysqlctl"
	"github.com/youtube/vitess/go/vt/mysqlctl/replication"
	"github.com/youtube/vitess/go/vt/topo"
	"github.com/youtube/vitess/go/vt/topo/topoproto"
	"github.com/youtube/vitess/go/vt/topotools"

	replicationdatapb "github.com/youtube/vitess/go/vt/proto/replicationdata"
	topodatapb "github.com/youtube/vitess/go/vt/proto/topodata"
)

var (
	enableSemiSync = flag.Bool("enable_semi_sync", false, "Enable semi-sync when configuring replication, on master and replica tablets only (rdonly tablets will not ack).")
)

// SlaveStatus returns the replication status
// Should be called under RPCWrap.
func (agent *ActionAgent) SlaveStatus(ctx context.Context) (*replicationdatapb.Status, error) {
	status, err := agent.MysqlDaemon.SlaveStatus()
	if err != nil {
		return nil, err
	}
	return replication.StatusToProto(status), nil
}

// MasterPosition returns the master position
// Should be called under RPCWrap.
func (agent *ActionAgent) MasterPosition(ctx context.Context) (string, error) {
	pos, err := agent.MysqlDaemon.MasterPosition()
	if err != nil {
		return "", err
	}
	return replication.EncodePosition(pos), nil
}

// StopSlave will stop the replication. Works both when Vitess manages
// replication or not (using hook if not).
// Should be called under RPCWrapLock.
func (agent *ActionAgent) StopSlave(ctx context.Context) error {
	// Remember that we were told to stop, so we don't try to
	// restart ourselves (in replication_reporter).
	agent.setSlaveStopped(true)

	// Also tell Orchestrator we're stopped on purpose for some Vitess task.
	// Do this in the background, as it's best-effort.
	go func() {
		if agent.orc == nil {
			return
		}
		if err := agent.orc.BeginMaintenance(agent.Tablet(), "vttablet has been told to StopSlave"); err != nil {
			log.Warningf("Orchestrator BeginMaintenance failed: %v", err)
		}
	}()

	return mysqlctl.StopSlave(agent.MysqlDaemon, agent.hookExtraEnv())
}

// StopSlaveMinimum will stop the slave after it reaches at least the
// provided position. Works both when Vitess manages
// replication or not (using hook if not).
func (agent *ActionAgent) StopSlaveMinimum(ctx context.Context, position string, waitTime time.Duration) (string, error) {
	pos, err := replication.DecodePosition(position)
	if err != nil {
		return "", err
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitTime)
	defer cancel()
	if err := agent.MysqlDaemon.WaitMasterPos(waitCtx, pos); err != nil {
		return "", err
	}
	if err := agent.StopSlave(ctx); err != nil {
		return "", err
	}
	pos, err = agent.MysqlDaemon.MasterPosition()
	if err != nil {
		return "", err
	}
	return replication.EncodePosition(pos), nil
}

// StartSlave will start the replication. Works both when Vitess manages
// replication or not (using hook if not).
// Should be called under RPCWrapLock.
func (agent *ActionAgent) StartSlave(ctx context.Context) error {
	agent.setSlaveStopped(false)

	// Tell Orchestrator we're no longer stopped on purpose.
	// Do this in the background, as it's best-effort.
	go func() {
		if agent.orc == nil {
			return
		}
		if err := agent.orc.EndMaintenance(agent.Tablet()); err != nil {
			log.Warningf("Orchestrator EndMaintenance failed: %v", err)
		}
	}()

	if *enableSemiSync {
		if err := agent.enableSemiSync(false); err != nil {
			return err
		}
	}
	return mysqlctl.StartSlave(agent.MysqlDaemon, agent.hookExtraEnv())
}

// GetSlaves returns the address of all the slaves
// Should be called under RPCWrap.
func (agent *ActionAgent) GetSlaves(ctx context.Context) ([]string, error) {
	return mysqlctl.FindSlaves(agent.MysqlDaemon)
}

// ResetReplication completely resets the replication on the host.
// All binary and relay logs are flushed. All replication positions are reset.
func (agent *ActionAgent) ResetReplication(ctx context.Context) error {
	cmds, err := agent.MysqlDaemon.ResetReplicationCommands()
	if err != nil {
		return err
	}
	agent.setSlaveStopped(true)
	return agent.MysqlDaemon.ExecuteSuperQueryList(ctx, cmds)
}

// InitMaster enables writes and returns the replication position.
func (agent *ActionAgent) InitMaster(ctx context.Context) (string, error) {
	// we need to insert something in the binlogs, so we can get the
	// current position. Let's just use the mysqlctl.CreateReparentJournal commands.
	cmds := mysqlctl.CreateReparentJournal()
	if err := agent.MysqlDaemon.ExecuteSuperQueryList(ctx, cmds); err != nil {
		return "", err
	}

	// get the current replication position
	pos, err := agent.MysqlDaemon.MasterPosition()
	if err != nil {
		return "", err
	}

	// If using semi-sync, we need to enable it before going read-write.
	if *enableSemiSync {
		if err := agent.enableSemiSync(true); err != nil {
			return "", err
		}
	}

	// Set the server read-write, from now on we can accept real
	// client writes. Note that if semi-sync replication is enabled,
	// we'll still need some slaves to be able to commit transactions.
	if err := agent.MysqlDaemon.SetReadOnly(false); err != nil {
		return "", err
	}

	// Change our type to master if not already
	if _, err := agent.TopoServer.UpdateTabletFields(ctx, agent.TabletAlias, func(tablet *topodatapb.Tablet) error {
		tablet.Type = topodatapb.TabletType_MASTER
		return nil
	}); err != nil {
		return "", err
	}

	agent.initReplication = true
	return replication.EncodePosition(pos), nil
}

// PopulateReparentJournal adds an entry into the reparent_journal table.
func (agent *ActionAgent) PopulateReparentJournal(ctx context.Context, timeCreatedNS int64, actionName string, masterAlias *topodatapb.TabletAlias, position string) error {
	pos, err := replication.DecodePosition(position)
	if err != nil {
		return err
	}
	cmds := mysqlctl.CreateReparentJournal()
	cmds = append(cmds, mysqlctl.PopulateReparentJournal(timeCreatedNS, actionName, topoproto.TabletAliasString(masterAlias), pos))

	return agent.MysqlDaemon.ExecuteSuperQueryList(ctx, cmds)
}

// InitSlave sets replication master and position, and waits for the
// reparent_journal table entry up to context timeout
func (agent *ActionAgent) InitSlave(ctx context.Context, parent *topodatapb.TabletAlias, position string, timeCreatedNS int64) error {
	pos, err := replication.DecodePosition(position)
	if err != nil {
		return err
	}
	ti, err := agent.TopoServer.GetTablet(ctx, parent)
	if err != nil {
		return err
	}

	agent.setSlaveStopped(false)

	// If using semi-sync, we need to enable it before connecting to master.
	if *enableSemiSync {
		if err := agent.enableSemiSync(false); err != nil {
			return err
		}
	}

	cmds, err := agent.MysqlDaemon.SetSlavePositionCommands(pos)
	if err != nil {
		return err
	}
	cmds2, err := agent.MysqlDaemon.SetMasterCommands(ti.Hostname, int(ti.PortMap["mysql"]))
	if err != nil {
		return err
	}
	cmds = append(cmds, cmds2...)
	cmds = append(cmds, "START SLAVE")

	if err := agent.MysqlDaemon.ExecuteSuperQueryList(ctx, cmds); err != nil {
		return err
	}
	agent.initReplication = true

	// wait until we get the replicated row, or our context times out
	return agent.MysqlDaemon.WaitForReparentJournal(ctx, timeCreatedNS)
}

// DemoteMaster marks the server read-only, wait until it is done with
// its current transactions, and returns its master position.
// Should be called under RPCWrapLockAction.
func (agent *ActionAgent) DemoteMaster(ctx context.Context) (string, error) {
	// Set the server read-only. Note all active connections are not
	// affected.
	if err := agent.MysqlDaemon.SetReadOnly(true); err != nil {
		return "", err
	}

	// Now disallow queries, to make sure nobody is writing to the
	// database.
	tablet := agent.Tablet()
	// We don't care if the QueryService state actually changed because we'll
	// let vtgate keep serving read traffic from this master (see comment below).
	log.Infof("DemoteMaster disabling query service")
	if _ /* state changed */, err := agent.QueryServiceControl.SetServingType(tablet.Type, false, nil); err != nil {
		return "", fmt.Errorf("SetServingType(serving=false) failed: %v", err)
	}

	// If using semi-sync, we need to disable master-side.
	if *enableSemiSync {
		if err := agent.enableSemiSync(false); err != nil {
			return "", err
		}
	}

	pos, err := agent.MysqlDaemon.DemoteMaster()
	if err != nil {
		return "", err
	}
	return replication.EncodePosition(pos), nil
	// There is no serving graph update - the master tablet will
	// be replaced. Even though writes may fail, reads will
	// succeed. It will be less noisy to simply leave the entry
	// until we'll promote the master.
}

// PromoteSlaveWhenCaughtUp waits for this slave to be caught up on
// replication up to the provided point, and then makes the slave the
// shard master.
func (agent *ActionAgent) PromoteSlaveWhenCaughtUp(ctx context.Context, position string) (string, error) {
	pos, err := replication.DecodePosition(position)
	if err != nil {
		return "", err
	}

	if err := agent.MysqlDaemon.WaitMasterPos(ctx, pos); err != nil {
		return "", err
	}

	pos, err = agent.MysqlDaemon.PromoteSlave(agent.hookExtraEnv())
	if err != nil {
		return "", err
	}

	// If using semi-sync, we need to enable it before going read-write.
	if *enableSemiSync {
		if err := agent.enableSemiSync(true); err != nil {
			return "", err
		}
	}

	if err := agent.MysqlDaemon.SetReadOnly(false); err != nil {
		return "", err
	}

	if _, err := topotools.ChangeType(ctx, agent.TopoServer, agent.TabletAlias, topodatapb.TabletType_MASTER); err != nil {
		return "", err
	}

	return replication.EncodePosition(pos), nil
}

// SlaveWasPromoted promotes a slave to master, no questions asked.
// Should be called under RPCWrapLockAction.
func (agent *ActionAgent) SlaveWasPromoted(ctx context.Context) error {
	_, err := topotools.ChangeType(ctx, agent.TopoServer, agent.TabletAlias, topodatapb.TabletType_MASTER)
	return err
}

// SetMaster sets replication master, and waits for the
// reparent_journal table entry up to context timeout
// Should be called under RPCWrapLockAction.
func (agent *ActionAgent) SetMaster(ctx context.Context, parentAlias *topodatapb.TabletAlias, timeCreatedNS int64, forceStartSlave bool) error {
	parent, err := agent.TopoServer.GetTablet(ctx, parentAlias)
	if err != nil {
		return err
	}

	// See if we were replicating at all, and should be replicating
	wasReplicating := false
	shouldbeReplicating := false
	rs, err := agent.MysqlDaemon.SlaveStatus()
	if err == nil && (rs.SlaveIORunning || rs.SlaveSQLRunning) {
		wasReplicating = true
		shouldbeReplicating = true
	}
	if forceStartSlave {
		shouldbeReplicating = true
	}

	// If using semi-sync, we need to enable it before connecting to master.
	if *enableSemiSync {
		if err := agent.enableSemiSync(false); err != nil {
			return err
		}
	}

	// Create the list of commands to set the master
	cmds := []string{}
	if wasReplicating {
		cmds = append(cmds, mysqlctl.SQLStopSlave)
	}
	smc, err := agent.MysqlDaemon.SetMasterCommands(parent.Hostname, int(parent.PortMap["mysql"]))
	if err != nil {
		return err
	}
	cmds = append(cmds, smc...)
	if shouldbeReplicating {
		cmds = append(cmds, mysqlctl.SQLStartSlave)
	}
	if err := agent.MysqlDaemon.ExecuteSuperQueryList(ctx, cmds); err != nil {
		return err
	}

	// change our type to REPLICA if we used to be the master
	runHealthCheck := false
	_, err = agent.TopoServer.UpdateTabletFields(ctx, agent.TabletAlias, func(tablet *topodatapb.Tablet) error {
		if tablet.Type == topodatapb.TabletType_MASTER {
			tablet.Type = topodatapb.TabletType_REPLICA
			runHealthCheck = true
			return nil
		}
		return topo.ErrNoUpdateNeeded
	})
	if err != nil {
		return err
	}

	// if needed, wait until we get the replicated row, or our
	// context times out
	if !shouldbeReplicating || timeCreatedNS == 0 {
		return nil
	}
	if err := agent.MysqlDaemon.WaitForReparentJournal(ctx, timeCreatedNS); err != nil {
		return err
	}
	if runHealthCheck {
		agent.runHealthCheckProtected()
	}
	return nil
}

// SlaveWasRestarted updates the parent record for a tablet.
// Should be called under RPCWrapLockAction.
func (agent *ActionAgent) SlaveWasRestarted(ctx context.Context, parent *topodatapb.TabletAlias) error {
	runHealthCheck := false

	// Once this action completes, update authoritative tablet node first.
	if _, err := agent.TopoServer.UpdateTabletFields(ctx, agent.TabletAlias, func(tablet *topodatapb.Tablet) error {
		if tablet.Type == topodatapb.TabletType_MASTER {
			tablet.Type = topodatapb.TabletType_REPLICA
			runHealthCheck = true
			return nil
		}
		return topo.ErrNoUpdateNeeded
	}); err != nil {
		return err
	}

	if runHealthCheck {
		agent.runHealthCheckProtected()
	}
	return nil
}

// StopReplicationAndGetStatus stops MySQL replication, and returns the
// current status
func (agent *ActionAgent) StopReplicationAndGetStatus(ctx context.Context) (*replicationdatapb.Status, error) {
	// get the status before we stop replication
	rs, err := agent.MysqlDaemon.SlaveStatus()
	if err != nil {
		return nil, fmt.Errorf("before status failed: %v", err)
	}
	if !rs.SlaveIORunning && !rs.SlaveSQLRunning {
		// no replication is running, just return what we got
		return replication.StatusToProto(rs), nil
	}
	if err := agent.StopSlave(ctx); err != nil {
		return nil, fmt.Errorf("stop slave failed: %v", err)
	}
	// now patch in the current position
	rs.Position, err = agent.MysqlDaemon.MasterPosition()
	if err != nil {
		return nil, fmt.Errorf("after position failed: %v", err)
	}
	return replication.StatusToProto(rs), nil
}

// PromoteSlave makes the current tablet the master
func (agent *ActionAgent) PromoteSlave(ctx context.Context) (string, error) {
	pos, err := agent.MysqlDaemon.PromoteSlave(agent.hookExtraEnv())
	if err != nil {
		return "", err
	}

	// If using semi-sync, we need to enable it before going read-write.
	if *enableSemiSync {
		if err := agent.enableSemiSync(true); err != nil {
			return "", err
		}
	}

	// Set the server read-write
	if err := agent.MysqlDaemon.SetReadOnly(false); err != nil {
		return "", err
	}

	if _, err := topotools.ChangeType(ctx, agent.TopoServer, agent.TabletAlias, topodatapb.TabletType_MASTER); err != nil {
		return "", err
	}

	return replication.EncodePosition(pos), nil
}

func (agent *ActionAgent) isMasterEligible() (bool, error) {
	switch agent.Tablet().Type {
	case topodatapb.TabletType_MASTER, topodatapb.TabletType_REPLICA:
		return true, nil
	}

	return false, nil
}

func (agent *ActionAgent) enableSemiSync(master bool) error {
	// Only enable if we're eligible for becoming master (REPLICA type).
	// Ineligible slaves (RDONLY) shouldn't ACK because we'll never promote them.
	masterEligible, err := agent.isMasterEligible()
	if err != nil {
		return fmt.Errorf("can't enable semi-sync: %v", err)
	}
	if !masterEligible {
		return nil
	}

	// Always enable slave-side since it doesn't hurt to keep it on for a master.
	// The master-side needs to be off for a slave, or else it will get stuck.
	return agent.MysqlDaemon.SetSemiSyncEnabled(master, true)
}
