package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/GeoNet/mtr/internal"
	"github.com/GeoNet/mtr/ts"
	"github.com/GeoNet/weft"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//appMetric for get requests.
type appMetric struct{}

type times []time.Time

// funcs needed to sort a slice of time.Time
func (t times) Len() int           { return len(t) }
func (t times) Less(i, j int) bool { return t[i].Before(t[j]) }
func (t times) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

// InstanceMetric for sorting instances for SVG plots.
// public for use with sort.
type InstanceMetric struct {
	instancePK, typePK int
}

type InstanceMetrics []InstanceMetric

func (l InstanceMetrics) Len() int {
	return len(l)
}
func (l InstanceMetrics) Less(i, j int) bool {
	return l[i].instancePK < l[j].instancePK
}
func (l InstanceMetrics) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func appMetricCsv(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	a := appMetric{}

	v := r.URL.Query()
	applicationID := v.Get("applicationID")

	// the Plot type holds all the data used to plot svgs, we'll create a CSV from the labels and point values
	var p ts.Plot

	switch v.Get("group") {
	case "counters":
		if res := a.loadCounters(applicationID, "full", &p); !res.Ok {
			return res
		}
	case "timers":
		// "full" resolution for timers is 90th percentile max per minute over fourty days
		sourceID := v.Get("sourceID")
		if sourceID != "" {
			if res := a.loadTimersWithSourceID(applicationID, sourceID, "full", &p); !res.Ok {
				return res
			}
		} else {
			if res := a.loadTimers(applicationID, "full", &p); !res.Ok {
				return res
			}
		}
	case "memory":
		if res := a.loadMemory(applicationID, "full", &p); !res.Ok {
			return res
		}
	case "objects":
		if res := a.loadAppMetrics(applicationID, "full", internal.MemHeapObjects, &p); !res.Ok {
			return res
		}
	case "routines":
		if res := a.loadAppMetrics(applicationID, "full", internal.Routines, &p); !res.Ok {
			return res
		}
	default:
		return weft.BadRequest("invalid value for group")
	}

	// CSV headers, the first label is always time
	labels := p.GetLabels()
	var headers []string
	for _, label := range labels {
		headers = append(headers, label.Label)
	}

	// Labels can be in random order so keep a sorted list but with time always at 0
	sort.Strings(headers)
	headers = append([]string{"time"}, headers...)

	values := make(map[time.Time]map[string]float64)
	ts := times{} // maintaining an ordered and unique list of times in the map

	// add all points to a map to collect duplicate times with different column names
	allData := p.GetSeries()
	for i, d := range allData {
		points := d.Series.Points
		for _, point := range points {
			if values[point.DateTime] == nil {
				values[point.DateTime] = map[string]float64{labels[i].Label: point.Value}
				ts = append(ts, point.DateTime)
			} else {
				v := values[point.DateTime]
				v[labels[i].Label] = point.Value
			}
		}
	}

	// CSV headers
	if len(values) > 0 {
		b.WriteString(strings.Join(headers, ",") + "\n")
	}

	// CSV data
	sort.Sort(ts)
	for _, t := range ts {

		fields := []string{t.Format(DYGRAPH_TIME_FORMAT)}

		// start at index 1: because we've already written out the time
		for _, colName := range headers[1:] {

			val := values[t][colName]

			// don't plot values of 0:
			if val != 0.0 {
				fields = append(fields, fmt.Sprintf("%.2f", val))
			} else {
				// Dygraphs expected an empty CSV field for missing data.
				fields = append(fields, "")
			}
		}

		b.WriteString(strings.Join(fields, ",") + "\n")
	}

	return &weft.StatusOK
}

func appMetricSvg(r *http.Request, h http.Header, b *bytes.Buffer) *weft.Result {
	a := appMetric{}

	v := r.URL.Query()

	applicationID := v.Get("applicationID")

	var p ts.Plot

	resolution := v.Get("resolution")

	switch resolution {
	case "", "minute":
		resolution = "minute"
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-12), time.Now().UTC())
		p.SetXLabel("12 hours")
	case "five_minutes":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*3), time.Now().UTC())
		p.SetXLabel("48 hours")
	case "hour":
		p.SetXAxis(time.Now().UTC().Add(time.Hour*-24*28), time.Now().UTC())
		p.SetXLabel("4 weeks")
	default:
		return weft.BadRequest("invalid value for resolution")
	}

	var err error

	if v.Get("yrange") != "" {
		y := strings.Split(v.Get("yrange"), `,`)

		var ymin, ymax float64

		if len(y) != 2 {
			return weft.BadRequest("invalid yrange query param.")
		}
		if ymin, err = strconv.ParseFloat(y[0], 64); err != nil {
			return weft.BadRequest("invalid yrange query param.")
		}
		if ymax, err = strconv.ParseFloat(y[1], 64); err != nil {
			return weft.BadRequest("invalid yrange query param.")
		}
		p.SetYAxis(ymin, ymax)
	}

	resTitle := resolution
	resTitle = strings.Replace(resTitle, "_", " ", -1)
	resTitle = strings.Title(resTitle)

	switch v.Get("group") {
	case "counters":
		if res := a.loadCounters(applicationID, resolution, &p); !res.Ok {
			return res
		}

		p.SetTitle(fmt.Sprintf("Application: %s, Metric: Counters - Sum per %s", applicationID, resTitle))
		err = ts.MixedAppMetrics.Draw(p, b)
	case "timers":
		sourceID := v.Get("sourceID")
		if sourceID != "" {
			if res := a.loadTimersWithSourceID(applicationID, sourceID, resolution, &p); !res.Ok {
				return res
			}

			p.SetTitle(fmt.Sprintf("Application: %s, Source: %s, Metric: Timers - 90th Percentile (ms) per %s",
				applicationID, sourceID, resTitle))
		} else {
			if res := a.loadTimers(applicationID, resolution, &p); !res.Ok {
				return res
			}

			p.SetTitle(fmt.Sprintf("Application: %s, Metric: Timers - 90th Percentile (ms) - Max per %s",
				applicationID, resTitle))
		}
		err = ts.ScatterAppTimers.Draw(p, b)
	case "memory":
		if res := a.loadMemory(applicationID, resolution, &p); !res.Ok {
			return res
		}

		p.SetTitle(fmt.Sprintf("Application: %s, Metric: Memory (bytes) - Average per %s",
			applicationID, resTitle))
		err = ts.LineAppMetrics.Draw(p, b)
	case "objects":
		if res := a.loadAppMetrics(applicationID, resolution, internal.MemHeapObjects, &p); !res.Ok {
			return res
		}

		p.SetTitle(fmt.Sprintf("Application: %s, Metric: Memory Heap Objects (n) - Average per %s",
			applicationID, resTitle))
		err = ts.LineAppMetrics.Draw(p, b)
	case "routines":
		if res := a.loadAppMetrics(applicationID, resolution, internal.Routines, &p); !res.Ok {
			return res
		}
		p.SetTitle(fmt.Sprintf("Application: %s, Metric: Routines (n) - Average per %s",
			applicationID, resTitle))
		err = ts.LineAppMetrics.Draw(p, b)
	default:
		return weft.BadRequest("invalid value for type")
	}

	if err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK

}

func (a appMetric) loadCounters(applicationID, resolution string, p *ts.Plot) *weft.Result {
	var err error
	var rows *sql.Rows

	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT typePK, date_trunc('`+resolution+`',time) as t, sum(count)
		FROM app.counter
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time), typePK
		ORDER BY t ASC`, applicationID)
	case "five_minutes":
		rows, err = dbR.Query(`SELECT typePK,
		date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t, sum(count)
		FROM app.counter
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min', typePK
		ORDER BY t ASC`, applicationID)
	case "hour":
		rows, err = dbR.Query(`SELECT typePK, date_trunc('`+resolution+`',time) as t, sum(count)
		FROM app.counter
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time), typePK
		ORDER BY t ASC`, applicationID)
	case "full":
		rows, err = dbR.Query(`SELECT typePK, time, count
		FROM app.counter
		JOIN app.type USING (typepk)
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '40 days'
		ORDER BY time ASC`, applicationID)
	default:
		return weft.InternalServerError(fmt.Errorf("invalid resolution: %s", resolution))
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var typePK, count int
	pts := make(map[int][]ts.Point)
	total := make(map[int]int)

	for rows.Next() {
		if err = rows.Scan(&typePK, &t, &count); err != nil {
			return weft.InternalServerError(err)
		}
		pts[typePK] = append(pts[typePK], ts.Point{DateTime: t, Value: float64(count)})
		total[typePK] += count
	}
	rows.Close()

	var keys []int
	for k := range pts {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	var labels ts.Labels

	for _, k := range keys {
		p.AddSeries(ts.Series{Colour: internal.Colour(k), Points: pts[k]})
		labels = append(labels, ts.Label{Colour: internal.Colour(k), Label: fmt.Sprintf("%s (n=%d)", internal.Label(k), total[k])})
	}

	p.SetLabels(labels)

	return &weft.StatusOK

}

func (a appMetric) loadTimers(applicationID, resolution string, p *ts.Plot) *weft.Result {
	var err error

	var rows *sql.Rows

	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT sourcePK, date_trunc('`+resolution+`',time) as t, max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time), sourcePK
		ORDER BY t ASC`, applicationID)
	case "five_minutes":
		rows, err = dbR.Query(`SELECT sourcePK,
		date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min', sourcePK
		ORDER BY t ASC`, applicationID)
	case "hour":
		rows, err = dbR.Query(`SELECT sourcePK, date_trunc('`+resolution+`',time) as t, max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time), sourcePK
		ORDER BY t ASC`, applicationID)
	case "full":
		rows, err = dbR.Query(`SELECT sourcePK, time, ninety, count
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND time > now() - interval '40 days'
		ORDER BY time ASC`, applicationID)
	default:
		return weft.InternalServerError(fmt.Errorf("invalid resolution: %s", resolution))
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var sourcePK, max, n int
	var sourceID string
	pts := make(map[int][]ts.Point)
	total := make(map[int]int) // track the total counts (call) for each timer.

	for rows.Next() {
		if err = rows.Scan(&sourcePK, &t, &max, &n); err != nil {
			return weft.InternalServerError(err)
		}
		pts[sourcePK] = append(pts[sourcePK], ts.Point{DateTime: t, Value: float64(max)})
		total[sourcePK] += n
	}
	rows.Close()

	sourceIDs := make(map[int]string)

	if rows, err = dbR.Query(`SELECT sourcePK, sourceID FROM app.source`); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(&sourcePK, &sourceID); err != nil {
			return weft.InternalServerError(err)
		}
		sourceIDs[sourcePK] = sourceID
	}
	rows.Close()

	// sort the sourcePKs based on number of calls.
	keys := rank(total)

	var labels ts.Labels

	for _, k := range keys {
		p.AddSeries(ts.Series{Points: pts[k.Key], Colour: "#e34a33"})
		labels = append(labels, ts.Label{Label: fmt.Sprintf("%s (n=%d)", strings.TrimPrefix(sourceIDs[k.Key], `main.`), total[k.Key]), Colour: "lightgrey"})
	}

	p.SetLabels(labels)

	return &weft.StatusOK

}

func (a appMetric) loadTimersWithSourceID(applicationID, sourceID, resolution string, p *ts.Plot) *weft.Result {
	var err error

	var rows *sql.Rows

	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT date_trunc('minute',time) as t, avg(average), max(fifty), max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND sourcePK = (SELECT sourcePK from app.source WHERE sourceID = $2)
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('minute',time)
		ORDER BY t ASC`, applicationID, sourceID)
	case "five_minutes":
		rows, err = dbR.Query(`SELECT
		date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t,
		avg(average), max(fifty), max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND sourcePK = (SELECT sourcePK from app.source WHERE sourceID = $2)
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min'
		ORDER BY t ASC`, applicationID, sourceID)
	case "hour":
		rows, err = dbR.Query(`SELECT date_trunc('hour',time) as t, avg(average), max(fifty), max(ninety), sum(count)
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND sourcePK = (SELECT sourcePK from app.source WHERE sourceID = $2)
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('hour', time)
		ORDER BY t ASC`, applicationID, sourceID)
	case "full":
		rows, err = dbR.Query(`SELECT time, average, fifty, ninety, count
		FROM app.timer
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND sourcePK = (SELECT sourcePK from app.source WHERE sourceID = $2)
		AND time > now() - interval '40 days'
		ORDER BY time ASC`, applicationID, sourceID)
	default:
		return weft.InternalServerError(fmt.Errorf("invalid resolution: %s", resolution))
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var avg_mean float64
	var max_fifty, max_ninety, n int
	pts := make(map[internal.ID][]ts.Point)

	for rows.Next() {
		if err = rows.Scan(&t, &avg_mean, &max_fifty, &max_ninety, &n); err != nil {
			return weft.InternalServerError(err)
		}

		pts[internal.AvgMean] = append(pts[internal.AvgMean], ts.Point{DateTime: t, Value: avg_mean})
		pts[internal.MaxFifty] = append(pts[internal.MaxFifty], ts.Point{DateTime: t, Value: float64(max_fifty)})
		pts[internal.MaxNinety] = append(pts[internal.MaxNinety], ts.Point{DateTime: t, Value: float64(max_ninety)})
	}
	rows.Close()

	var labels ts.Labels

	for k, v := range pts {
		i := int(k)
		p.AddSeries(ts.Series{Points: v, Colour: internal.Colour(i)})
		labels = append(labels, ts.Label{Label: fmt.Sprintf("%s (n=%d)", strings.TrimPrefix(internal.Label(i), `main.`), len(v)), Colour: internal.Colour(i)})
	}

	p.SetLabels(labels)

	return &weft.StatusOK

}

func (a appMetric) loadMemory(applicationID, resolution string, p *ts.Plot) *weft.Result {
	var err error

	var rows *sql.Rows

	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT instancePK, typePK, date_trunc('`+resolution+`',time) as t, avg(value)
		FROM app.metric
		 WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK IN (1000, 1001, 1002)
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time), typePK, instancePK
		ORDER BY t ASC`, applicationID)
	case "five_minutes":
		rows, err = dbR.Query(`SELECT instancePK, typePK,
		date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t, avg(value)
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK IN (1000, 1001, 1002)
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min', typePK, instancePK
		ORDER BY t ASC`, applicationID)
	case "hour":
		rows, err = dbR.Query(`SELECT instancePK, typePK, date_trunc('`+resolution+`',time) as t, avg(value)
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK IN (1000, 1001, 1002)
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time), typePK, instancePK
		ORDER BY t ASC`, applicationID)
	case "full":
		rows, err = dbR.Query(`SELECT instancePK, typePK, time, value
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK IN (1000, 1001, 1002)
		AND time > now() - interval '40 days'
		ORDER BY time ASC`, applicationID)
	default:
		return weft.InternalServerError(fmt.Errorf("invalid resolution: %s", resolution))
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var typePK, instancePK int
	var avg float64
	var instanceID string
	pts := make(map[InstanceMetric][]ts.Point)

	for rows.Next() {
		if err = rows.Scan(&instancePK, &typePK, &t, &avg); err != nil {
			return weft.InternalServerError(err)
		}
		key := InstanceMetric{instancePK: instancePK, typePK: typePK}
		pts[key] = append(pts[key], ts.Point{DateTime: t, Value: avg})
	}
	rows.Close()

	instanceIDs := make(map[int]string)

	if rows, err = dbR.Query(`SELECT instancePK, instanceID FROM app.instance`); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(&instancePK, &instanceID); err != nil {
			return weft.InternalServerError(err)
		}
		instanceIDs[instancePK] = instanceID
	}
	rows.Close()

	var labels ts.Labels

	for k := range pts {
		p.AddSeries(ts.Series{Colour: internal.Colour(k.typePK), Points: pts[k]})
		labels = append(labels, ts.Label{Colour: internal.Colour(k.typePK), Label: fmt.Sprintf("%s.%s", instanceIDs[k.instancePK], strings.TrimPrefix(internal.Label(k.typePK), `Mem `))})
	}

	p.SetLabels(labels)

	return &weft.StatusOK

}

func (a appMetric) loadAppMetrics(applicationID, resolution string, typeID internal.ID, p *ts.Plot) *weft.Result {
	var err error

	var rows *sql.Rows

	switch resolution {
	case "minute":
		rows, err = dbR.Query(`SELECT instancePK, typePK, date_trunc('`+resolution+`',time) as t, avg(value)
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK = $2
		AND time > now() - interval '12 hours'
		GROUP BY date_trunc('`+resolution+`',time), typePK, instancePK
		ORDER BY t ASC`, applicationID, int(typeID))
	case "five_minutes":
		rows, err = dbR.Query(`SELECT instancePK, typePK,
		date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min' as t, avg(value)
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK = $2
		AND time > now() - interval '2 days'
		GROUP BY date_trunc('hour', time) + extract(minute from time)::int / 5 * interval '5 min', typePK, instancePK
		ORDER BY t ASC`, applicationID, int(typeID))
	case "hour":
		rows, err = dbR.Query(`SELECT instancePK, typePK, date_trunc('`+resolution+`',time) as t, avg(value)
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK = $2
		AND time > now() - interval '28 days'
		GROUP BY date_trunc('`+resolution+`',time), typePK, instancePK
		ORDER BY t ASC`, applicationID, int(typeID))
	case "full":
		rows, err = dbR.Query(`SELECT instancePK, typePK, time as t, value
		FROM app.metric
		WHERE applicationPK = (SELECT applicationPK from app.application WHERE applicationID = $1)
		AND typePK = $2
		AND time > now() - interval '40 days'
		ORDER BY time ASC`, applicationID, int(typeID))
	default:
		return weft.InternalServerError(fmt.Errorf("invalid resolution: %s", resolution))
	}
	if err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	var t time.Time
	var typePK, instancePK int
	var avg float64
	var instanceID string
	pts := make(map[InstanceMetric][]ts.Point)

	for rows.Next() {
		if err = rows.Scan(&instancePK, &typePK, &t, &avg); err != nil {
			return weft.InternalServerError(err)
		}
		key := InstanceMetric{instancePK: instancePK, typePK: typePK}
		pts[key] = append(pts[key], ts.Point{DateTime: t, Value: avg})
	}
	rows.Close()

	instanceIDs := make(map[int]string)

	if rows, err = dbR.Query(`SELECT instancePK, instanceID FROM app.instance`); err != nil {
		return weft.InternalServerError(err)
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(&instancePK, &instanceID); err != nil {
			return weft.InternalServerError(err)
		}
		instanceIDs[instancePK] = instanceID
	}
	rows.Close()

	var keys InstanceMetrics

	for k := range pts {
		keys = append(keys, k)
	}

	sort.Sort(keys)

	var labels ts.Labels

	for i, k := range keys {
		if i > len(colours) {
			i = 0
		}

		c := colours[i]

		p.AddSeries(ts.Series{Colour: c, Points: pts[k]})
		labels = append(labels, ts.Label{Colour: c, Label: fmt.Sprintf("%s.%s", instanceIDs[k.instancePK], internal.Label(k.typePK))})
	}

	p.SetLabels(labels)

	return &weft.StatusOK

}

/*
merge merges the output of cs into the single returned chan and waits for all
cs to return.

https://blog.golang.org/pipelines
*/
func merge(cs ...<-chan *weft.Result) <-chan *weft.Result {
	var wg sync.WaitGroup
	out := make(chan *weft.Result)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan *weft.Result) {
		for err := range c {
			out <- err
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func rank(r map[int]int) SourceList {
	pl := make(SourceList, len(r))
	i := 0
	for k, v := range r {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl
}

type Pair struct {
	Key   int
	Value int
}

type SourceList []Pair

func (p SourceList) Len() int {
	return len(p)
}
func (p SourceList) Less(i, j int) bool {
	return p[i].Value < p[j].Value
}
func (p SourceList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
