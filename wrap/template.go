package wrap

const (
	wrapperTemplate = `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .Package }}

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	gofrGRPC "gofr.dev/pkg/gofr/grpc"
)

type {{ .Service }}ServerWithGofr interface {
{{- range .Methods }}
	{{- if not .Streaming }}
	{{ .Name }}(*gofr.Context) (any, error)
	{{- end }}
{{- end }}
}

type healthServer struct {
	*health.Server
}

type {{ .Service }}ServerWrapper struct {
	{{ .Service }}Server
	*healthServer
	Container *container.Container
	server    {{ .Service }}ServerWithGofr
}

{{- range .Methods }}
{{- if not .Streaming }}
func (h *{{ $.Service }}ServerWrapper) {{ .Name }}(ctx context.Context, req *{{ .Request }}) (*{{ .Response }}, error) {
	gctx := h.GetGofrContext(ctx, &{{ .Request }}Wrapper{ctx: ctx, {{ .Request }}: req})

	res, err := h.server.{{ .Name }}(gctx)
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*{{ .Response }})
	if !ok {
		return nil, status.Errorf(codes.Unknown, "unexpected response type %T", res)
	}

	return resp, nil
}
{{- end }}
{{- end }}

func (h *healthServer) Check(ctx *gofr.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	start := time.Now()
	span := ctx.Trace("check")
	res, err := h.Server.Check(ctx.Context, req)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, err, "/grpc.health.v1.Health/Check", "app_gRPC-Server_stats")
	span.End()
	return res, err
}

func (h *healthServer) Watch(ctx *gofr.Context, in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	start := time.Now()
	span := ctx.Trace("watch")
	err := h.Server.Watch(in, stream)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, err, "/grpc.health.v1.Health/Watch", "app_gRPC-Server_stats")
	span.End()
	return err
}

func (h *healthServer) SetServingStatus(ctx *gofr.Context, service string, servingStatus healthpb.HealthCheckResponse_ServingStatus) {
	start := time.Now()
	span := ctx.Trace("setServingStatus")
	h.Server.SetServingStatus(service, servingStatus)
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, 
	"/grpc.health.v1.Health/SetServingStatus", "app_gRPC-Server_stats")
	span.End()
}

func (h *healthServer) Shutdown(ctx *gofr.Context) {
	start := time.Now()
	span := ctx.Trace("Shutdown")
	h.Server.Shutdown()
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, "/grpc.health.v1.Health/Shutdown", "app_gRPC-Server_stats")
	span.End()
}

func (h *healthServer) Resume(ctx *gofr.Context) {
	start := time.Now()
	span := ctx.Trace("Resume")
	h.Server.Resume()
	gofrGRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), start, nil, "/grpc.health.v1.Health/Resume", "app_gRPC-Server_stats")
	span.End()
}

func (h *{{ .Service }}ServerWrapper) mustEmbedUnimplemented{{ .Service }}Server() {}

func Register{{ .Service }}ServerWithGofr(app *gofr.App, srv {{ .Service }}ServerWithGofr) {
	var s grpc.ServiceRegistrar = app

	h := health.NewServer()
	res, _ := srv.(*{{ .Service }}GoFrServer)
	res.health = &healthServer{h}

	wrapper := &{{ .Service }}ServerWrapper{server: srv, healthServer: res.health}

	gRPCBuckets := []float64{0.005, 0.01, .05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	app.Metrics().NewHistogram("app_gRPC-Server_stats", "Response time of gRPC server in milliseconds.", gRPCBuckets...)

	Register{{ .Service }}Server(s, wrapper)
	healthpb.RegisterHealthServer(s, h)

	h.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	h.SetServingStatus("{{ .Service }}", healthpb.HealthCheckResponse_SERVING)
}

func (h *{{ .Service }}ServerWrapper) GetGofrContext(ctx context.Context, req gofr.Request) *gofr.Context {
	return &gofr.Context{
		Context:   ctx,
		Container: h.Container,
		Request:   req,
	}
}

{{- range $request := .Requests }}
type {{ $request }}Wrapper struct {
	ctx context.Context
	*{{ $request }}
}

func (h *{{ $request }}Wrapper) Context() context.Context {
	return h.ctx
}

func (h *{{ $request }}Wrapper) Param(s string) string {
	return ""
}

func (h *{{ $request }}Wrapper) PathParam(s string) string {
	return ""
}

func (h *{{ $request }}Wrapper) Bind(p interface{}) error {
	ptr := reflect.ValueOf(p)
	if ptr.Kind() != reflect.Ptr {
		return fmt.Errorf("expected a pointer, got %T", p)
	}

	hValue := reflect.ValueOf(h.{{ $request }}).Elem()
	ptrValue := ptr.Elem()

	for i := 0; i < hValue.NumField(); i++ {
		field := hValue.Type().Field(i)
		if field.Name == "state" || field.Name == "sizeCache" || field.Name == "unknownFields" {
			continue
		}

		if field.IsExported() {
			ptrValue.Field(i).Set(hValue.Field(i))
		}
	}

	return nil
}

func (h *{{ $request }}Wrapper) HostName() string {
	return ""
}

func (h *{{ $request }}Wrapper) Params(s string) []string {
	return nil
}
{{- end }}
`

	serverTemplate = `package {{ .Package }}

import "gofr.dev/pkg/gofr"

// Register the gRPC service in your app using the following code in your main.go:
//
// {{ .Package }}.Register{{ $.Service }}ServerWithGofr(app, &{{ .Package }}.{{ $.Service }}GoFrServer{})
//
// {{ $.Service }}GoFrServer defines the gRPC server implementation.
// Customize the struct with required dependencies and fields as needed.

type {{ $.Service }}GoFrServer struct {
 health *healthServer
}

{{- range .Methods }}
func (s *{{ $.Service }}GoFrServer) {{ .Name }}(ctx *gofr.Context) (any, error) {
// Uncomment and use the following code if you need to bind the request payload
// request := {{ .Request }}{}
// err := ctx.Bind(&request)
// if err != nil {
//     return nil, err
// }

return &{{ .Response }}{}, nil
}
{{- end }}
`
	clientTemplate = `// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package {{ .Package }}

import (
	"time"

	"gofr.dev/pkg/gofr"
	gofrgRPC "gofr.dev/pkg/gofr/grpc"
	"gofr.dev/pkg/gofr/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	 _ "google.golang.org/grpc/health"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type {{ .Service }}GoFrClient interface {
{{- range .Methods }}
	{{ .Name }}(*gofr.Context, *{{ .Request }}, ...grpc.CallOption) (*{{ .Response }}, error)
{{- end }}
	health
}

type health interface {
	Check(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error)
	Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, 
	opts ...grpc.CallOption) (grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse], error)
}

type {{ .Service }}ClientWrapper struct {
	client {{ .Service }}Client
	health grpc_health_v1.HealthClient
	{{ .Service }}GoFrClient
}

func createGRPCConn(host string) (*grpc.ClientConn, error) {
	serviceConfig := ` + "`" + `{
        "loadBalancingPolicy": "round_robin",
        "healthCheckConfig": {
            "serviceName": "{{ .Service }}"
        }
    }` + "`" + `

	conn, err := grpc.Dial(host,
		grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func New{{ .Service }}GoFrClient(host string, metrics metrics.Manager) ({{ .Service }}GoFrClient, error) {
	conn, err := createGRPCConn(host)
	if err != nil {
		return &{{ .Service }}ClientWrapper{client: nil}, err
	}

	gRPCBuckets := []float64{0.005, 0.01, .05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	metrics.NewHistogram("app_gRPC-Client_stats", "Response time of gRPC client in milliseconds.", gRPCBuckets...)

	res := New{{ .Service }}Client(conn)
	healthClient := grpc_health_v1.NewHealthClient(conn)

	return &{{ .Service }}ClientWrapper{
		client: res,
		health: healthClient,
	}, nil
}

func invokeRPC(ctx *gofr.Context, rpcName string, rpcFunc func() (interface{}, error)) (interface{}, error) {
	span := ctx.Trace("gRPC-srv-call: " + rpcName)
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	md := metadata.Pairs("x-gofr-traceid", traceID, "x-gofr-spanid", spanID)

	ctx.Context = metadata.NewOutgoingContext(ctx.Context, md)
	transactionStartTime := time.Now()

	res, err := rpcFunc()
	gofrgRPC.DocumentRPCLog(ctx.Context, ctx.Logger, ctx.Metrics(), transactionStartTime, err, rpcName,"app_gRPC-Client_stats")

	return res, err
}

{{- range .Methods }}
func (h *{{ $.Service }}ClientWrapper) {{ .Name }}(ctx *gofr.Context, req *{{ .Request }}, 
opts ...grpc.CallOption) (*{{ .Response }}, error) {
	result, err := invokeRPC(ctx, "/{{ $.Service }}/{{ .Name }}", func() (interface{}, error) {
		return h.client.{{ .Name }}(ctx.Context, req, opts...)
	})

	if err != nil {
		return nil, err
	}
	return result.(*{{ .Response }}), nil
}
{{- end }}

func (h *{{ $.Service }}ClientWrapper) Check(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, 
opts ...grpc.CallOption) (*grpc_health_v1.HealthCheckResponse, error) {
	result, err := invokeRPC(ctx, "/grpc.health.v1.Health/Check", func() (interface{}, error) {
		return h.health.Check(ctx.Context, in, opts...)
	})

	if err != nil {
		return nil, err
	}
	return result.(*grpc_health_v1.HealthCheckResponse), nil
}

func (h *{{ $.Service }}ClientWrapper) Watch(ctx *gofr.Context, in *grpc_health_v1.HealthCheckRequest, 
opts ...grpc.CallOption) (grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse], error) {
	result, err := invokeRPC(ctx, "/grpc.health.v1.Health/Watch", func() (interface{}, error) {
		return h.health.Watch(ctx, in, opts...)
	})

	if err != nil {
		return nil, err
	}
	return result.(grpc.ServerStreamingClient[grpc_health_v1.HealthCheckResponse]), nil
}
`
)
