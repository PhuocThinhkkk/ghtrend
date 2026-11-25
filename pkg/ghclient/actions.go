package ghclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// IsRepoStarred reports whether the authenticated user has starred owner/repo.
func IsRepoStarred(ctx context.Context, owner, repo string) (bool, error) {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	apiPath := fmt.Sprintf("/user/starred/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	if _, err := activeAPIClient().DoREST(ctx, http.MethodGet, apiPath, nil, "", nil); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// SetRepoStarred stars (star=true) or unstars (star=false) owner/repo for the authenticated user.
func SetRepoStarred(ctx context.Context, owner, repo string, star bool) error {
	if err := validateOwnerRepo(owner, repo); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	method := http.MethodDelete
	if star {
		method = http.MethodPut
	}

	apiPath := fmt.Sprintf("/user/starred/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	_, err := activeAPIClient().DoREST(ctx, method, apiPath, nil, "", nil)
	return err
}
