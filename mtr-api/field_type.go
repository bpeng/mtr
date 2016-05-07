package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type fieldType struct {
	typePK int
	Scale  float64 // used to scale the stored metric for display
	Name   string
	Unit   string // display unit after the metric has been multiplied by scale.
}

var fieldTypes = map[string]fieldType{
	"voltage": {
		typePK: 1,
		Scale:  0.001,
		Name:   "Voltage",
		Unit:   "V",
	},
	"clock": {
		typePK: 2,
		Scale:  1.0,
		Name:   "Clock Quality",
		Unit:   "%",
	},
	"satellites": {
		typePK: 3,
		Scale:  1.0,
		Name:   "Satellites Tracked",
		Unit:   "n",
	},
	"conn": {
		typePK: 4,
		Scale:  0.001,
		Name:   "Connectivity",
		Unit:   "ms",
	},
	"ping": {
		typePK: 5,
		Scale:  0.001,
		Name:   "ping",
		Unit:   "ms",
	},

	"disk.hd1": {
		typePK: 6,
		Scale:  1.0,
		Name:   "disk hd1",
		Unit:   "%",
	},
	"disk.hd2": {
		typePK: 7,
		Scale:  1.0,
		Name:   "disk hd2",
		Unit:   "%",
	},
	"disk.hd3": {
		typePK: 8,
		Scale:  1.0,
		Name:   "disk hd3",
		Unit:   "%",
	},
	"disk.hd4": {
		typePK: 9,
		Scale:  1.0,
		Name:   "disk hd4",
		Unit:   "%",
	},

	"centre": {
		typePK: 10,
		Scale:  1.0,
		Name:   "centre",
		Unit:   "mV",
	},

	"rf.signal": {
		typePK: 11,
		Scale:  1.0,
		Name:   "rf signal",
		Unit:   "dB",
	},
	"rf.noise": {
		typePK: 12,
		Scale:  1.0,
		Name:   "rf noise",
		Unit:   "dB",
	},
}

func loadFieldType(typeID string) (fieldType, *result) {

	if f, ok := fieldTypes[typeID]; ok {
		return f, &statusOK
	}

	return fieldType{}, badRequest("invalid type " + typeID)
}

func (f *fieldType) loadPK(r *http.Request) *result {
	var t fieldType
	var res *result

	if t, res = loadFieldType(r.URL.Query().Get("typeID")); !res.ok {
		return res
	}

	f.typePK = t.typePK

	return &statusOK
}

func (f *fieldType) jsonV1(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	if by, err := json.Marshal(fieldTypes); err == nil {
		b.Write(by)
	} else {
		return internalServerError(err)
	}

	h.Set("Content-Type", "application/json;version=1")

	return &statusOK
}
