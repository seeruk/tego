package tego

import "context"

// RPCInfo identifies the RPC currently being adapted.
type RPCInfo struct {
	Service   string
	Method    string
	Procedure string
}

// BeforeRequestMappingInterfaceHookFunc applies to protobuf requests before request mapping.
type BeforeRequestMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (context.Context, bool, error)
}

// AfterRequestMappingInterfaceHookFunc applies to Tego requests after request mapping.
type AfterRequestMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (context.Context, bool, error)
}

// BeforeResponseMappingInterfaceHookFunc applies to Tego responses before response mapping.
type BeforeResponseMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (bool, error)
}

// AfterResponseMappingInterfaceHookFunc applies to protobuf responses after response mapping.
type AfterResponseMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (bool, error)
}

// InterfaceHooks groups reusable hooks matched by Go interface implementation.
type InterfaceHooks struct {
	BeforeRequestMapping  []BeforeRequestMappingInterfaceHookFunc
	AfterRequestMapping   []AfterRequestMappingInterfaceHookFunc
	BeforeResponseMapping []BeforeResponseMappingInterfaceHookFunc
	AfterResponseMapping  []AfterResponseMappingInterfaceHookFunc
}

// MergeInterfaceHooks appends matching hook slots from all hook groups.
func MergeInterfaceHooks(hooks ...InterfaceHooks) InterfaceHooks {
	var merged InterfaceHooks
	for _, hooks := range hooks {
		merged.BeforeRequestMapping = append(merged.BeforeRequestMapping, hooks.BeforeRequestMapping...)
		merged.AfterRequestMapping = append(merged.AfterRequestMapping, hooks.AfterRequestMapping...)
		merged.BeforeResponseMapping = append(merged.BeforeResponseMapping, hooks.BeforeResponseMapping...)
		merged.AfterResponseMapping = append(merged.AfterResponseMapping, hooks.AfterResponseMapping...)
	}
	return merged
}

// AddBeforeRequestMappingHook appends hooks for protobuf requests before request mapping.
func (h *InterfaceHooks) AddBeforeRequestMappingHook(hooks ...BeforeRequestMappingInterfaceHookFunc) *InterfaceHooks {
	h.BeforeRequestMapping = append(h.BeforeRequestMapping, hooks...)
	return h
}

// SetBeforeRequestMappingHooks replaces hooks for protobuf requests before request mapping.
func (h *InterfaceHooks) SetBeforeRequestMappingHooks(hooks ...BeforeRequestMappingInterfaceHookFunc) *InterfaceHooks {
	h.BeforeRequestMapping = hooks
	return h
}

// AddAfterRequestMappingHook appends hooks for Tego requests after request mapping.
func (h *InterfaceHooks) AddAfterRequestMappingHook(hooks ...AfterRequestMappingInterfaceHookFunc) *InterfaceHooks {
	h.AfterRequestMapping = append(h.AfterRequestMapping, hooks...)
	return h
}

// SetAfterRequestMappingHooks replaces hooks for Tego requests after request mapping.
func (h *InterfaceHooks) SetAfterRequestMappingHooks(hooks ...AfterRequestMappingInterfaceHookFunc) *InterfaceHooks {
	h.AfterRequestMapping = hooks
	return h
}

// AddBeforeResponseMappingHook appends hooks for Tego responses before response mapping.
func (h *InterfaceHooks) AddBeforeResponseMappingHook(hooks ...BeforeResponseMappingInterfaceHookFunc) *InterfaceHooks {
	h.BeforeResponseMapping = append(h.BeforeResponseMapping, hooks...)
	return h
}

// SetBeforeResponseMappingHooks replaces hooks for Tego responses before response mapping.
func (h *InterfaceHooks) SetBeforeResponseMappingHooks(hooks ...BeforeResponseMappingInterfaceHookFunc) *InterfaceHooks {
	h.BeforeResponseMapping = hooks
	return h
}

// AddAfterResponseMappingHook appends hooks for protobuf responses after response mapping.
func (h *InterfaceHooks) AddAfterResponseMappingHook(hooks ...AfterResponseMappingInterfaceHookFunc) *InterfaceHooks {
	h.AfterResponseMapping = append(h.AfterResponseMapping, hooks...)
	return h
}

// SetAfterResponseMappingHooks replaces hooks for protobuf responses after response mapping.
func (h *InterfaceHooks) SetAfterResponseMappingHooks(hooks ...AfterResponseMappingInterfaceHookFunc) *InterfaceHooks {
	h.AfterResponseMapping = hooks
	return h
}

// BeforeRequestMappingAnyHook adapts a hook that applies to every protobuf request before request mapping.
func BeforeRequestMappingAnyHook(
	hook func(context.Context, RPCInfo, any) (context.Context, error),
) BeforeRequestMappingInterfaceHookFunc {
	return BeforeRequestMappingInterfaceHook[any](hook)
}

// BeforeRequestMappingInterfaceHook adapts a typed interface hook for protobuf requests.
func BeforeRequestMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) (context.Context, error),
) BeforeRequestMappingInterfaceHookFunc {
	return BeforeRequestMappingInterfaceHookFunc{
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

// AfterRequestMappingAnyHook adapts a hook that applies to every Tego request after request mapping.
func AfterRequestMappingAnyHook(
	hook func(context.Context, RPCInfo, any) (context.Context, error),
) AfterRequestMappingInterfaceHookFunc {
	return AfterRequestMappingInterfaceHook[any](hook)
}

// AfterRequestMappingInterfaceHook adapts a typed interface hook for Tego requests.
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

// BeforeResponseMappingAnyHook adapts a hook that applies to every Tego response before response mapping.
func BeforeResponseMappingAnyHook(
	hook func(context.Context, RPCInfo, any) error,
) BeforeResponseMappingInterfaceHookFunc {
	return BeforeResponseMappingInterfaceHook[any](hook)
}

// BeforeResponseMappingInterfaceHook adapts a typed interface hook for Tego responses.
func BeforeResponseMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) error,
) BeforeResponseMappingInterfaceHookFunc {
	return BeforeResponseMappingInterfaceHookFunc{
		run: func(ctx context.Context, info RPCInfo, value any) (bool, error) {
			typed, ok := value.(I)
			if !ok {
				return false, nil
			}
			return true, hook(ctx, info, typed)
		},
	}
}

// AfterResponseMappingAnyHook adapts a hook that applies to every protobuf response after response mapping.
func AfterResponseMappingAnyHook(
	hook func(context.Context, RPCInfo, any) error,
) AfterResponseMappingInterfaceHookFunc {
	return AfterResponseMappingInterfaceHook[any](hook)
}

// AfterResponseMappingInterfaceHook adapts a typed interface hook for protobuf responses.
func AfterResponseMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) error,
) AfterResponseMappingInterfaceHookFunc {
	return AfterResponseMappingInterfaceHookFunc{
		run: func(ctx context.Context, info RPCInfo, value any) (bool, error) {
			typed, ok := value.(I)
			if !ok {
				return false, nil
			}
			return true, hook(ctx, info, typed)
		},
	}
}

// RunBeforeRequestMappingInterfaceHooks runs matching before-request mapping interface hooks.
func RunBeforeRequestMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []BeforeRequestMappingInterfaceHookFunc,
) (context.Context, error) {
	for _, hook := range hooks {
		var err error
		ctx, err = runRequestInterfaceHook(ctx, info, value, hook.run)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// RunAfterRequestMappingInterfaceHooks runs matching after-request mapping interface hooks.
func RunAfterRequestMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []AfterRequestMappingInterfaceHookFunc,
) (context.Context, error) {
	for _, hook := range hooks {
		var err error
		ctx, err = runRequestInterfaceHook(ctx, info, value, hook.run)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// RunBeforeResponseMappingInterfaceHooks runs matching before-response mapping interface hooks.
func RunBeforeResponseMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []BeforeResponseMappingInterfaceHookFunc,
) error {
	for _, hook := range hooks {
		if err := runResponseInterfaceHook(ctx, info, value, hook.run); err != nil {
			return err
		}
	}
	return nil
}

// RunAfterResponseMappingInterfaceHooks runs matching after-response mapping interface hooks.
func RunAfterResponseMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []AfterResponseMappingInterfaceHookFunc,
) error {
	for _, hook := range hooks {
		if err := runResponseInterfaceHook(ctx, info, value, hook.run); err != nil {
			return err
		}
	}
	return nil
}

func runRequestInterfaceHook[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	run func(context.Context, RPCInfo, any) (context.Context, bool, error),
) (context.Context, error) {
	ctx, ok, err := run(ctx, info, value)
	if ok || err != nil {
		return ctx, err
	}
	ctx, _, err = run(ctx, info, &value)
	return ctx, err
}

func runResponseInterfaceHook[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	run func(context.Context, RPCInfo, any) (bool, error),
) error {
	ok, err := run(ctx, info, value)
	if ok || err != nil {
		return err
	}
	_, err = run(ctx, info, &value)
	return err
}
