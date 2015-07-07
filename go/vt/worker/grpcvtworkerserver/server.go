// Copyright 2015, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package grpcvtworkerserver contains the gRPC implementation of the server side
of the remote execution of vtworker commands.
*/
package grpcvtworkerserver

import (
	"sync"

	"google.golang.org/grpc"

	"github.com/youtube/vitess/go/vt/logutil"
	"github.com/youtube/vitess/go/vt/servenv"
	"github.com/youtube/vitess/go/vt/worker"

	pbl "github.com/youtube/vitess/go/vt/proto/logutil"
	pb "github.com/youtube/vitess/go/vt/proto/vtworkerdata"
	pbs "github.com/youtube/vitess/go/vt/proto/vtworkerservice"
)

// VtworkerServer is our RPC server
type VtworkerServer struct {
	wi *worker.Instance
}

// NewVtworkerServer returns a new Vtworker Server for the topo server.
func NewVtworkerServer(wi *worker.Instance) *VtworkerServer {
	return &VtworkerServer{wi}
}

// ExecuteVtworkerCommand is part of the pb.VtworkerServer interface
func (s *VtworkerServer) ExecuteVtworkerCommand(args *pb.ExecuteVtworkerCommandRequest, stream pbs.Vtworker_ExecuteVtworkerCommandServer) (err error) {
	// Please note that this panic handler catches only panics occuring in the code below.
	// The actual execution of the vtworker command takes place in a new go routine
	// (started in Instance.setAndStartWorker()) which has its own panic handler.
	defer servenv.HandlePanic("vtworker", &err)

	// create a logger, send the result back to the caller
	logstream := logutil.NewChannelLogger(10)
	logger := logutil.NewTeeLogger(logstream, logutil.NewMemoryLogger())

	// send logs to the caller
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for e := range logstream {
			// Note we don't interrupt the loop here, as
			// we still need to flush and finish the
			// command, even if the channel to the client
			// has been broken. We'll just keep trying.
			stream.Send(&pb.ExecuteVtworkerCommandResponse{
				Event: &pbl.Event{
					Time: &pbl.Time{
						Seconds:     e.Time.Unix(),
						Nanoseconds: int32(e.Time.Nanosecond()),
					},
					Level: pbl.Level(e.Level),
					File:  e.File,
					Line:  int64(e.Line),
					Value: e.Value,
				},
			})
		}
		wg.Done()
	}()

	// create the wrangler
	wr := s.wi.CreateWrangler(logger)

	// execute the command
	if len(args.Args) >= 1 && args.Args[0] == "Reset" {
		err = s.wi.Reset()
	} else {
		// Make sure we use the global "err" variable and do not redeclare it in this scope.
		var worker worker.Worker
		var done chan struct{}
		worker, done, err = s.wi.RunCommand(args.Args, wr, false /*runFromCli*/)
		if err == nil {
			err = s.wi.WaitForCommand(worker, done)
		}
	}

	// close the log channel, and wait for them all to be sent
	close(logstream)
	wg.Wait()

	return err
}

// StartServer registers the VtworkerServer for RPCs
func StartServer(s *grpc.Server, wi *worker.Instance) {
	pbs.RegisterVtworkerServer(s, NewVtworkerServer(wi))
}