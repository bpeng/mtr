package main

import (
	"bytes"
	"net/http"
)

func appMetricHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var a appMetric

	switch r.Method {
	case "POST":
		return a.save(r)
	case "GET":
		switch r.Header.Get("Accept") {
		default:
			return a.svg(r, h, b)
		}
	default:
		return &methodNotAllowed
	}
}

func fieldMetricHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldMetric

	switch r.Method {
	case "PUT":
		return f.save(r)
	case "DELETE":
		return f.delete(r)
	case "GET":
		switch r.Header.Get("Accept") {
		default:
			return f.svg(r, h, b)
		}
	default:
		return &methodNotAllowed
	}
}

func fieldMetricTagHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldMetricTag

	switch r.Method {
	case "PUT":
		return f.save(r, h, b)
	case "DELETE":
		return f.delete(r, h, b)
	default:
		return &methodNotAllowed
	}
}

func tagHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var t tag

	switch r.Method {
	case "PUT":
		return t.save(r)
	case "DELETE":
		return t.delete(r)

	default:
		return &methodNotAllowed
	}
}

func fieldModelHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldModel

	switch r.Method {
	case "PUT":
		return f.save(r)
	case "DELETE":
		return f.delete(r)
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/json;version=1":
			return f.jsonV1(r, h, b)
		default:
			return &notAcceptable
		}
	default:
		return &methodNotAllowed
	}
}

func fieldDeviceHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldDevice

	switch r.Method {
	case "PUT":
		return f.save(r)
	case "DELETE":
		return f.delete(r)
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/json;version=1":
			return f.jsonV1(r, h, b)
		default:
			return &notAcceptable
		}
	default:
		return &methodNotAllowed
	}
}

func fieldTypeHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldType

	switch r.Method {
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/json;version=1":
			return f.jsonV1(r, h, b)
		default:
			return &notAcceptable
		}
	default:
		return &methodNotAllowed
	}
}

func fieldThresholdHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldThreshold

	switch r.Method {
	case "PUT":
		return f.save(r)
	case "DELETE":
		return f.delete(r)
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/json;version=1":
			return f.jsonV1(r, h, b)
		default:
			return &notAcceptable
		}
	default:
		return &methodNotAllowed
	}
}

func fieldMetricLatestHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f fieldLatest

	switch r.Method {
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/x-protobuf":
			return f.proto(r, h, b)
		default:
			return f.svg(r, h, b)
		}
	default:
		return &methodNotAllowed
	}
}

func dataSiteHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var d dataSite

	switch r.Method {
	case "PUT":
		return d.save(r)
	case "DELETE":
		return d.delete(r)
	default:
		return &methodNotAllowed
	}
}

func dataLatencyHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var d dataLatency

	switch r.Method {
	case "PUT":
		return d.save(r)
	case "DELETE":
		return d.delete(r)
	case "GET":
		switch r.Header.Get("Accept") {
		default:
			return d.svg(r, h, b)
		}
	default:
		return &methodNotAllowed
	}
}

func dataLatencyThresholdHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var d dataLatencyThreshold

	switch r.Method {
	case "PUT":
		return d.save(r)
	case "DELETE":
		return d.delete(r)
	//case "GET":
	//	switch r.Header.Get("Accept") {
	//	case "application/json;version=1":
	//		return f.jsonV1(r, h, b)
	//	default:
	//		return &notAcceptable
	//	}
	default:
		return &methodNotAllowed
	}
}

func dataLatencyTagHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var f dataLatencyTag

	switch r.Method {
	case "PUT":
		return f.save(r, h, b)
	case "DELETE":
		return f.delete(r, h, b)
	default:
		return &methodNotAllowed
	}
}

func dataLatencySummaryHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	var d dataLatencySummary

	switch r.Method {
	case "GET":
		switch r.Header.Get("Accept") {
		case "application/x-protobuf":
			return d.proto(r, h, b)
			//default:
			//	return f.svg(r, h, b)
		default:
			return &notAcceptable
		}
	default:
		return &methodNotAllowed
	}
}
