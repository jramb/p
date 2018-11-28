package tools

import (
	//"github.com/ttacon/chalk"
	"github.com/jramb/chalk"
	"time"
	// go get github.com/mattn/go-sqlite3
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
)

type JSONHeader struct {
	UUID string `json:"uuid"`
	//ID           json.ObjectId `json:"_id,omitempty"`
	//HeaderUUID string `json:"uuid"`
	//Owner        string    `json:"owner,omitempty"`
	Revision     int        `json:"revision"`
	Header       string     `json:"header"`
	Handle       string     `json:"handle"`
	Active       bool       `json:"active"`
	CreationDate *time.Time `json:"creation_date"`
	//UpdateDate   time.Time `json:"update_date"`
	Data *map[string]interface{} `json:"data,omitempty"`
}

type JSONEntry struct {
	UUID string `json:"uuid"`
	//ID         json.ObjectId `json:"_id,omitempty"`
	//EntryUUID  string `json:"uuid"`
	Revision   int                     `json:"revision"`
	HeaderUUID string                  `json:"header_uuid"`
	Start      *time.Time              `json:"start"`
	End        *time.Time              `json:"end,omitempty"`
	Data       *map[string]interface{} `json:"data,omitempty"`
	//UpdateDate time.Time  `json:"update_date"`
}

func dbDebug(action string, elapsed time.Duration, query string, res *sql.Result, args ...interface{}) {
	resStr := ""
	if res != nil {
		ra, _ := (*res).RowsAffected()
		resStr = chalk.Magenta.Color(fmt.Sprintf("\n==>%d", ra))
	}
	d(chalk.Green.Color(action)+": ["+chalk.Blue.Color(elapsed.String())+"]\n",
		chalk.Blue.Color(query), " ", chalk.Red, args, chalk.Reset, resStr)
}

func dbQ(dbF func(string, ...interface{}) (*sql.Rows, error), query string, args ...interface{}) *sql.Rows {
	start := time.Now()
	res, err := dbF(query, args...)
	errCheck(err, query)
	elapsed := time.Since(start)
	dbDebug("db", elapsed, query, nil, args)
	return res
}

func dbX(dbF func(string, ...interface{}) (sql.Result, error), query string, args ...interface{}) sql.Result {
	start := time.Now()
	res, err := dbF(query, args...)
	errCheck(err, query)
	elapsed := time.Since(start)
	dbDebug("db", elapsed, query, &res, args)
	return res
}

func checkDBErr(rows *sql.Rows) {
	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("DB Error: %s", err))
	}
}

func GetParam(tx *sql.Tx, param string, whenNew string) string {
	rows := dbQ(tx.Query, `select value from params where param=?`, param)
	defer rows.Close()
	defer checkDBErr(rows)
	for rows.Next() {
		var val string
		rows.Scan(&val)
		return val
	}
	return whenNew
}

func SetParam(tx *sql.Tx, param string, value string) {
	//log.Print(`in setParam`)
	res := dbX(tx.Exec, `update params set value = ? where param= ?`, value, param)
	updatedCnt, _ := res.RowsAffected()
	if updatedCnt == 0 {
		_ = dbX(tx.Exec, `insert into params (param, value) values(?,?)`, param, value)
	}
}

func SetParamInt(tx *sql.Tx, param string, value int) {
	SetParam(tx, param, strconv.Itoa(value))
}

func GetParamInt(tx *sql.Tx, param string, whenNew int) int {
	p := GetParam(tx, param, strconv.Itoa(whenNew))
	v, _ := strconv.Atoi(p)
	return v
}

func GetUncommitted(tx *sql.Tx) (*[]JSONHeader, *[]JSONEntry) {
	hdrs := make([]JSONHeader, 0, 5)
	entr := make([]JSONEntry, 0, 10)
	rh := dbQ(tx.Query, `select header_uuid, header, handle, active, creation_date from headers where coalesce(revision,'')=''`)
	defer rh.Close()
	defer checkDBErr(rh)
	for rh.Next() {
		h := JSONHeader{}
		//var active bool // column created as "boolean" -> this works
		rh.Scan(&h.UUID, &h.Header, &h.Handle, &h.Active, &h.CreationDate)
		//panic("exit")
		hdrs = append(hdrs, h)
	}
	re := dbQ(tx.Query, `select e.entry_uuid, h.header_uuid, e.start, e.end from entries e
	join headers h on h.header_id = e.header_id
	where coalesce(e.revision,'')=''`)
	defer re.Close()
	defer checkDBErr(re)
	for re.Next() {
		e := JSONEntry{}
		re.Scan(&e.UUID, &e.HeaderUUID, &e.Start, &e.End)
		entr = append(entr, e)
	}

	return &hdrs, &entr
}

func CommitRevision(tx *sql.Tx, revision int) error {
	_ = dbX(tx.Exec, `update headers set revision=? where revision is null`, revision)
	_ = dbX(tx.Exec, `update entries set revision=? where revision is null`, revision)
	return nil
}

func ApplyUpdates(tx *sql.Tx, hdr []JSONHeader, entr []JSONEntry, revision int) error {
	if len(hdr) == 0 && len(entr) == 0 {
		return nil
	}
	for _, h := range hdr {
		active := 1
		if h.Active {
			active = 0
		}
		_ = dbX(tx.Exec, `insert or replace into headers
					(header_uuid, header, handle, active, creation_date, revision)
					values (?, ?, ?, ?, ?, ?)`,
			h.UUID, h.Header, h.Handle, active, h.CreationDate, revision)
		//log.Println("UpH:", res)
	}
	for _, e := range entr {
		_ = dbX(tx.Exec, `insert or replace into entries
					(entry_uuid, header_id, start, end, revision)
					values (?,(select header_id from headers where header_uuid=?),?,?,?)`,
			e.UUID, e.HeaderUUID, e.Start, e.End, revision)
		//log.Println("UpE:", res)
	}
	return nil
}
