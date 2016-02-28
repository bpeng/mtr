package main

import (
	"bytes"
	"log"
	"net/http"
	"strings"
)

type result struct {
	ok   bool   // set true to indicated success
	code int    // http status code for writing back the client e.g., http.StatusOK for success.
	msg  string // any error message for logging or to send to the client.
}

/*
requestHandler for handling http requests.  The response for the request
should be written into.  Any header values for the client can be set in h
e.g., Content-Type.
*/
type requestHandler func(r *http.Request, h http.Header, b *bytes.Buffer) *result

var (
	statusOK         = result{ok: true, code: http.StatusOK, msg: ""}
	methodNotAllowed = result{ok: false, code: http.StatusMethodNotAllowed, msg: "method not allowed"}
	notFound         = result{ok: false, code: http.StatusNotFound, msg: ""}
	notAcceptable    = result{ok: false, code: http.StatusNotAcceptable, msg: "specify accept"}
)

func internalServerError(err error) *result {
	return &result{ok: false, code: http.StatusInternalServerError, msg: err.Error()}
}

func badRequest(message string) *result {
	return &result{ok: false, code: http.StatusBadRequest, msg: message}
}

/*
checkQuery inspects r and makes sure all required query parameters
are present and that no more than the required and optional parameters
are present.
*/
func checkQuery(r *http.Request, required, optional []string) *result {
	if strings.Contains(r.URL.Path, ";") {
		return badRequest("cache buster")
	}

	v := r.URL.Query()

	if len(required) == 0 && len(optional) == 0 {
		if len(v) == 0 {
			return &statusOK
		} else {
			return badRequest("found unexpected query parameters")
		}
	}

	var missing []string

	for _, k := range required {
		if v.Get(k) == "" {
			missing = append(missing, k)
		} else {
			v.Del(k)
		}
	}

	switch len(missing) {
	case 0:
	case 1:
		return badRequest("missing required query parameter: " + missing[0])
	default:
		return badRequest("missing required query parameters: " + strings.Join(missing, ", "))
	}

	for _, k := range optional {
		v.Del(k)
	}

	if len(v) > 0 {
		badRequest("found additional query parameters")
	}

	return &statusOK
}

/*
toHandler adds basic auth to f and returns a handler.
*/
func toHandler(f requestHandler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT", "DELETE":
			if user, password, ok := r.BasicAuth(); ok && userW == user && keyW == password {
				// PUT and DELETE do not have a response body for the client so pass a nil buffer.
				res := f(r, w.Header(), nil)

				switch res.code {
				case http.StatusOK:
					w.WriteHeader(http.StatusOK)
				case http.StatusInternalServerError:
					http.Error(w, res.msg, res.code)
					log.Printf("500 serving %s %s", r.URL, res.msg)
				default:
					http.Error(w, res.msg, res.code)
				}

				return
			} else {
				http.Error(w, "Access denied", http.StatusUnauthorized)
				return
			}
		case "GET":
			if user, password, ok := r.BasicAuth(); ok && userR == user && keyR == password {
				var b bytes.Buffer
				res := f(r, w.Header(), &b)

				switch res.code {
				case http.StatusOK:
					b.WriteTo(w)
				case http.StatusInternalServerError:
					http.Error(w, res.msg, res.code)
					log.Printf("500 serving %s %s", r.URL, res.msg)
				default:
					http.Error(w, res.msg, res.code)
				}

				return
			} else {
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GeoNet MTR\"")
				http.Error(w, "Access denied", http.StatusUnauthorized)
				return
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
