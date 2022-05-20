package api

import (
	"context"
	"net"
	"net/http"
	"time"
)

// StartServerArgs is intended as args for func StartServer. Check if it's set
// up correctly with the StartServerArgs.Ok() method.
type StartServerArgs struct {
	// Addr will be used as the address for this http server.
	Addr string
	// Ctx is used to shut down the server and all associated state.
	Ctx context.Context

	// ReadTimeout is the read timeout for this http server.
	ReadTimeout time.Duration
	// WriteTimeout is the write timeout for this http server.
	WriteTimeout time.Duration

	// OnStart is called in a new goroutine right after the server starts
	// listening successfully. This is intended to work with a sync.WaitGroup.
	// or something similar, in order to continue logic after start.
	OnStart func()
	// onRunning is similar to OnStart but is called after the server has
	// started serving requests. It also gives access to the *handle after
	// it is set up; as such it is only intended for in-pkg testing.
	// Note that it is also started with a separate goroutine.
	onRunning func(h *handle)

	// UpdateFrequencyAddrSet specifies how often the internal set of rpc addrs
	// will be refreshed. These addrs are used with the /service/ops pkg for
	// things such as doing KNN requests -- the refreshing will be done by
	// calling /service/ops/Client.Ping and is as such costly network calls.
	// Note that adding these addrs is done with endpoint ip:port/ops/addrs/put.
	UpdateFrequencyAddrSet time.Duration
}

// Ok returns true if all the minimum requirements are met, specifically:
// - args.Ctx != nil
// - args.ReadTimeout > 0
// - args.WriteTimeout > 0
// - args.UpdateFrequencyAddrSet > 0
func (args *StartServerArgs) Ok() bool {
	ok := true
	ok = ok && args.Ctx != nil
	ok = ok && args.ReadTimeout > 0
	ok = ok && args.WriteTimeout > 0
	ok = ok && args.UpdateFrequencyAddrSet > 0
	return ok
}

// StartServer starts the http server in this pkg, see docs of StartServerArgs
// for details about configuration. This has a few fail cases:
// - (false, nil) if args.Ok() == false.
// - (false, err) if net.Listen(...) fails. This might be caused by for example
//   an args.Addr that is formatted madly or is simply in use (i.e port).
// - (true, err) if http.Server.Serve(...) returns false after start.
// - (true,  ? ) if args.Ctx is done. The unknown/potential err will be from
//   Server.Shutdown(...).
func StartServer(args StartServerArgs) (bool, error) {
	if !args.Ok() {
		return false, nil
	}

	// Start listener.
	l, err := net.Listen("tcp", args.Addr)
	if err != nil {
		return false, err
	}

	// Signal started.
	if args.OnStart != nil {
		go args.OnStart()
	}

	// Setup server.
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:         args.Addr,
		Handler:      mux,
		ReadTimeout:  args.ReadTimeout,
		WriteTimeout: args.WriteTimeout,
	}

	chErr := make(chan error)
	go func() {
		chErr <- srv.Serve(l)
		close(chErr)
	}()

	// Setup handle and routes.
	h := handle{
		ctx: args.Ctx,
		addrSet: addrSet{
			_addrs:          make(map[string]bool),
			updateFrequency: args.UpdateFrequencyAddrSet,
		},
	}
	h.registerRoutes(mux)

	// Give handle to testing.
	if args.onRunning != nil {
		go args.onRunning(&h)
	}

	// Wait.
	select {
	case err := <-chErr:
		return true, err
	case <-args.Ctx.Done():
		return true, srv.Shutdown(context.Background())
	}
}
