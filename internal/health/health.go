package health

import (
	"context"
	"net/http"
	"time"
)

func Check(ctx context.Context, url string) error {
	client := http.Client{Timeout: 5 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return &StatusError{Code: response.StatusCode}
	}
	return nil
}

type StatusError struct{ Code int }

func (e *StatusError) Error() string { return http.StatusText(e.Code) }
