package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/yarpc/transport/http"
	"go.uber.org/yarpc/yarpcerrors"
	"golang.org/x/sync/errgroup"
)

type Handler struct {
	ServiceName string
	Outbounds   yarpc.Outbounds
}

func (h *Handler) Handle(ctx context.Context, req *transport.Request, _ transport.ResponseWriter) error {
	_, err := h.HandleCall(ctx, req)
	return err
}

func (h *Handler) HandleCall(ctx context.Context, _ *transport.Request) (*transport.Response, error) {
	if os.Getenv("SHOULD_FAIL") == "true" {
		if bypass, _ := ctx.Value(bypassKey).(bool); !bypass {
			return nil, yarpcerrors.Newf(getErrorCode(os.Getenv("ERROR_TYPE")), "an error occurred in service %s", h.ServiceName)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	downstreams := os.Getenv("DOWNSTREAMS")
	if downstreams != "" {
		for _, ds := range strings.Split(downstreams, ",") {
			target := strings.Split(ds, "://")[1]
			svcName := strings.Split(target, ":")[0]

			g.Go(func() error {
				outReq := &transport.Request{
					Caller:    h.ServiceName,
					Service:   svcName,
					Procedure: "call",
					Encoding:  "raw",
				}
				_, err := h.Outbounds[svcName].Unary.Call(ctx, outReq)
				return err
			})
		}
	}

	if err := g.Wait(); err != nil {
		if os.Getenv("FORCE_ERROR_MAPPING") == "true" {
			return nil, yarpcerrors.Newf(getErrorCode(os.Getenv("ERROR_TYPE")), "an upstream dependency failed in service %s", h.ServiceName)
		}
		return nil, err
	}
	res := &transport.Response{
		Body: io.NopCloser(strings.NewReader("OK")),
	}
	return res, nil
}

func main() {
	svcName := os.Getenv("SERVICE_NAME")
	mw := &TreeMiddleware{ServiceName: svcName}
	httpT := http.NewTransport()
	grpcT := grpc.NewTransport()

	outbounds := yarpc.Outbounds{}
	if ds := os.Getenv("DOWNSTREAMS"); ds != "" {
		for _, d := range strings.Split(ds, ",") {
			parts := strings.Split(d, "://")
			name := strings.Split(parts[1], ":")[0]
			outbounds[name] = transport.Outbounds{
				Unary: grpcT.NewSingleOutbound(parts[1]),
			}
		}
	}

	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatalf("failed to open port 8081: %v", err)
	}
	defer listener.Close()

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: svcName,
		Inbounds: yarpc.Inbounds{
			httpT.NewInbound(":8080"),
			grpcT.NewInbound(listener),
		},
		Outbounds:          outbounds,
		InboundMiddleware:  yarpc.InboundMiddleware{Unary: mw},
		OutboundMiddleware: yarpc.OutboundMiddleware{Unary: mw},
	})

	h := &Handler{ServiceName: svcName, Outbounds: dispatcher.Outbounds()}
	dispatcher.Register([]transport.Procedure{{
		Name:        "call",
		HandlerSpec: transport.NewUnaryHandlerSpec(h),
	}})

	log.Printf("Service %s starting...", svcName)
	if err := dispatcher.Start(); err != nil {
		log.Fatal(err)
	}
	select {}
}

func getErrorCode(code string) yarpcerrors.Code {
	switch strings.ToLower(code) {
	case "notfound":
		return yarpcerrors.CodeNotFound
	case "permissiondenied":
		return yarpcerrors.CodePermissionDenied
	case "unauthenticated":
		return yarpcerrors.CodeUnauthenticated
	default:
		return yarpcerrors.CodeInternal
	}
}
