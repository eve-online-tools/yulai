package esi

import (
	"errors"
	"net/http"
	"testing"

	"github.com/eve-online-tools/lib-esi-go/request"
)

type out struct{ V int }

func TestResponseError(t *testing.T) {
	ok := &request.Response[*out]{Response: &http.Response{StatusCode: 200}, Data: &out{1}}
	if err := ResponseError(ok); err != nil {
		t.Fatalf("2xx with data: %v", err)
	}

	empty := &request.Response[*out]{Response: &http.Response{StatusCode: 200}}
	var e *Error
	if err := ResponseError(empty); !errors.As(err, &e) || e.Status != 200 {
		t.Fatalf("2xx without data: %v", err)
	}

	notFound := &request.Response[*out]{Response: &http.Response{StatusCode: 404, Status: "404 Not Found"}}
	if err := ResponseError(notFound); !errors.As(err, &e) || e.Status != 404 {
		t.Fatalf("404: %v", err)
	}

	values := &request.Response[[]int]{Response: &http.Response{StatusCode: 200}}
	if err := ResponseError(values); err != nil {
		t.Fatalf("2xx with a non-pointer type: %v", err)
	}
}
