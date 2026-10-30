package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/internal/mcpauth"
	"github.com/adambiggs/gangline/internal/mcpevents"
	"github.com/adambiggs/gangline/internal/outsidemcp"
	"github.com/adambiggs/gangline/store"
)

type reportDirectories []string

func (d *reportDirectories) String() string         { return fmt.Sprint([]string(*d)) }
func (d *reportDirectories) Set(value string) error { *d = append(*d, value); return nil }

type outsideService struct {
	http    http.Handler
	events  outsideEvents
	reports *outsidemcp.Reports
	run     *runtime
}

type outsideEvents interface {
	ActivePrincipals(context.Context) ([]string, error)
	Deliver(context.Context, string, string, time.Time) error
	Prune(string, []string) error
}

func (run *runtime) newOutsideService(issuer, clientID, redirect string, roots []string) (*outsideService, error) {
	paths := store.Paths{Root: run.settings.StateRoot}
	authorize := func(principal string) error {
		_, err := paths.ResolveOutsidePrincipal(run.settings.Session, principal)
		return err
	}
	auth, err := mcpauth.New(mcpauth.Config{Issuer: issuer, ClientID: clientID, RedirectURI: redirect, StateDir: filepath.Join(run.team.Directory, "outside-mcp"), Clock: run.cmd.now,
		AuthenticateCredential: func(token string) (string, error) {
			p, err := paths.AuthenticateOutside(run.settings.Session, token)
			if err != nil {
				return "", err
			}
			return p.String(), nil
		}, Authorize: authorize})
	if err != nil {
		return nil, err
	}
	events, err := mcpevents.New(mcpevents.Config{StatePath: filepath.Join(run.team.Directory, "outside-mcp", "events.json"), Now: run.cmd.now, Authorize: func(_ context.Context, p string) (bool, error) {
		err := authorize(p)
		if errors.Is(err, store.ErrOutsideUnauthorized) {
			return false, nil
		}
		return err == nil, err
	}})
	if err != nil {
		return nil, err
	}
	reports, err := outsidemcp.OpenReports(roots)
	if err != nil {
		return nil, err
	}
	handler := outsidemcp.New(outsidemcp.Config{Origin: issuer, Challenge: auth.Challenge(), Authenticate: auth.AuthenticateAccess, Authorize: authorize, Reports: reports, Events: events,
		ReportError: func(err error) { fmt.Fprintf(run.cmd.stderr, "outside MCP operation failed: %v\n", err) },
		PublicError: func(err error) bool {
			var e commandError
			return errors.As(err, &e) && (e.status == exitUsage || e.status == exitRefused)
		},
		Pending: run.outsidePending,
		Ack: func(opaque string, ids []string) error {
			principal, err := paths.ResolveOutsidePrincipal(run.settings.Session, opaque)
			if err != nil {
				return err
			}
			return paths.WithOutside(run.settings.Session, principal, func(m *store.OutsideMailbox) error {
				values := make([]core.EnvelopeID, len(ids))
				for i, id := range ids {
					values[i] = core.EnvelopeID(id)
				}
				return m.Ack(values)
			})
		},
		Send: func(p, to, body, key string) (outsidemcp.Receipt, error) {
			r, err := run.outsideSend(p, to, body, key)
			return outsidemcp.Receipt{ID: r.ID, Outcome: r.Outcome}, err
		},
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/", auth)
	return &outsideService{http: mux, events: events, reports: reports, run: run}, nil
}

func (run *runtime) outsidePending(opaque string) ([]outsidemcp.Mail, error) {
	paths := store.Paths{Root: run.settings.StateRoot}
	principal, err := paths.ResolveOutsidePrincipal(run.settings.Session, opaque)
	if err != nil {
		return nil, err
	}
	envelopes, err := paths.ReadOutsidePending(run.settings.Session, principal)
	if err != nil {
		return nil, err
	}
	mail := make([]outsidemcp.Mail, 0, len(envelopes))
	for _, e := range envelopes {
		mail = append(mail, outsidemcp.Mail{ID: string(e.ID), From: senderLabel(e.From), Body: e.Message.Text, CreatedAt: e.CreatedAt})
	}
	return mail, nil
}

func (s *outsideService) flush(ctx context.Context) error {
	principals, err := s.events.ActivePrincipals(ctx)
	if err != nil {
		return err
	}
	for _, p := range principals {
		pending, err := s.run.outsidePending(p)
		if errors.Is(err, store.ErrOutsideUnauthorized) || errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(pending))
		for _, e := range pending {
			ids = append(ids, e.ID)
			if err := s.events.Deliver(ctx, p, e.ID, e.CreatedAt); err != nil {
				fmt.Fprintf(s.run.cmd.stderr, "outside event %s failed: %v; pending mail remains available through fetch_mail\n", e.ID, err)
			}
		}
		if err := s.events.Prune(p, ids); err != nil {
			return err
		}
	}
	return nil
}

// watchMail arms the filesystem watch before reading mail, so an atomic
// mailbox write cannot fall between the read and the wait.
func (s *outsideService) watchMail(ctx context.Context) error {
	path := filepath.Join(s.run.team.Directory, "outside", ".mail-changes")
	for {
		watch, err := store.Watch(path)
		if err != nil {
			return err
		}
		if err = s.flush(ctx); err != nil {
			watch.Close()
			return err
		}
		err = watch.Wait(ctx)
		closeErr := watch.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func (cmd command) outsideServe(args []string) error {
	o := outsideOptions{listen: "127.0.0.1:8484"}
	flags := outsideFlags(&o)
	positionals, err := parseOptions(flags, args)
	if err != nil {
		return usageError("outside serve: %v", err)
	}
	if len(positionals) != 0 {
		return usageError("outside serve: unexpected arguments")
	}
	if flagWasSet(flags, "credential-file") {
		return usageError("outside serve: --credential-file is a register/rotate option")
	}
	host, _, err := net.SplitHostPort(o.listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return usageError("outside serve: --listen must be a loopback IP address and port")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	if _, err := run.team.ReadTeam(); err != nil {
		return err
	}
	state := filepath.Join(run.team.Directory, "outside-mcp")
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(state, "server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return refuseError("outside MCP server state is already in use: %v", err)
	}
	service, err := run.newOutsideService(o.issuer, o.clientID, o.redirect, o.roots)
	if err != nil {
		return err
	}
	defer service.reports.Close()
	if err := os.MkdirAll(filepath.Join(run.team.Directory, "outside"), 0700); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", o.listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server := &http.Server{Handler: service.http}
	results := make(chan error, 2)
	go func() { results <- server.Serve(listener) }()
	go func() { results <- service.watchMail(ctx) }()
	fmt.Fprintf(cmd.stdout, "outside MCP listening on %s; HTTPS resource %s/mcp\n", listener.Addr(), o.issuer)
	var result error
	select {
	case result = <-results:
	case <-ctx.Done():
	}
	cancel()
	closeErr := server.Close()
	if errors.Is(result, http.ErrServerClosed) || errors.Is(result, context.Canceled) {
		result = nil
	}
	return errors.Join(result, closeErr)
}
