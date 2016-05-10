package main

import (
	"bytes"
	"github.com/GeoNet/mtr/mtrpb"
	"github.com/golang/protobuf/proto"
	"net/http"
	"sort"
)

type fieldPage struct {
	page
	Path         string
	Summary      map[string]int
	Metrics      []idCount
	DeviceModels []deviceModel
	Devices      []device
	ModelId      string
	DeviceId     string
	TypeId       string
	Status       string
	Resolution   string
	MtrApiUrl    string
}

type deviceModels []deviceModel

func (m deviceModels) Len() int {
	return len(m)
}

func (m deviceModels) Less(i, j int) bool {
	return m[i].ModelId < m[j].ModelId
}

func (m deviceModels) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type devices []device

func (m devices) Len() int {
	return len(m)
}

func (m devices) Less(i, j int) bool {
	return m[i].DeviceId < m[j].DeviceId
}

func (m devices) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type idCounts []idCount

func (m idCounts) Len() int {
	return len(m)
}

func (m idCounts) Less(i, j int) bool {
	return m[i].Id < m[j].Id
}

func (m idCounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type deviceModel struct {
	ModelId     string
	TypeCount   int
	DeviceCount int
	Count       map[string]int
}

type device struct {
	DeviceId string
	ModelId  string
	typeStatus
}

type typeStatus struct {
	TypeId string
	Status string
}

type idCount struct {
	Id    string
	Count map[string]int
}

func fieldPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := fieldPage{}
	p.Path = r.URL.Path
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	if p.Summary, err = getFieldSummary(); err != nil {
		return internalServerError(err)
	}

	if err = fieldTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func fieldMetricsPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := fieldPage{}
	p.Path = r.URL.Path
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	if err = p.getMetricsSummary(); err != nil {
		return internalServerError(err)
	}

	if err = fieldTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func fieldDevicesPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {

	var err error

	if res := checkQuery(r, []string{}, []string{"modelID", "typeID", "status"}); !res.ok {
		return res
	}

	// We create a page struct with variables to substitute into the loaded template
	p := fieldPage{}
	p.Path = r.URL.Path
	p.MtrApiUrl = mtrApiUrl.String()
	p.Border.Title = "GeoNet MTR"

	if err = p.populateTags(); err != nil {
		return internalServerError(err)
	}

	q := r.URL.Query()
	p.ModelId = q.Get("modelID")
	p.TypeId = q.Get("typeID")
	p.Status = q.Get("status")
	if p.ModelId != "" && p.TypeId != "" {
		if err = p.getDevicesByModelType(); err != nil {
			return internalServerError(err)
		}
	} else if p.ModelId != "" && p.Status != "" {
		if err = p.getDevicesByModelStatus(); err != nil {
			return internalServerError(err)
		}
	} else {
		if err = p.getDevicesSummary(); err != nil {
			return internalServerError(err)
		}
	}
	if err = fieldTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func fieldPlotPageHandler(r *http.Request, h http.Header, b *bytes.Buffer) *result {
	if res := checkQuery(r, []string{"deviceID", "typeID"}, []string{"resolution"}); !res.ok {
		return res
	}
	p := fieldPage{}
	p.Path = r.URL.Path
	p.MtrApiUrl = mtrApiUrl.String()
	p.Border.Title = "GeoNet MTR"
	q := r.URL.Query()
	p.DeviceId = q.Get("deviceID")
	p.TypeId = q.Get("typeID")
	p.Resolution = q.Get("resolution")
	if p.Resolution == "" {
		p.Resolution = "minute"
	}

	if err := fieldTemplate.ExecuteTemplate(b, "border", p); err != nil {
		return internalServerError(err)
	}

	return &statusOK
}

func getFieldSummary() (m map[string]int, err error) {
	u := *mtrApiUrl
	u.Path = "/field/metric/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.FieldMetricSummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	m = make(map[string]int)
	m["metrics"] = len(f.Result)
	devices := make(map[string]bool)
	for _, r := range f.Result {
		devices[r.DeviceID] = true
		incFieldCount(m, r)
	}
	m["devices"] = len(devices)
	return
}

func (p *fieldPage) getMetricsSummary() (err error) {
	u := *mtrApiUrl
	u.Path = "/field/metric/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.FieldMetricSummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.Metrics = make([]idCount, 0)
	for _, r := range f.Result {
		p.Metrics = updateFieldMetric(p.Metrics, r)
	}

	sort.Sort(idCounts(p.Metrics))
	return
}

func (p *fieldPage) getDevicesSummary() (err error) {
	u := *mtrApiUrl
	u.Path = "/field/metric/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.FieldMetricSummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.DeviceModels = make([]deviceModel, 0)
	for _, r := range f.Result {
		p.DeviceModels = updateFieldDevice(p.DeviceModels, r)
	}

	sort.Sort(deviceModels(p.DeviceModels))
	return
}

func (p *fieldPage) getDevicesByModelStatus() (err error) {
	u := *mtrApiUrl
	u.Path = "/field/metric/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.FieldMetricSummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.Devices = make([]device, 0)
	for _, r := range f.Result {
		if r.ModelID == p.ModelId && fieldStatusString(r) == p.Status {
			t := device{ModelId: p.ModelId, DeviceId: r.DeviceID}
			t.TypeId = r.TypeID
			t.Status = fieldStatusString(r)
			p.Devices = append(p.Devices, t)
		}
	}

	sort.Sort(devices(p.Devices))
	return
}

func (p *fieldPage) getDevicesByModelType() (err error) {
	u := *mtrApiUrl
	u.Path = "/field/metric/summary"

	var b []byte
	if b, err = getBytes(u.String(), "application/x-protobuf"); err != nil {
		return
	}

	var f mtrpb.FieldMetricSummaryResult

	if err = proto.Unmarshal(b, &f); err != nil {
		return
	}

	p.Devices = make([]device, 0)
	for _, r := range f.Result {
		if r.ModelID == p.ModelId && r.TypeID == p.TypeId {
			t := device{ModelId: p.ModelId, DeviceId: r.DeviceID}
			t.TypeId = r.TypeID
			t.Status = fieldStatusString(r)
			p.Devices = append(p.Devices, t)
		}
	}

	sort.Sort(devices(p.Devices))
	return
}

// Increase count if Id exists in slice, append to slice if it's a new Id
func updateFieldMetric(m []idCount, result *mtrpb.FieldMetricSummary) []idCount {
	for _, r := range m {
		if r.Id == result.TypeID {
			incFieldCount(r.Count, result)
			return m
		}
	}

	c := make(map[string]int)
	incFieldCount(c, result)
	return append(m, idCount{Id: result.TypeID, Count: c})
}

// Increase count if Id exists in slice, append to slice if it's a new Id
func updateFieldDevice(m []deviceModel, result *mtrpb.FieldMetricSummary) []deviceModel {
	for i, r := range m {
		if r.ModelId == result.ModelID {
			r.TypeCount++
			incFieldCount(r.Count, result)
			m[i] = r
			return m
		}
	}

	c := make(map[string]int)
	incFieldCount(c, result)

	return append(m, deviceModel{ModelId: result.ModelID, Count: c, TypeCount: 1})
}

func incFieldCount(m map[string]int, r *mtrpb.FieldMetricSummary) {
	s := fieldStatusString(r)
	m[s] = m[s] + 1
	m["total"] = m["total"] + 1
}

func fieldStatusString(r *mtrpb.FieldMetricSummary) string {
	switch {
	case r.Upper == 0 && r.Lower == 0:
		return "unknown"
	case r.Value >= r.Lower && r.Value <= r.Upper:
		return "good"
		// TBD: late
	}
	return "bad"
}
