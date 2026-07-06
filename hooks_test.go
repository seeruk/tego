package tego

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hookValidator interface {
	Validate() error
}

type hookValueValidated struct {
	err error
}

func (v hookValueValidated) Validate() error {
	return v.err
}

type hookPointerValidated struct {
	err error
}

func (v *hookPointerValidated) Validate() error {
	return v.err
}

type hookFinalizer interface {
	Finalize() error
}

type hookFinalized struct {
	value string
}

func (v *hookFinalized) Finalize() error {
	v.value = "finalized"
	return nil
}

func TestMergeInterfaceHooks(t *testing.T) {
	beforeRequest := BeforeRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	afterRequest := AfterRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	beforeResponse := BeforeResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})
	afterResponse := AfterResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})

	merged := MergeInterfaceHooks(
		InterfaceHooks{
			BeforeRequestMapping:  []BeforeRequestMappingInterfaceHookFunc{beforeRequest},
			BeforeResponseMapping: []BeforeResponseMappingInterfaceHookFunc{beforeResponse},
		},
		InterfaceHooks{
			AfterRequestMapping:  []AfterRequestMappingInterfaceHookFunc{afterRequest},
			AfterResponseMapping: []AfterResponseMappingInterfaceHookFunc{afterResponse},
		},
	)

	assert.Len(t, merged.BeforeRequestMapping, 1)
	assert.Len(t, merged.AfterRequestMapping, 1)
	assert.Len(t, merged.BeforeResponseMapping, 1)
	assert.Len(t, merged.AfterResponseMapping, 1)
}

func TestInterfaceHookHelpers(t *testing.T) {
	beforeRequest := BeforeRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	anotherBeforeRequest := BeforeRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	afterRequest := AfterRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	beforeResponse := BeforeResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})
	afterResponse := AfterResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})

	var hooks InterfaceHooks
	returned := hooks.
		AddBeforeRequestMappingHook(beforeRequest, anotherBeforeRequest).
		AddAfterRequestMappingHook(afterRequest).
		AddBeforeResponseMappingHook(beforeResponse).
		AddAfterResponseMappingHook(afterResponse)

	require.Same(t, &hooks, returned)
	assert.Len(t, hooks.BeforeRequestMapping, 2)
	assert.Len(t, hooks.AfterRequestMapping, 1)
	assert.Len(t, hooks.BeforeResponseMapping, 1)
	assert.Len(t, hooks.AfterResponseMapping, 1)

	hooks.SetBeforeRequestMappingHooks(beforeRequest)
	assert.Len(t, hooks.BeforeRequestMapping, 1)

	hooks.SetBeforeRequestMappingHooks()
	hooks.SetAfterRequestMappingHooks()
	hooks.SetBeforeResponseMappingHooks()
	hooks.SetAfterResponseMappingHooks()
	assert.Empty(t, hooks.BeforeRequestMapping)
	assert.Empty(t, hooks.AfterRequestMapping)
	assert.Empty(t, hooks.BeforeResponseMapping)
	assert.Empty(t, hooks.AfterResponseMapping)
}

func TestRequestMappingInterfaceHooks(t *testing.T) {
	t.Run("matches value receiver interfaces", func(t *testing.T) {
		var called bool
		hook := AfterRequestMappingInterfaceHook(func(ctx context.Context, info RPCInfo, value hookValidator) (context.Context, error) {
			called = true
			assert.Equal(t, "books.v1.BookService", info.Service)
			return context.WithValue(ctx, contextKey("validated"), true), value.Validate()
		})

		ctx, err := RunAfterRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{Service: "books.v1.BookService"},
			hookValueValidated{},
			[]AfterRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, true, ctx.Value(contextKey("validated")))
	})

	t.Run("matches pointer receiver interfaces", func(t *testing.T) {
		var called bool
		hook := BeforeRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
			called = true
			return ctx, value.Validate()
		})

		ctx, err := RunBeforeRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookPointerValidated{},
			[]BeforeRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
		assert.NotNil(t, ctx)
	})

	t.Run("skips non matching values", func(t *testing.T) {
		var called bool
		hook := AfterRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
			called = true
			return ctx, nil
		})

		_, err := RunAfterRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"not validated",
			[]AfterRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("invalid")
		hook := BeforeRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
			return ctx, value.Validate()
		})

		_, err := RunBeforeRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookValueValidated{err: wantErr},
			[]BeforeRequestMappingInterfaceHookFunc{hook},
		)

		require.ErrorIs(t, err, wantErr)
	})
}

func TestAnyInterfaceHooks(t *testing.T) {
	t.Run("matches every mapping slot", func(t *testing.T) {
		var calls []string
		beforeRequest := BeforeRequestMappingAnyHook(func(ctx context.Context, _ RPCInfo, value any) (context.Context, error) {
			calls = append(calls, "before-request:"+value.(string))
			return context.WithValue(ctx, contextKey("before-request"), true), nil
		})
		afterRequest := AfterRequestMappingAnyHook(func(ctx context.Context, _ RPCInfo, value any) (context.Context, error) {
			calls = append(calls, "after-request:"+value.(string))
			return context.WithValue(ctx, contextKey("after-request"), true), nil
		})
		beforeResponse := BeforeResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
			calls = append(calls, "before-response:"+value.(string))
			return nil
		})
		afterResponse := AfterResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
			calls = append(calls, "after-response:"+value.(string))
			return nil
		})

		ctx, err := RunBeforeRequestMappingInterfaceHooks(context.Background(), RPCInfo{}, "value", []BeforeRequestMappingInterfaceHookFunc{beforeRequest})
		require.NoError(t, err)
		assert.Equal(t, true, ctx.Value(contextKey("before-request")))

		ctx, err = RunAfterRequestMappingInterfaceHooks(ctx, RPCInfo{}, "value", []AfterRequestMappingInterfaceHookFunc{afterRequest})
		require.NoError(t, err)
		assert.Equal(t, true, ctx.Value(contextKey("after-request")))

		err = RunBeforeResponseMappingInterfaceHooks(ctx, RPCInfo{}, "value", []BeforeResponseMappingInterfaceHookFunc{beforeResponse})
		require.NoError(t, err)

		err = RunAfterResponseMappingInterfaceHooks(ctx, RPCInfo{}, "value", []AfterResponseMappingInterfaceHookFunc{afterResponse})
		require.NoError(t, err)

		assert.Equal(t, []string{
			"before-request:value",
			"after-request:value",
			"before-response:value",
			"after-response:value",
		}, calls)
	})

	t.Run("can check multiple interfaces inside one hook", func(t *testing.T) {
		var validated bool
		var finalized bool
		value := hookMultiPurpose{}
		hook := BeforeResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
			if validator, ok := value.(hookValidator); ok {
				validated = true
				if err := validator.Validate(); err != nil {
					return err
				}
			}
			if finalizer, ok := value.(hookFinalizer); ok {
				finalized = true
				return finalizer.Finalize()
			}
			return nil
		})

		err := RunBeforeResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			&value,
			[]BeforeResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, validated)
		assert.True(t, finalized)
		assert.Equal(t, "finalized", value.value)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("any hook failed")
		hook := AfterResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, _ any) error {
			return wantErr
		})

		err := RunAfterResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"value",
			[]AfterResponseMappingInterfaceHookFunc{hook},
		)

		require.ErrorIs(t, err, wantErr)
	})
}

func TestResponseMappingInterfaceHooks(t *testing.T) {
	t.Run("matches value receiver interfaces", func(t *testing.T) {
		var called bool
		hook := BeforeResponseMappingInterfaceHook(func(infoCtx context.Context, info RPCInfo, value hookValidator) error {
			called = true
			assert.Equal(t, "GetBook", info.Method)
			assert.Equal(t, true, infoCtx.Value(contextKey("response")))
			return value.Validate()
		})

		err := RunBeforeResponseMappingInterfaceHooks(
			context.WithValue(context.Background(), contextKey("response"), true),
			RPCInfo{Method: "GetBook"},
			hookValueValidated{},
			[]BeforeResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("matches pointer receiver interfaces", func(t *testing.T) {
		var called bool
		hook := AfterResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookValidator) error {
			called = true
			return value.Validate()
		})

		err := RunAfterResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookPointerValidated{},
			[]AfterResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("preserves pointer receiver mutations for addressable values", func(t *testing.T) {
		value := hookFinalized{}
		hook := BeforeResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookFinalizer) error {
			return value.Finalize()
		})

		err := RunBeforeResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			&value,
			[]BeforeResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.Equal(t, "finalized", value.value)
	})

	t.Run("skips non matching values", func(t *testing.T) {
		var called bool
		hook := BeforeResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
			called = true
			return nil
		})

		err := RunBeforeResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"not validated",
			[]BeforeResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("invalid")
		hook := AfterResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookValidator) error {
			return value.Validate()
		})

		err := RunAfterResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookValueValidated{err: wantErr},
			[]AfterResponseMappingInterfaceHookFunc{hook},
		)

		require.ErrorIs(t, err, wantErr)
	})
}

type contextKey string

type hookMultiPurpose struct {
	value string
}

func (v *hookMultiPurpose) Validate() error {
	return nil
}

func (v *hookMultiPurpose) Finalize() error {
	v.value = "finalized"
	return nil
}
