//go:build !acceptance
// +build !acceptance

package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"

	neon "github.com/kislerdm/neon-sdk-go"
	"github.com/stretchr/testify/assert"
)

func Test_confirmDeleted(t *testing.T) {
	refused := neon.Error{HTTPCode: http.StatusUnprocessableEntity}
	notFound := neon.Error{HTTPCode: http.StatusNotFound}

	tests := map[string]struct {
		err  error
		get  error
		want error
	}{
		"nil passes through":                  {err: nil, get: nil, want: nil},
		"non-422 passes through":              {err: notFound, get: nil, want: notFound},
		"untyped error passes through":        {err: errors.New("boom"), get: notFound, want: errors.New("boom")},
		"422 kept when resource still exists": {err: refused, get: nil, want: refused},
		"422 kept when the check fails":       {err: refused, get: neon.Error{HTTPCode: http.StatusInternalServerError}, want: refused},
		"422 becomes 404 when resource gone":  {err: refused, get: notFound, want: notFound},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := confirmDeleted(tt.err, func() error { return tt.get })
			assert.Equal(t, tt.want, got)
		})
	}
}

type stubProjectDelete struct {
	sdkClientStub
	deleteErr error
	getErr    error
}

func (s *stubProjectDelete) DeleteProject(string) (neon.ProjectResponse, error) {
	return neon.ProjectResponse{}, s.deleteErr
}

func (s *stubProjectDelete) GetProject(string) (neon.ProjectResponse, error) {
	return neon.ProjectResponse{}, s.getErr
}

func Test_resourceProjectDeleteRetry(t *testing.T) {
	refused := neon.Error{HTTPCode: http.StatusUnprocessableEntity}
	refused.Message = "project has protected branch"

	t.Run("shall fail and keep the project in state when Neon refuses the delete", func(t *testing.T) {
		d := resourceProject().TestResourceData()
		d.SetId("foo")

		diags := resourceProjectDeleteRetry(context.TODO(), d, &stubProjectDelete{deleteErr: refused})

		assert.True(t, diags.HasError())
		assert.Contains(t, diags[0].Summary, "project has protected branch")
		assert.Equal(t, "foo", d.Id())
	})

	t.Run("shall remove the project from state when the 422 is for a project already gone", func(t *testing.T) {
		d := resourceProject().TestResourceData()
		d.SetId("foo")

		diags := resourceProjectDeleteRetry(context.TODO(), d, &stubProjectDelete{
			deleteErr: refused,
			getErr:    neon.Error{HTTPCode: http.StatusNotFound},
		})

		assert.False(t, diags.HasError())
		assert.Empty(t, d.Id())
	})
}
