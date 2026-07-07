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
	beforeRequest := PreRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	afterRequest := PostRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	})
	beforeResponse := PreResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})
	afterResponse := PostResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	})

	merged := MergeInterfaceHooks(
		InterfaceHooks{
			PreRequestMapping:  []PreRequestMappingInterfaceHookFunc{beforeRequest},
			PreResponseMapping: []PreResponseMappingInterfaceHookFunc{beforeResponse},
		},
		InterfaceHooks{
			PostRequestMapping:  []PostRequestMappingInterfaceHookFunc{afterRequest},
			PostResponseMapping: []PostResponseMappingInterfaceHookFunc{afterResponse},
		},
	)

	assert.Len(t, merged.PreRequestMapping, 1)
	assert.Len(t, merged.PostRequestMapping, 1)
	assert.Len(t, merged.PreResponseMapping, 1)
	assert.Len(t, merged.PostResponseMapping, 1)
}

func TestInterfaceHookHelpers(t *testing.T) {
	var beforeRequestCalled bool
	beforeRequest := func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
		beforeRequestCalled = true
		return context.WithValue(ctx, contextKey("pre-request"), true), value.Validate()
	}
	anotherPreRequest := func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	}
	var afterRequestCalled bool
	afterRequest := func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
		afterRequestCalled = true
		return context.WithValue(ctx, contextKey("post-request"), true), value.Validate()
	}
	var beforeResponseCalled bool
	beforeResponse := func(_ context.Context, _ RPCInfo, value hookValidator) error {
		beforeResponseCalled = true
		return value.Validate()
	}
	var afterResponseCalled bool
	afterResponse := func(_ context.Context, _ RPCInfo, value hookValidator) error {
		afterResponseCalled = true
		return value.Validate()
	}

	var hooks InterfaceHooks
	AddPreRequestMappingHook(&hooks, beforeRequest, anotherPreRequest)
	AddPostRequestMappingHook(&hooks, afterRequest)
	AddPreResponseMappingHook(&hooks, beforeResponse)
	AddPostResponseMappingHook(&hooks, afterResponse)

	assert.Len(t, hooks.PreRequestMapping, 2)
	assert.Len(t, hooks.PostRequestMapping, 1)
	assert.Len(t, hooks.PreResponseMapping, 1)
	assert.Len(t, hooks.PostResponseMapping, 1)

	ctx, err := RunPreRequestMappingInterfaceHooks(context.Background(), RPCInfo{}, hookValueValidated{}, hooks.PreRequestMapping)
	require.NoError(t, err)
	assert.True(t, beforeRequestCalled)
	assert.Equal(t, true, ctx.Value(contextKey("pre-request")))

	ctx, err = RunPostRequestMappingInterfaceHooks(ctx, RPCInfo{}, hookValueValidated{}, hooks.PostRequestMapping)
	require.NoError(t, err)
	assert.True(t, afterRequestCalled)
	assert.Equal(t, true, ctx.Value(contextKey("post-request")))

	require.NoError(t, RunPreResponseMappingInterfaceHooks(ctx, RPCInfo{}, hookValueValidated{}, hooks.PreResponseMapping))
	assert.True(t, beforeResponseCalled)

	require.NoError(t, RunPostResponseMappingInterfaceHooks(ctx, RPCInfo{}, hookValueValidated{}, hooks.PostResponseMapping))
	assert.True(t, afterResponseCalled)

	replacementPreRequest := func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	}
	replacementPostRequest := func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
		return ctx, nil
	}
	replacementPreResponse := func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	}
	replacementPostResponse := func(_ context.Context, _ RPCInfo, _ hookValidator) error {
		return nil
	}

	SetPreRequestMappingHooks(&hooks, replacementPreRequest)
	SetPostRequestMappingHooks(&hooks, replacementPostRequest)
	SetPreResponseMappingHooks(&hooks, replacementPreResponse)
	SetPostResponseMappingHooks(&hooks, replacementPostResponse)
	assert.Len(t, hooks.PreRequestMapping, 1)
	assert.Len(t, hooks.PostRequestMapping, 1)
	assert.Len(t, hooks.PreResponseMapping, 1)
	assert.Len(t, hooks.PostResponseMapping, 1)

	SetPreRequestMappingHooks[hookValidator](&hooks)
	SetPostRequestMappingHooks[hookValidator](&hooks)
	SetPreResponseMappingHooks[hookValidator](&hooks)
	SetPostResponseMappingHooks[hookValidator](&hooks)
	assert.Empty(t, hooks.PreRequestMapping)
	assert.Empty(t, hooks.PostRequestMapping)
	assert.Empty(t, hooks.PreResponseMapping)
	assert.Empty(t, hooks.PostResponseMapping)
}

func TestRequestMappingInterfaceHooks(t *testing.T) {
	t.Run("matches value receiver interfaces", func(t *testing.T) {
		var called bool
		hook := PostRequestMappingInterfaceHook(func(ctx context.Context, info RPCInfo, value hookValidator) (context.Context, error) {
			called = true
			assert.Equal(t, "books.v1.BookService", info.Service)
			return context.WithValue(ctx, contextKey("validated"), true), value.Validate()
		})

		ctx, err := RunPostRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{Service: "books.v1.BookService"},
			hookValueValidated{},
			[]PostRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, true, ctx.Value(contextKey("validated")))
	})

	t.Run("matches pointer receiver interfaces", func(t *testing.T) {
		var called bool
		hook := PreRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
			called = true
			return ctx, value.Validate()
		})

		ctx, err := RunPreRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookPointerValidated{},
			[]PreRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
		assert.NotNil(t, ctx)
	})

	t.Run("skips non matching values", func(t *testing.T) {
		var called bool
		hook := PostRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, _ hookValidator) (context.Context, error) {
			called = true
			return ctx, nil
		})

		_, err := RunPostRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"not validated",
			[]PostRequestMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("invalid")
		hook := PreRequestMappingInterfaceHook(func(ctx context.Context, _ RPCInfo, value hookValidator) (context.Context, error) {
			return ctx, value.Validate()
		})

		_, err := RunPreRequestMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookValueValidated{err: wantErr},
			[]PreRequestMappingInterfaceHookFunc{hook},
		)

		require.ErrorIs(t, err, wantErr)
	})
}

func TestAnyInterfaceHooks(t *testing.T) {
	t.Run("matches every mapping slot", func(t *testing.T) {
		var calls []string
		beforeRequest := PreRequestMappingAnyHook(func(ctx context.Context, _ RPCInfo, value any) (context.Context, error) {
			calls = append(calls, "before-request:"+value.(string))
			return context.WithValue(ctx, contextKey("before-request"), true), nil
		})
		afterRequest := PostRequestMappingAnyHook(func(ctx context.Context, _ RPCInfo, value any) (context.Context, error) {
			calls = append(calls, "after-request:"+value.(string))
			return context.WithValue(ctx, contextKey("after-request"), true), nil
		})
		beforeResponse := PreResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
			calls = append(calls, "before-response:"+value.(string))
			return nil
		})
		afterResponse := PostResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
			calls = append(calls, "after-response:"+value.(string))
			return nil
		})

		ctx, err := RunPreRequestMappingInterfaceHooks(context.Background(), RPCInfo{}, "value", []PreRequestMappingInterfaceHookFunc{beforeRequest})
		require.NoError(t, err)
		assert.Equal(t, true, ctx.Value(contextKey("before-request")))

		ctx, err = RunPostRequestMappingInterfaceHooks(ctx, RPCInfo{}, "value", []PostRequestMappingInterfaceHookFunc{afterRequest})
		require.NoError(t, err)
		assert.Equal(t, true, ctx.Value(contextKey("after-request")))

		err = RunPreResponseMappingInterfaceHooks(ctx, RPCInfo{}, "value", []PreResponseMappingInterfaceHookFunc{beforeResponse})
		require.NoError(t, err)

		err = RunPostResponseMappingInterfaceHooks(ctx, RPCInfo{}, "value", []PostResponseMappingInterfaceHookFunc{afterResponse})
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
		hook := PreResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, value any) error {
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

		err := RunPreResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			&value,
			[]PreResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, validated)
		assert.True(t, finalized)
		assert.Equal(t, "finalized", value.value)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("any hook failed")
		hook := PostResponseMappingAnyHook(func(_ context.Context, _ RPCInfo, _ any) error {
			return wantErr
		})

		err := RunPostResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"value",
			[]PostResponseMappingInterfaceHookFunc{hook},
		)

		require.ErrorIs(t, err, wantErr)
	})
}

func TestResponseMappingInterfaceHooks(t *testing.T) {
	t.Run("matches value receiver interfaces", func(t *testing.T) {
		var called bool
		hook := PreResponseMappingInterfaceHook(func(infoCtx context.Context, info RPCInfo, value hookValidator) error {
			called = true
			assert.Equal(t, "GetBook", info.Method)
			assert.Equal(t, true, infoCtx.Value(contextKey("response")))
			return value.Validate()
		})

		err := RunPreResponseMappingInterfaceHooks(
			context.WithValue(context.Background(), contextKey("response"), true),
			RPCInfo{Method: "GetBook"},
			hookValueValidated{},
			[]PreResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("matches pointer receiver interfaces", func(t *testing.T) {
		var called bool
		hook := PostResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookValidator) error {
			called = true
			return value.Validate()
		})

		err := RunPostResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookPointerValidated{},
			[]PostResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("preserves pointer receiver mutations for addressable values", func(t *testing.T) {
		value := hookFinalized{}
		hook := PreResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookFinalizer) error {
			return value.Finalize()
		})

		err := RunPreResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			&value,
			[]PreResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.Equal(t, "finalized", value.value)
	})

	t.Run("skips non matching values", func(t *testing.T) {
		var called bool
		hook := PreResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, _ hookValidator) error {
			called = true
			return nil
		})

		err := RunPreResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			"not validated",
			[]PreResponseMappingInterfaceHookFunc{hook},
		)

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("returns hook errors", func(t *testing.T) {
		wantErr := errors.New("invalid")
		hook := PostResponseMappingInterfaceHook(func(_ context.Context, _ RPCInfo, value hookValidator) error {
			return value.Validate()
		})

		err := RunPostResponseMappingInterfaceHooks(
			context.Background(),
			RPCInfo{},
			hookValueValidated{err: wantErr},
			[]PostResponseMappingInterfaceHookFunc{hook},
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
