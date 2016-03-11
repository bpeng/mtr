package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/GeoNet/map180"
	"math"
	"net/http"
	"strconv"
	"time"
)

type fieldLatest struct {
	typeID string
}

type point struct {
	latitude, longitude float64
	x, y                float64
}

func (f *fieldLatest) jsonV1(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	if res := checkQuery(r, []string{}, []string{"typeID"}); !res.ok {
		return res
	}

	f.typeID = r.URL.Query().Get("typeID")

	var s string
	var err error

	switch f.typeID {
	case "":
		err = dbR.QueryRow(`SELECT COALESCE(array_to_json(array_agg(row_to_json(l))), '[]') 
			FROM (
				SELECT latitude AS "Latitude", longitude AS "Longitude", 
				deviceID AS "DeviceID", time AS "Time", avg AS "Value",
				typeID AS "TypeID",
				lower as "Lower",
				upper as "Upper"
				FROM field.metric_summary_hour) l`).Scan(&s)
	default:
		err = dbR.QueryRow(`SELECT COALESCE(array_to_json(array_agg(row_to_json(l))), '[]') 
			FROM (
				SELECT latitude AS "Latitude", longitude AS "Longitude", 
				deviceID AS "DeviceID", time AS "Time", avg AS "Value",
				typeID AS "TypeID",
				lower as "Lower",
				upper as "Upper"
				FROM field.metric_summary_hour where typeID = $1) l`, f.typeID).Scan(&s)
	}
	if err != nil {
		return internalServerError(err)
	}

	b.WriteString(s)

	h.Set("Content-Type", "application/json;version=1")

	return &statusOK
}

func (f *fieldLatest) svg(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	if res := checkQuery(r, []string{"bbox", "width"}, []string{"typeID"}); !res.ok {
		return res
	}

	var rows *sql.Rows
	var width int
	var err error

	f.typeID = r.URL.Query().Get("typeID")
	bbox := r.URL.Query().Get("bbox")

	if err = map180.ValidBbox(bbox); err != nil {
		return badRequest(err.Error())
	}

	if width, err = strconv.Atoi(r.URL.Query().Get("width")); err != nil {
		return badRequest("invalid width")
	}

	var raw map180.Raw
	if raw, err = wm.MapRaw(bbox, width); err != nil {
		return internalServerError(err)
	}

	switch f.typeID {
	case "":
		rows, err = dbR.Query(`with p as (select longitude,latitude, time, avg, lower,upper, 
			st_transform(geom::geometry, 3857) as pt
			from field.metric_summary_hour)
			select ST_X(pt), ST_Y(pt)*-1, longitude,latitude, time, avg, lower,upper from p`)
	default:
		rows, err = dbR.Query(`with p as (select longitude,latitude, time, avg, lower,upper,
			st_transform(geom::geometry, 3857) as pt
			from field.metric_summary_hour where typeID = $1)
			select ST_X(pt), ST_Y(pt)*-1, longitude,latitude, time, avg, lower,upper from p`, f.typeID)
	}
	if err != nil {
		return internalServerError(err)
	}
	defer rows.Close()

	ago := time.Now().UTC().Add(time.Hour * -3)

	var late []point
	var good []point
	var bad []point
	var dunno []point

	for rows.Next() {
		var p point
		var t time.Time
		var min, max, v int

		if err = rows.Scan(&p.x, &p.y, &p.longitude, &p.latitude, &t, &v, &min, &max); err != nil {
			return internalServerError(err)
		}

		switch raw.CrossesCentral && p.longitude > -180.0 && p.longitude < 0.0 {
		case true:
			p.x = (p.x + map180.Width3857 - raw.LLX) * raw.DX
			p.y = (p.y - math.Abs(raw.YShift)) * raw.DX
		case false:
			p.x = (p.x - math.Abs(raw.XShift)) * raw.DX
			p.y = (p.y - math.Abs(raw.YShift)) * raw.DX

		}
		switch {
		case t.Before(ago):
			late = append(late, p)
		case min == 0 && max == 0:
			dunno = append(dunno, p)
		case v < min || v > max:
			bad = append(bad, p)
		default:
			good = append(good, p)
		}
	}
	rows.Close()

	b.WriteString(`<?xml version="1.0"?>`)
	b.WriteString(fmt.Sprintf("<svg  viewBox=\"0 0 %d %d\"  xmlns=\"http://www.w3.org/2000/svg\">",
		raw.Width, raw.Height))
	b.WriteString(fmt.Sprintf("<rect x=\"0\" y=\"0\" width=\"%d\" height=\"%d\" style=\"fill: azure\"/>", raw.Width, raw.Height))
	b.WriteString(fmt.Sprintf("<path style=\"fill: wheat; stroke-width: 1; stroke-linejoin: round; stroke: lightslategrey\" d=\"%s\"/>", raw.Land))
	b.WriteString(fmt.Sprintf("<path style=\"fill: azure; stroke-width: 1; stroke-linejoin: round; stroke: lightslategrey\" d=\"%s\"/>", raw.Lakes))

	b.WriteString("<g style=\"stroke: #377eb8; fill: #377eb8; \">") // blueish
	for _, p := range dunno {
		b.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"%d\"/>", p.x, p.y, 5))
	}
	b.WriteString("</g>")

	b.WriteString("<g style=\"stroke: #4daf4a; fill: #4daf4a; \">") // greenish
	for _, p := range good {
		b.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"%d\"/>", p.x, p.y, 5))
	}
	b.WriteString("</g>")

	b.WriteString("<g style=\"stroke: #e41a1c; fill: #e41a1c; \">") //red
	for _, p := range bad {
		b.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"%d\"/>", p.x, p.y, 6))
	}
	b.WriteString("</g>")

	b.WriteString("<g style=\"stroke: #984ea3; fill: #984ea3; \">") // purple
	for _, p := range late {
		b.WriteString(fmt.Sprintf("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"%d\"/>", p.x, p.y, 6))
	}
	b.WriteString("</g>")

	b.WriteString("</svg>")

	h.Set("Content-Type", "image/svg+xml")

	return &statusOK
}
