# Hooks

Tego service hooks are a proposed adapter-level extension point for running code around the mapping
steps that sit between native transport messages and facade service implementations.

The main motivation is validation and normalization of either protobuf messages or Tego-generated
request/response structs, especially when facade methods inline request or response fields. With
inlining, user service code may receive ordinary method parameters instead of the generated request
struct. The generated adapter still sees both the protobuf message and the generated request struct
around mapping and inline expansion, so the adapter is the natural place to run hooks.

Hooks should not replace native gRPC interceptors or Connect interceptors. Native interceptors are
still the right tool for transport-level concerns. Proto-side Tego hooks operate on protobuf message
types, not native Connect or gRPC request wrappers. Tego-side hooks operate on facade request and
response types.

## Generated Typed Hooks

Request-specific and response-specific hooks should be generated per service. They are RPC
method-based, not message-type-based. Method names only need to be unique within a protobuf service,
so top-level generated hook types include the service name while fields inside the service hook
struct can stay short.

Each RPC has four mapping hook slots:

- `BeforeGetBookRequestMapping`: protobuf request before `GetBookRequestFromProto`.
- `AfterGetBookRequestMapping`: Tego request after `GetBookRequestFromProto`.
- `BeforeGetBookResponseMapping`: Tego response before `GetBookResponseToProto`.
- `AfterGetBookResponseMapping`: protobuf response after `GetBookResponseToProto`.

```go
type BookServiceHooks struct {
	BeforeGetBookRequestMapping  []BookServiceBeforeGetBookRequestMappingHook
	AfterGetBookRequestMapping   []BookServiceAfterGetBookRequestMappingHook
	BeforeGetBookResponseMapping []BookServiceBeforeGetBookResponseMappingHook
	AfterGetBookResponseMapping  []BookServiceAfterGetBookResponseMappingHook
}

type BookServiceBeforeGetBookRequestMappingHook func(
	context.Context,
	tego.RPCInfo,
	*bookspbv1.GetBookRequest,
) (context.Context, *bookspbv1.GetBookRequest, error)

type BookServiceAfterGetBookRequestMappingHook func(
	context.Context,
	tego.RPCInfo,
	GetBookRequest,
) (context.Context, GetBookRequest, error)

type BookServiceBeforeGetBookResponseMappingHook func(
	context.Context,
	tego.RPCInfo,
	GetBookResponse,
) (GetBookResponse, error)

type BookServiceAfterGetBookResponseMappingHook func(
	context.Context,
	tego.RPCInfo,
	*bookspbv1.GetBookResponse,
) (*bookspbv1.GetBookResponse, error)
```

Typed hooks are fully type-safe. The generated adapter stores the service hook struct and calls the
relevant slice directly:

```go
for _, hook := range a.serviceHooks.AfterGetBookRequestMapping {
	ctx, request, err = hook(ctx, info, request)
	if err != nil {
		return nil, a.mapError(err)
	}
}
```

Request-side hooks can replace the request value and return a new context, which supports
normalization, defaulting, validation, and context enrichment before the facade service call.
Response-side hooks receive the context but do not return it, because there is no later facade
service call to influence. Both response-side hooks can replace the response value at their mapping
slot.

## Adapter Configuration

Typed hooks should be configured on generated adapters rather than squeezed into the existing shared
Tego constructor options. This keeps generated hook APIs type-safe while preserving reusable
transport option slices such as `[]tego.GRPCServerOption` and `[]tego.ConnectHandlerOption`.

```go
adapter := books.NewBookServiceConnectAdapter(store)
var hooks books.BookServiceHooks
hooks.AddAfterGetBookRequestMappingHook(trimGetBookID, validateGetBook)
adapter.AddServiceHooks(hooks)

path, handler := books.NewBookServiceConnectHandlerWithAdapter(
	adapter,
	tego.WithErrorMapper(connectError),
)
```

Generated service hook structs also expose `Set...Hooks` methods for each slot. The public slice
fields remain available for struct literals, but the helper methods are the preferred setup style
when composing hooks in code.

`AddServiceHooks` should append/merge rather than replace, and should return the adapter to allow
compact setup:

```go
adapter := books.NewBookServiceGRPCAdapter(store).AddServiceHooks(bookHooks)
```

`SetServiceHooks` should replace the whole generated hook struct, allowing callers to reset typed
hooks with the zero value:

```go
adapter.SetServiceHooks(books.BookServiceHooks{})
```

Adapters should be treated as setup-time mutable. Hooks are configured before the adapter is used to
serve requests; calling `AddServiceHooks`, `SetServiceHooks`, `AddInterfaceHooks`, or
`SetInterfaceHooks` concurrently with active requests is not expected to be safe.

Generated adapters would grow a hook field:

```go
type BookServiceConnectAdapter struct {
	service        BookService
	errorMapper    tego.ErrorMapper
	serviceHooks   BookServiceHooks
	interfaceHooks tego.InterfaceHooks
}
```

## Construction Scenarios

Tego currently generates several service construction paths. Hooks need a story for each one.

Simple APIs stay unchanged and remain suitable when only shared Tego transport options are needed:

```go
books.RegisterBookServiceGRPCServer(server, store, opts...)
grpcServer := books.NewBookServiceGRPCServer(store, opts...)
path, handler := books.NewBookServiceConnectHandler(store, opts...)
```

Hooked APIs use the generated adapter path:

```go
adapter := books.NewBookServiceGRPCAdapter(store).AddServiceHooks(bookHooks)

books.RegisterBookServiceGRPCServerWithAdapter(server, adapter, opts...)
grpcServer := books.NewBookServiceGRPCServerWithAdapter(adapter, opts...)
```

```go
adapter := books.NewBookServiceConnectAdapter(store).AddServiceHooks(bookHooks)

path, handler := books.NewBookServiceConnectHandlerWithAdapter(adapter, opts...)
```

The missing helper for gRPC registration should be generated so users do not have to manually call
the protobuf registration function:

```go
func RegisterBookServiceGRPCServerWithAdapter(
	registrar grpc.ServiceRegistrar,
	adapter *BookServiceGRPCAdapter,
	opts ...tego.GRPCServerOption,
) {
	bookspbv1.RegisterBookServiceServer(
		registrar,
		NewBookServiceGRPCServerWithAdapter(adapter, opts...),
	)
}
```

## Generic Interface Hooks

Some hooks should apply broadly to any request or response implementing an interface, without
copying a typed hook into every generated field. Tego can provide generic interface hooks for the
same four mapping slots.

```go
type Validator interface {
	Validate() error
}

var hooks tego.InterfaceHooks
hooks.AddAfterRequestMappingHook(tego.AfterRequestMappingInterfaceHook[Validator](func(
	ctx context.Context,
	info tego.RPCInfo,
	request Validator,
) (context.Context, error) {
	return ctx, request.Validate()
}))

adapter := books.NewBookServiceConnectAdapter(store)
adapter.AddInterfaceHooks(hooks)
```

The user-facing API does not need a type check. Internally, Tego or the generated adapter stores
small runtime hook entries for each mapping slot. Each entry checks whether the current request or
response value implements the target interface and skips the hook if it does not.

When one hook needs to inspect multiple interfaces or concrete types, use the explicit `AnyHook`
constructors instead of binding to a single interface:

```go
hooks.AddAfterRequestMappingHook(tego.AfterRequestMappingAnyHook(func(
	ctx context.Context,
	info tego.RPCInfo,
	value any,
) (context.Context, error) {
	if validator, ok := value.(Validator); ok {
		return ctx, validator.Validate()
	}
	return ctx, nil
}))
```

`AnyHook` constructors intentionally match every value at that mapping slot. `InterfaceHook[I]`
constructors are still the better fit when a hook only applies to one interface or concrete type.

```go
func AfterRequestMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) (context.Context, error),
) AfterRequestMappingInterfaceHookFunc {
	return AfterRequestMappingInterfaceHookFunc{
		run: func(ctx context.Context, info RPCInfo, value any) (context.Context, bool, error) {
			typed, ok := value.(I)
			if !ok {
				return ctx, false, nil
			}
			ctx, err := hook(ctx, info, typed)
			return ctx, true, err
		},
	}
}
```

The generated adapter should expose grouped add and set methods:

```go
var hooks tego.InterfaceHooks
hooks.AddBeforeRequestMappingHook(protoValidate)
hooks.AddAfterRequestMappingHook(tegoValidate)
hooks.AddBeforeResponseMappingHook(tegoObserve)
hooks.AddAfterResponseMappingHook(protoObserve)
adapter.AddInterfaceHooks(hooks)
```

`tego.InterfaceHooks` also has per-slot `Set...Hooks` methods. The adapter set method replaces the
whole interface hook group and can reset it with the zero value:

```go
adapter.SetInterfaceHooks(tego.InterfaceHooks{})
```

This is not completely type-static internally, because Go has to perform a dynamic interface
assertion to answer "does this concrete request implement `I`?". The important part is that users do
not write those checks repeatedly.

Pointer receiver methods need an explicit policy. A practical default is to try the value first and
then a pointer to the value for addressable request/response locals. That allows both of these to
work:

```go
func (GetBookRequest) Validate() error
func (*GetBookRequest) Validate() error
```

Interface hooks are narrower than typed hooks: they cannot replace the request or response value.
Typed hooks remain the mechanism for replacement. Interface hooks can still mutate pointer receiver
values where the generated adapter passes an addressable value, such as Tego responses before
protobuf mapping. Proto interface hooks run against protobuf messages; Tego interface hooks run
against generated facade types.

## Ordering

Hooks should run in a predictable order at each adapter boundary:

1. Run `Before...RequestMapping` hooks against the protobuf request.
2. Convert the protobuf request to the Tego request type.
3. Run `After...RequestMapping` hooks against the Tego request.
4. Inline the request if the facade method is inlined.
5. Call the facade service implementation.
6. Re-wrap an inline response into the generated response type if needed.
7. Run `Before...ResponseMapping` hooks against the Tego response.
8. Convert the Tego response type to the protobuf response.
9. Run `After...ResponseMapping` hooks against the protobuf response.

Within each slot, generated typed hooks should run before generic interface hooks. This lets
method-specific normalization happen before broad validation or observation hooks.

For a unary Connect method, the generated flow would look like:

```go
requestProtoMsg := requestProto.Msg
ctx, requestProtoMsg, err = a.runBeforeGetBookRequestMapping(ctx, info, requestProtoMsg)
if err != nil {
	return nil, a.mapError(err)
}
request, err := GetBookRequestFromProto(requestProtoMsg)
if err != nil {
	return nil, err
}
ctx, request, err = a.runAfterGetBookRequestMapping(ctx, info, request)
if err != nil {
	return nil, a.mapError(err)
}

response, err := GetBookResponseFromInline(
	a.service.GetBook(GetBookRequestToInline(ctx, request)),
)
if err != nil {
	return nil, a.mapError(err)
}
response, err = a.runBeforeGetBookResponseMapping(ctx, info, response)
if err != nil {
	return nil, a.mapError(err)
}
responseProto, err := GetBookResponseToProto(response)
if err != nil {
	return nil, err
}
responseProto, err = a.runAfterGetBookResponseMapping(ctx, info, responseProto)
if err != nil {
	return nil, a.mapError(err)
}
return connect.NewResponse(responseProto), nil
```

Errors from request hooks should be mapped through the adapter error mapper, just like facade
service errors. Errors from response hooks should also be mapped as facade-boundary errors.

`RPCInfo` should include the full service identity so generic hooks can branch without relying on
locally unique method names:

```go
type RPCInfo struct {
	Service   string
	Method    string
	Procedure string
}
```

For example:

```go
tego.RPCInfo{
	Service:   "books.v1.BookService",
	Method:    "GetBook",
	Procedure: "/books.v1.BookService/GetBook",
}
```

## Streaming

For streaming methods, hooks apply per message:

- Server-streaming: run request mapping hooks once, then response mapping hooks for each yielded
  response.
- Client-streaming: run request mapping hooks for each received request, then response mapping hooks
  once.
- Bidi-streaming: run request mapping hooks for each received request and response mapping hooks for
  each yielded response.

For inbound streams, hook errors should appear as iterator errors to the facade service. The adapter
should also remember the first request hook error so it can return that error if the service ignores
the iterator error and returns nil. This matches the existing receive-error pattern used by
generated stream adapters.

For outbound streams, response hook errors should stop the stream and return the mapped error.

## Decisions

- Generic interface hooks live only on generated adapters. Shared Tego transport options remain for
  reusable transport configuration such as error mapping and native Connect handler options.
- Generated adapters expose `AddServiceHooks`/`SetServiceHooks` for typed service hooks and
  `AddInterfaceHooks`/`SetInterfaceHooks` for grouped interface hooks, so setup code can either
  compose hooks or replace/reset them.
- Generated service hook structs and `tego.InterfaceHooks` expose per-slot `Add...Hook` and
  `Set...Hooks` helpers for ergonomic construction while keeping public fields available.
- All hook errors are mapped through the adapter error mapper. Response hooks only run after service
  execution and response mapping have succeeded up to that hook point, so there is no previously
  returned error for them to observe or map.
