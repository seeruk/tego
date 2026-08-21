package tego

import "context"

// RPCInfo identifies the RPC currently being adapted.
type RPCInfo struct {
	Service   string
	Method    string
	Procedure string
}

// PreRequestMappingInterfaceHookFunc applies to protobuf requests before request mapping.
type PreRequestMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (context.Context, bool, error)
}

// PostRequestMappingInterfaceHookFunc applies to Tego requests after request mapping.
type PostRequestMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (context.Context, bool, error)
}

// PreResponseMappingInterfaceHookFunc applies to Tego responses before response mapping.
type PreResponseMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (bool, error)
}

// PostResponseMappingInterfaceHookFunc applies to protobuf responses after response mapping.
type PostResponseMappingInterfaceHookFunc struct {
	run func(context.Context, RPCInfo, any) (bool, error)
}

// InterfaceHooks groups reusable hooks matched by Go interface implementation.
type InterfaceHooks struct {
	PreRequestMapping   []PreRequestMappingInterfaceHookFunc
	PostRequestMapping  []PostRequestMappingInterfaceHookFunc
	PreResponseMapping  []PreResponseMappingInterfaceHookFunc
	PostResponseMapping []PostResponseMappingInterfaceHookFunc
}

// MergeInterfaceHooks appends matching hook slots from all hook groups.
func MergeInterfaceHooks(hooks ...InterfaceHooks) InterfaceHooks {
	var merged InterfaceHooks
	for _, hooks := range hooks {
		merged.PreRequestMapping = append(merged.PreRequestMapping, hooks.PreRequestMapping...)
		merged.PostRequestMapping = append(merged.PostRequestMapping, hooks.PostRequestMapping...)
		merged.PreResponseMapping = append(merged.PreResponseMapping, hooks.PreResponseMapping...)
		merged.PostResponseMapping = append(merged.PostResponseMapping, hooks.PostResponseMapping...)
	}
	return merged
}

// AddPreRequestMappingHook appends hooks for protobuf requests before request mapping.
func (ih *InterfaceHooks) AddPreRequestMappingHook[I any](
	hooks ...func(context.Context, RPCInfo, I) (context.Context, error),
) {
	for _, hook := range hooks {
		ih.PreRequestMapping = append(ih.PreRequestMapping, PreRequestMappingInterfaceHook(hook))
	}
}

// SetPreRequestMappingHooks replaces hooks for protobuf requests before request mapping.
func (ih *InterfaceHooks) SetPreRequestMappingHooks[I any](
	hooks ...func(context.Context, RPCInfo, I) (context.Context, error),
) {
	ih.PreRequestMapping = nil
	ih.AddPreRequestMappingHook(hooks...)
}

// AddPostRequestMappingHook appends hooks for Tego requests after request mapping.
func (ih *InterfaceHooks) AddPostRequestMappingHook[I any](
	hooks ...func(context.Context, RPCInfo, I) (context.Context, error),
) {
	for _, hook := range hooks {
		ih.PostRequestMapping = append(ih.PostRequestMapping, PostRequestMappingInterfaceHook(hook))
	}
}

// SetPostRequestMappingHooks replaces hooks for Tego requests after request mapping.
func (ih *InterfaceHooks) SetPostRequestMappingHooks[I any](
	hooks ...func(context.Context, RPCInfo, I) (context.Context, error),
) {
	ih.PostRequestMapping = nil
	ih.AddPostRequestMappingHook(hooks...)
}

// AddPreResponseMappingHook appends hooks for Tego responses before response mapping.
func (ih *InterfaceHooks) AddPreResponseMappingHook[I any](
	hooks ...func(context.Context, RPCInfo, I) error,
) {
	for _, hook := range hooks {
		ih.PreResponseMapping = append(ih.PreResponseMapping, PreResponseMappingInterfaceHook(hook))
	}
}

// SetPreResponseMappingHooks replaces hooks for Tego responses before response mapping.
func (ih *InterfaceHooks) SetPreResponseMappingHooks[I any](
	hooks ...func(context.Context, RPCInfo, I) error,
) {
	ih.PreResponseMapping = nil
	ih.AddPreResponseMappingHook(hooks...)
}

// AddPostResponseMappingHook appends hooks for protobuf responses after response mapping.
func (ih *InterfaceHooks) AddPostResponseMappingHook[I any](
	hooks ...func(context.Context, RPCInfo, I) error,
) {
	for _, hook := range hooks {
		ih.PostResponseMapping = append(ih.PostResponseMapping, PostResponseMappingInterfaceHook(hook))
	}
}

// SetPostResponseMappingHooks replaces hooks for protobuf responses after response mapping.
func (ih *InterfaceHooks) SetPostResponseMappingHooks[I any](
	hooks ...func(context.Context, RPCInfo, I) error,
) {
	ih.PostResponseMapping = nil
	ih.AddPostResponseMappingHook(hooks...)
}

// PreRequestMappingAnyHook adapts a hook that applies to every protobuf request before request mapping.
func PreRequestMappingAnyHook(
	hook func(context.Context, RPCInfo, any) (context.Context, error),
) PreRequestMappingInterfaceHookFunc {
	return PreRequestMappingInterfaceHook[any](hook)
}

// PreRequestMappingInterfaceHook adapts a typed interface hook for protobuf requests.
func PreRequestMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) (context.Context, error),
) PreRequestMappingInterfaceHookFunc {
	return PreRequestMappingInterfaceHookFunc{
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

// PostRequestMappingAnyHook adapts a hook that applies to every Tego request after request mapping.
func PostRequestMappingAnyHook(
	hook func(context.Context, RPCInfo, any) (context.Context, error),
) PostRequestMappingInterfaceHookFunc {
	return PostRequestMappingInterfaceHook[any](hook)
}

// PostRequestMappingInterfaceHook adapts a typed interface hook for Tego requests.
func PostRequestMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) (context.Context, error),
) PostRequestMappingInterfaceHookFunc {
	return PostRequestMappingInterfaceHookFunc{
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

// PreResponseMappingAnyHook adapts a hook that applies to every Tego response before response mapping.
func PreResponseMappingAnyHook(
	hook func(context.Context, RPCInfo, any) error,
) PreResponseMappingInterfaceHookFunc {
	return PreResponseMappingInterfaceHook[any](hook)
}

// PreResponseMappingInterfaceHook adapts a typed interface hook for Tego responses.
func PreResponseMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) error,
) PreResponseMappingInterfaceHookFunc {
	return PreResponseMappingInterfaceHookFunc{
		run: func(ctx context.Context, info RPCInfo, value any) (bool, error) {
			typed, ok := value.(I)
			if !ok {
				return false, nil
			}
			return true, hook(ctx, info, typed)
		},
	}
}

// PostResponseMappingAnyHook adapts a hook that applies to every protobuf response after response mapping.
func PostResponseMappingAnyHook(
	hook func(context.Context, RPCInfo, any) error,
) PostResponseMappingInterfaceHookFunc {
	return PostResponseMappingInterfaceHook[any](hook)
}

// PostResponseMappingInterfaceHook adapts a typed interface hook for protobuf responses.
func PostResponseMappingInterfaceHook[I any](
	hook func(context.Context, RPCInfo, I) error,
) PostResponseMappingInterfaceHookFunc {
	return PostResponseMappingInterfaceHookFunc{
		run: func(ctx context.Context, info RPCInfo, value any) (bool, error) {
			typed, ok := value.(I)
			if !ok {
				return false, nil
			}
			return true, hook(ctx, info, typed)
		},
	}
}

// RunPreRequestMappingInterfaceHooks runs matching before-request mapping interface hooks.
func RunPreRequestMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []PreRequestMappingInterfaceHookFunc,
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

// RunPostRequestMappingInterfaceHooks runs matching after-request mapping interface hooks.
func RunPostRequestMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []PostRequestMappingInterfaceHookFunc,
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

// RunPreResponseMappingInterfaceHooks runs matching before-response mapping interface hooks.
func RunPreResponseMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []PreResponseMappingInterfaceHookFunc,
) error {
	for _, hook := range hooks {
		if err := runResponseInterfaceHook(ctx, info, value, hook.run); err != nil {
			return err
		}
	}
	return nil
}

// RunPostResponseMappingInterfaceHooks runs matching after-response mapping interface hooks.
func RunPostResponseMappingInterfaceHooks[T any](
	ctx context.Context,
	info RPCInfo,
	value T,
	hooks []PostResponseMappingInterfaceHookFunc,
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
