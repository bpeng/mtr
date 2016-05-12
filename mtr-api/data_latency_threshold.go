package main

import (
	"github.com/GeoNet/weft"
	"github.com/lib/pq"
	"net/http"
	"strconv"
)

type dataLatencyThreshold struct {
	lower, upper int
}

func (f *dataLatencyThreshold) save(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID", "lower", "upper"}, []string{}); !res.Ok {
		return res
	}

	var err error

	if f.lower, err = strconv.Atoi(r.URL.Query().Get("lower")); err != nil {
		return weft.BadRequest("invalid lower")
	}

	if f.upper, err = strconv.Atoi(r.URL.Query().Get("upper")); err != nil {
		return weft.BadRequest("invalid upper")
	}

	var dm dataLatency

	if res := dm.loadPK(r); !res.Ok {
		return res
	}

	// Ignore errors then update anyway.  TODO Change to upsert 9.5
	if _, err = db.Exec(`INSERT INTO data.latency_threshold(sitePK, typePK, lower, upper)
		VALUES ($1,$2,$3,$4)`,
		dm.sitePK, dm.dataType.typePK, f.lower, f.upper); err != nil {
		if err, ok := err.(*pq.Error); ok && err.Code == errorUniqueViolation {
			// ignore unique constraint errors
		} else {
			return weft.InternalServerError(err)
		}
	}

	if _, err = db.Exec(`UPDATE data.latency_threshold SET lower=$3, upper=$4
		WHERE
		sitePK = $1
		AND
		typePK = $2`,
		dm.sitePK, dm.dataType.typePK, f.lower, f.upper); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

func (f *dataLatencyThreshold) delete(r *http.Request) *weft.Result {
	if res := weft.CheckQuery(r, []string{"siteID", "typeID"}, []string{}); !res.Ok {
		return res
	}

	var err error

	var dm dataLatency

	if res := dm.loadPK(r); !res.Ok {
		return res
	}

	if _, err = db.Exec(`DELETE FROM data.latency_threshold
		WHERE sitePK = $1
		AND typePK = $2 `,
		dm.sitePK, dm.dataType.typePK); err != nil {
		return weft.InternalServerError(err)
	}

	return &weft.StatusOK
}

//func (f *dataLatencyThreshold) jsonV1(r *http.Request, h http.Header, b *bytes.Buffer) *result {
//	if res := weft.CheckQuery(r, []string{}, []string{}); !res.Ok {
//		return res
//	}
//
//	var s string
//
//	if err := dbR.QueryRow(`SELECT COALESCE(array_to_json(array_agg(row_to_json(l))), '[]')
//		FROM (SELECT deviceID as "DeviceID", typeID as "TypeID",
//		lower as "Lower", upper AS "Upper"
//		FROM
//		field.threshold JOIN field.device USING (devicepk)
//		JOIN field.type USING (typepk)) l`).Scan(&s); err != nil {
//		return weft.InternalServerError(err)
//	}
//
//	b.WriteString(s)
//
//	h.Set("Content-Type", "application/json;version=1")
//
//	return &weft.StatusOK
//}
