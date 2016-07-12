package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/GeoNet/mtr/internal"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/GeoNet/mtr/ts"
	"github.com/GeoNet/weft"
	"github.com/golang/protobuf/proto"
	"github.com/lib/pq"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// dataLatency - table data.latency
type dataLatency struct{}

func (a dataLatency) put(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID", "time", "mean"}, []string{"min", "max", "fifty", "ninety"}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	var err error

	var t time.Time
	var mean, min, max, fifty, ninety int

	if mean, err = strconv.Atoi(v.Get("mean")); err != nil {
		return weft.BadRequest("invalid value for mean")
	}

	if v.Get("min") != "" {
		if min, err = strconv.Atoi(v.Get("min")); err != nil {
			return weft.BadRequest("invalid value for min")
		}
	}

	if v.Get("max") != "" {
		if max, err = strconv.Atoi(v.Get("max")); err != nil {
			return weft.BadRequest("invalid value for max")
		}
	}

	if v.Get("fifty") != "" {
		if fifty, err = strconv.Atoi(v.Get("fifty")); err != nil {
			return weft.BadRequest("invalid value for fifty")
		}
	}

	if v.Get("ninety") != "" {
		if ninety, err = strconv.Atoi(v.Get("ninety")); err != nil {
			return weft.BadRequest("invalid value for ninety")
		}
	}

	if t, err = time.Parse(time.RFC3339, v.Get("time")); err != nil {
		return weft.BadRequest("invalid time")
	}

	siteID := v.Get("siteID")
	typeID := v.Get("typeID")

	var result sql.Result

	if result, err = db.Exec(`INSERT INTO data.latency(sitePK, typePK, rate_limit, time, mean, min, max, fifty, ninety)
				SELECT sitePK, typePK, $3, $4, $5, $6, $7, $8, $9
				FROM data.site, data.type
				WHERE siteID = $1
				AND typeID = $2`,
		siteID, typeID, t.Truncate(time.Minute).Unix(),
		t, int32(mean), int32(min), int32(max), int32(fifty), int32(ninety)); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
			return &statusTooManyRequests
		} else {
			return weft.InternalServerError(err)
		}
	}

	var i int64
	if i, err = result.RowsAffected(); err != nil {
		return weft.InternalServerError(err)
	}
	if i != 1 {
		return weft.BadRequest("Didn't create row, check your query parameters exist")
	}

	// Update the summary values if the incoming is newer.
	if result, err = db.Exec(`UPDATE data.latency_summary SET
				time = $3, mean = $4, min = $5, max = $6, fifty = $7, ninety = $8
				WHERE time < $3
				AND sitePK = (SELECT sitePK from data.site WHERE siteID = $1)
				AND typePK = (SELECT typePK from data.type WHERE typeID = $2)`,
		siteID, typeID, t, int32(mean), int32(min), int32(max), int32(fifty), int32(ninety)); err != nil {
		return weft.InternalServerError(err)
	}

	// If no rows change either the values are old or it's the first time we've seen this metric.
	if i, err = result.RowsAffected(); err != nil {
		return weft.InternalServerError(err)
	}
	if i != 1 {
		if _, err = db.Exec(`INSERT INTO data.latency_summary(sitePK, typePK, time, mean, min, max, fifty, ninety)
				SELECT sitePK, typePK, $3, $4, $5, $6, $7, $8
				FROM data.site, data.type
				WHERE siteID = $1
				AND typeID = $2`,
			siteID, typeID, t, int32(mean), int32(min), int32(max), int32(fifty), int32(ninety)); err != nil {
			if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
				// incoming value was old
			} else {
				return weft.InternalServerError(err)
			}
		}
	}

	return &weft.StatusOK
}

func (a dataLatency) delete(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	siteID := v.Get("siteID")
	typeID := v.Get("typeID")

	var txn *sql.Tx
	var err error

	if txn, err = db.Begin(); err != nil {
		return weft.InternalServerError(err)
	}

	for _, table := range []string{"data.latency", "data.latency_summary", "data.latency_threshold", "data.latency_tag"} {
		if _, err = txn.Exec(`DELETE FROM `+table+` WHERE
				sitePK = (SELECT sitePK FROM data.site WHERE siteID = $1)
				AND typePK = (SELECT typePK FROM data.type WHERE typeID = $2)`,
			siteID, typeID); err != nil {
			txn.Rollback()
			return weft.InternalServerError(err)
		}
	}

	if err = txn.Commit(); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (a dataLatency) svg(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{"plot", "resolution", "yrange"}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	switch r.URL.Query().Get("plot") {
	case "", "line":
		resolution := v.Get("resolution")
		if resolution == "" {
			resolution = "minute"
		}
		if res := a.plot(v.Get("siteID"), v.Get("typeID"), resolution, ts.Line, b); !res.Ok {
			return res
		}
	case "scatter":
		resolution := v.Get("resolution")
		if resolution == "" {
			resolution = "minute"
		}
		if res := a.plot(v.Get("siteID"), v.Get("typeID"), resolution, ts.Scatter, b); !res.Ok {
			return res
		}
	default:
		if res := a.spark(v.Get("siteID"), v.Get("typeID"), b); !res.Ok {
			return res
		}
	}

	h.Set("Content-Type", "image/svg+xml")

	return &weft.StatusOK
}

func (a dataLatency) csv(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{}); !res.Ok {
		return res
	}

	v := r.URL.Query()

	// read directly from the DB and write out a CSV formatted output (time, val1, val2, etc.)
	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT format('%s,%s,%s,%s', to_char(time, 'YYYY/MM/DD HH24:MI:SS'), mean, fifty, ninety)
		AS csv FROM data.latency
		WHERE sitePK = (SELECT sitePK FROM data.site WHERE siteID = $1)
		AND typePK = (SELECT typePK FROM data.type WHERE typeID = $2)
		AND time > now() - interval '40 days'
		ORDER BY time ASC`,
		v.Get("siteID"), v.Get("typeID")); err != nil {
		return weft.InternalServerError(err)
	}
	defer rows.Close()

	// CSV headers
	b.WriteString("time,mean,fifty,ninety\n")

	// CSV data
	var l string
	for rows.Next() {
		err := rows.Scan(&l)
		if err != nil {
			return weft.InternalServerError(err)
		}
		b.WriteString(l + "\n")
	}
	rows.Close()

	h.Set("Content-Type", "text/csv")
	// allow cross domain requests, lets us use AJAX from mtr-ui.  TODO: use proxy instead
	//h.Set("Access-Control-Allow-Origin", "*")

	return &weft.StatusOK
}

// proto's query is the same as svg. The difference between them is only output mimetype.
func (a dataLatency) proto(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{"resolution"}); !res.Ok {
		return res
	}

	v := r.URL.Query()
	resolution := v.Get("resolution")
	if resolution == "" {
		resolution = "minute"
	}

	siteID := v.Get("siteID")
	typeID := v.Get("typeID")
	var err error

	var sitePK int
	if err = dbR.QueryRow(`SELECT sitePK FROM data.site WHERE siteID = $1`,
		siteID).Scan(&sitePK); err != nil {
		if err == sql.ErrNoRows {
			return &weft.NotFound
		}
		return weft.InternalServerError(err)
	}

	var typePK int

	if err = dbR.QueryRow(`SELECT typePK FROM data.type WHERE typeID = $1`,
		typeID).Scan(&typePK); err != nil {
		if err == sql.ErrNoRows {
			return &weft.NotFound
		}
		return weft.InternalServerError(err)
	}

	var dlr mtrpb.DataLatencyResult
	dlr.SiteID = siteID
	dlr.TypeID = typeID

	if err := dbR.QueryRow(`SELECT lower,upper FROM data.latency_threshold
		WHERE sitePK = $1 AND typePK = $2`,
		sitePK, typePK).Scan(&dlr.Lower, &dlr.Upper); err != nil && err != sql.ErrNoRows {
		return weft.InternalServerError(err)
	}

	rows, err := queryLatencyRows(sitePK, typePK, resolution)
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()
	for rows.Next() {
		var dl mtrpb.DataLatency
		var t time.Time
		if err = rows.Scan(&t, &dl.Mean, &dl.Fifty, &dl.Ninety); err != nil {
			return weft.InternalServerError(err)
		}

		dl.Seconds = t.Unix()
		dlr.Result = append(dlr.Result, &dl)
	}

	var by []byte

	if by, err = proto.Marshal(&dlr); err != nil {
		return weft.InternalServerError(err)
	}

	b.Write(by)

	h.Set("Content-Type", "application/x-protobuf")

	return &weft.StatusOK
}

func (a dataLatency) plot(siteID, typeID, resolution string, plotter ts.SVGPlot, b *bytes.Buffer) *weft.Result {
	var err error
	// we need the sitePK often so read it once.
	var sitePK int
	if err = dbR.QueryRow(`SELECT sitePK FROM data.site WHERE siteID = $1`,
		siteID).Scan(&sitePK); err != nil {
		if err == sql.ErrNoRows {
			return &weft.NotFound
		}
		return weft.InternalServerError(err)
	}

	var typePK int
	var scale float64
	var display string

	if err = dbR.QueryRow(`SELECT typePK, scale, display FROM data.type WHERE typeID = $1`,
		typeID).Scan(&typePK, &scale, &display); err != nil {
		if err == sql.ErrNoRows {
			return &weft.NotFound
		}
		return weft.InternalServerError(err)
	}

	var p ts.Plot

	p.SetUnit(display)

	var lower, upper int

	if err := dbR.QueryRow(`SELECT lower,upper FROM data.latency_threshold
		WHERE sitePK = $1 AND typePK = $2`,
		sitePK, typePK).Scan(&lower, &upper); err != nil && err != sql.ErrNoRows {
		return weft.InternalServerError(err)
	}

	if !(lower == 0 && upper == 0) {
		p.SetThreshold(float64(lower)*scale, float64(upper)*scale)
	}

	var tags []string
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT tag FROM data.latency_tag JOIN mtr.tag USING (tagpk) WHERE
		sitePK = $1 AND typePK = $2
		ORDER BY tag asc`,
		sitePK, typePK); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return weft.InternalServerError(err)
		}
		tags = append(tags, s)
	}
	rows.Close()

	p.SetSubTitle("Tags: " + strings.Join(tags, ","))
	p.SetTitle(fmt.Sprintf("Site: %s - %s", siteID, strings.Title(typeID)))

	// TODO - loading avg(mean) at each resolution.  Need to add max(fifty) and max(ninety) when there are some values.
	switch resolution {
	case "minute":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-12), time.Now().UTC())
		p.SetXLabel("12 hours")
	case "five_minutes":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*2), time.Now().UTC())
		p.SetXLabel("48 hours")
	case "hour":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*28), time.Now().UTC())
		p.SetXLabel("4 weeks")
	default:
		return weft.BadRequest("invalid resolution")
	}

	if err != nil {
		return weft.InternalServerError(err)
	}

	rows, err = queryLatencyRows(sitePK, typePK, resolution)
	defer rows.Close()

	pts := make(map[internal.ID]([]ts.Point))

	var mean float64
	var fifty int
	var ninety int
	var pt ts.Point

	for rows.Next() {
		if err = rows.Scan(&pt.DateTime, &mean, &fifty, &ninety); err != nil {
			return weft.InternalServerError(err)
		}
		pt.Value = mean * scale
		pts[internal.Mean] = append(pts[internal.Mean], pt)

		pt.Value = float64(fifty) * scale
		pts[internal.Fifty] = append(pts[internal.Fifty], pt)

		pt.Value = float64(ninety) * scale
		pts[internal.Ninety] = append(pts[internal.Ninety], pt)

	}
	rows.Close()

	// Add the latest value to the plot - this may be different to the average at minute or hour resolution.
	if err = dbR.QueryRow(`SELECT time, mean, fifty, ninety FROM data.latency WHERE
			sitePK = $1 AND typePK = $2
			ORDER BY time DESC
			LIMIT 1`,
		sitePK, typePK).Scan(&pt.DateTime, &mean, &fifty, &ninety); err != nil {
		return weft.InternalServerError(err)
	}

	pt.Value = mean * scale
	pts[internal.Mean] = append(pts[internal.Mean], pt)
	p.SetLatest(pt, internal.Colour(int(internal.Mean)))

	// No latest label for fifty and ninety
	pt.Value = float64(fifty) * scale
	pts[internal.Fifty] = append(pts[internal.Fifty], pt)

	pt.Value = float64(ninety) * scale
	pts[internal.Ninety] = append(pts[internal.Ninety], pt)

	for k, v := range pts {
		i := int(k)
		p.AddSeries(ts.Series{Colour: internal.Colour(i), Points: v})
	}

	// We need the labels in the order of "Mean, Fifty, Ninety" so put labels in the "range pts" won't work
	var labels ts.Labels
	labels = append(labels, ts.Label{Label: internal.Label(int(internal.Mean)), Colour: internal.Colour(int(internal.Mean))})
	labels = append(labels, ts.Label{Label: internal.Label(int(internal.Fifty)), Colour: internal.Colour(int(internal.Fifty))})
	labels = append(labels, ts.Label{Label: internal.Label(int(internal.Ninety)), Colour: internal.Colour(int(internal.Ninety))})
	p.SetLabels(labels)

	if err = plotter.Draw(p, b); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

/*
spark draws an svg spark line to b.  Assumes f.loadPK has been called first.
*/
func (a dataLatency) spark(siteID, typeID string, b *bytes.Buffer) *weft.Result {
	var p ts.Plot

	p.SetXAxis(time.Now().UTC().Add(time.Hour*-12), time.Now().UTC())

	var err error
	var rows *sql.Rows

	if rows, err = dbR.Query(`SELECT date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		 avg(mean) FROM data.latency
		 WHERE sitePK = (SELECT sitePK FROM data.site WHERE siteID = $1)
		 AND typePK = (SELECT typePK FROM data.type WHERE typeID = $2)
		 AND time > now() - interval '12 hours'
		 GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min'
		 ORDER BY t ASC`,
		siteID, typeID); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var pts []ts.Point

	for rows.Next() {
		var pt ts.Point
		if err = rows.Scan(&pt.DateTime, &pt.Value); err != nil {
			return weft.InternalServerError(err)
		}
		// No need to scale spark data for display.
		pts = append(pts, pt)
	}
	rows.Close()

	p.AddSeries(ts.Series{Colour: internal.Colour(int(internal.Mean)), Points: pts})

	if err = ts.SparkLine.Draw(p, b); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func queryLatencyRows(sitePK, typePK int, resolution string) (*sql.Rows, error) {
	var err error
	var rows *sql.Rows
	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT date_trunc('`+resolution+`',time) as t, avg(mean), max(fifty), max(ninety) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time)
		ORDER BY t ASC`,
			sitePK, typePK)
	case "five_minutes":
		rows, err = dbR.Query(`SELECT date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		 avg(mean), max(fifty), max(ninety) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min'
		ORDER BY t ASC`,
			sitePK, typePK)
	case "hour":
		rows, err = dbR.Query(`SELECT date_trunc('`+resolution+`',time) as t, avg(mean), max(fifty), max(ninety) FROM data.latency WHERE
		sitePK = $1 AND typePK = $2
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time)
		ORDER BY t ASC`,
			sitePK, typePK)
	default:
		return nil, fmt.Errorf("invalid resolution")
	}
	if err != nil {
		return nil, err
	}

	return rows, nil
}
