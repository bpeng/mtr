package main

import (

	"bytes"
	"database/sql"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/GeoNet/weft"
	"github.com/golang/protobuf/proto"
	"net/http"
	"strings"
	"time"
)



func (a *tagSearch) allProto(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{}, []string{}); !res.Ok {
		return res
	}

	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT tag FROM mtr.tag ORDER BY TAG ASC`); err != nil {
		return weft.InternalServerError(err)
	}
	defer rows.Close()

	var ts mtrpb.TagResult

	for rows.Next() {
		var t mtrpb.Tag

		if err = rows.Scan(&t.Tag); err != nil {
			return weft.InternalServerError(err)
		}

		ts.Result = append(ts.Result, &t)
	}

	var by []byte
	if by, err = proto.Marshal(&ts); err != nil {
		return weft.InternalServerError(err)
	}

	b.Write(by)

	h.Set("Content-Type", "application/x-protobuf")

	return &weft.StatusOK
}

func (a *tagSearch) singleProto(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{}, []string{}); !res.Ok {
		return res
	}

	if res := a.read(strings.TrimPrefix(r.URL.Path, "/tag/")); !res.Ok {
		return res
	}

	// Load tagged metrics, latency etc in parallel.
	c1 := a.fieldMetric()
	c2 := a.dataLatency()

	resFinal := &weft.StatusOK

	for res := range merge(c1, c2) {
		if !res.Ok {
			resFinal = res
		}
	}

	if !resFinal.Ok {
		return resFinal
	}

	var by []byte
	var err error
	if by, err = proto.Marshal(&a.tagResult); err != nil {
		return weft.InternalServerError(err)
	}

	b.Write(by)

	h.Set("Content-Type", "application/x-protobuf")

	return &weft.StatusOK
}

// call read first
func (a *tagSearch) fieldMetric() <-chan *weft.Result {
	out := make(chan *weft.Result)
	go func() {
		defer close(out)
		var err error
		var rows *sql.Rows

		if rows, err = dbR.Query(`SELECT deviceID, modelID, typeid, time, value, lower, upper
	 			  FROM field.metric_tag JOIN field.metric_summary USING (devicepk, typepk)
			          WHERE tagPK = $1
				`, a.pk); err != nil {
			out <- weft.InternalServerError(err)
			return
		}
		defer rows.Close()

		var tm time.Time

		for rows.Next() {
			var fmr mtrpb.FieldMetricSummary

			if err = rows.Scan(&fmr.DeviceID, &fmr.ModelID, &fmr.TypeID, &tm, &fmr.Value,
				&fmr.Lower, &fmr.Upper); err != nil {
				out <- weft.InternalServerError(err)
				return
			}

			fmr.Seconds = tm.Unix()

			a.tagResult.FieldMetric = append(a.tagResult.FieldMetric, &fmr)
		}

		out <- &weft.StatusOK
		return
	}()
	return out
}

// call read first
func (a *tagSearch) dataLatency() <-chan *weft.Result {
	out := make(chan *weft.Result)
	go func() {
		defer close(out)
		var err error
		var rows *sql.Rows

		if rows, err = dbR.Query(`SELECT siteID, typeID, time, mean, fifty, ninety, lower, upper
	 			  FROM data.latency_tag JOIN data.latency_summary USING (sitePK, typePK)
			          WHERE tagPK = $1
				`, a.pk); err != nil {
			out <- weft.InternalServerError(err)
			return
		}
		defer rows.Close()

		var tm time.Time

		for rows.Next() {

			var dls mtrpb.DataLatencySummary

			if err = rows.Scan(&dls.SiteID, &dls.TypeID, &tm, &dls.Mean, &dls.Fifty, &dls.Ninety,
				&dls.Lower, &dls.Upper); err != nil {
				out <- weft.InternalServerError(err)
				return
			}

			dls.Seconds = tm.Unix()

			a.tagResult.DataLatency = append(a.tagResult.DataLatency, &dls)
		}

		out <- &weft.StatusOK
		return
	}()
	return out
}
