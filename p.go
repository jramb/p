package main

/* 2016 by J Ramb */

import (
	"bufio"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	// go get github.com/mattn/go-sqlite3
	_ "github.com/mattn/go-sqlite3"
)

var orgDateTime = "2006-01-02 Mon 15:04"
var simpleDateFormat = `2006-01-02`
var timeFormat = `15:04`
var effectiveTimeNow = time.Now() //.Round(time.Minute)
var force = flag.Bool("force", false, "force the action")
var modifyEffectiveTime = flag.Duration("m", time.Duration(0), "modified effective time, e.g. -m 7m subtracts 7 minutes")
var roundTime = flag.Int64("r", 1, "round multiple of minutes")
var debug = flag.Bool("d", false, "debug")
var all = flag.Bool("a", false, "show all")

type lineType int
type RowId int64

type myDuration time.Duration

func d(args ...interface{}) {
	if *debug {
		log.Println(args...)
	}
}

/*
These are only to be able to log what is being executed
*/
func txQuery(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	d(`txQuery: `, query, args)
	return tx.Query(query, args...)
}

func dbQuery(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	d(`txQuery: `, query, args)
	return db.Query(query, args...)
}

func txExec(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	d(`txExec: `, query, args)
	return tx.Exec(query, args...)
}

func dbExec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	d(`dbExec: `, query, args)
	return db.Exec(query, args...)
}

func (d myDuration) String() string {
	ds := time.Duration(d).String()
	_ = ds
	mins := (time.Duration(d) / time.Minute) % 60
	hours := (time.Duration(d) - mins*time.Minute) / time.Hour
	return fmt.Sprintf("%d:%02d", hours, mins)
	//return fmt.Sprintf("%4d:%02d %s", hours, mins, ds)
	//return strings.Replace(ds, "m0s", "m", 1)
}

const (
	header lineType = iota
	clock
	text
)

type orgEntry struct {
	lType       lineType
	deep        int
	header      string
	text        string
	start       *time.Time
	end         *time.Time
	duration    time.Duration
	depthChange int
	modified    bool
}
type orgData []orgEntry

func errCheck(err error, msg string) {
	if err != nil {
		panic(fmt.Errorf("%s: %s", msg, err))
	}
}

/*
copyFileContents copies the contents of the file named src to the file named
by dst. The file will be created if it does not already exist. If the
destination file exists, all it's contents will be replaced by the contents
of the source file.
*/
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func getParam(db *sql.DB, param string) string {
	rows, err := dbQuery(db, `select value from params where param=?`, param)
	errCheck(err, "selecting getParam")
	defer rows.Close()
	for rows.Next() {
		var val string
		rows.Scan(&val)
		return val
	}
	return ""
}

func setParam(db *sql.DB, param string, value string) {
	//log.Print(`in setParam`)
	_, err := dbExec(db, `insert into params (param, value) values(?,?)`, param, value)
	if err != nil {
		_, err = dbExec(db, `update params set value = ? where param= ?`, value, param)
		errCheck(err, `setParam`)
	}
}

func findHeader(tx *sql.Tx, header string, handle string) (hdr RowId, headerText string, err error) {
	var rows *sql.Rows
	if handle != "" {
		d(`Find using handle: `, handle)
		rows, err = txQuery(tx, `select rowid, header from headers
		where handle = ?`, handle)
	} else {
		d(`Find using part of title: `, header)
		rows, err = txQuery(tx, `select rowid, header from headers
		where lower(header) like '%'||lower(?)||'%'`, header)
	}
	defer d(`done find header`)
	errCheck(err, `findHeader`)
	defer rows.Close()
	var hdrId int
	if rows.Next() {
		rows.Scan(&hdrId, &headerText)
		hdr = RowId(hdrId)
	} else {
		err = errors.New("Header '" + header + "' not found!")
		hdr = RowId(0)
	}
	if rows.Next() {
		var anotherHeader string
		rows.Scan(&hdrId, &anotherHeader)
		err = errors.New("Too many matching headers: " + headerText + ", " + anotherHeader)
		hdr = RowId(hdrId)
	}
	return
}

func addHeader(tx *sql.Tx, header string, parent RowId, depth int) RowId {
	res, err := txExec(tx, `insert into headers (header, parent, depth, creation_date, active) values(?,?,?,?,1)`,
		header, parent, depth, effectiveTimeNow)
	errCheck(err, `inserting header`)
	rowid, err := res.LastInsertId()
	fmt.Printf("Inserted %s\n", header)
	return RowId(rowid)
}

func addTime(tx *sql.Tx, entry orgEntry, headerId RowId) {
	_, err := txExec(tx, `insert into entries (header_id, start, end) values(?,?,?)`,
		headerId, entry.start, entry.end)
	errCheck(err, "inserting entry")
	//log.Print(fmt.Sprintf("Inserted %s\n", entry))
}

func getTx(db *sql.DB) *sql.Tx {
	tx, err := db.Begin()
	errCheck(err, "begin transaction")
	return tx
}

func openDB(dbfile string) *sql.DB {
	db, err := sql.Open("sqlite3", dbfile)
	errCheck(err, "Open database")
	return db
}

func prepareDB(dbfile string) *sql.DB {
	db := openDB(dbfile)
	//NOTdefer db.Close()
	//fmt.Printf("database type: %T\n", db)
	_, err := dbExec(db, `create table if not exists headers
	( header_id integer primary key autoincrement
	, handle text
	, header text
	, depth int
	, parent integer
	, active boolean
	, creation_date datetime
	)`)
	errCheck(err, "create header table")
	_, err = dbExec(db, `create table if not exists entries
	( header_id integer
	, start datetime not null
	, end datetime)`)
	errCheck(err, "create entries table")
	_, err = dbExec(db, `create table if not exists log
	( creation_date datetime, log_text text)`)
	errCheck(err, "create log table")
	_, err = dbExec(db, `create table if not exists params
	(param text,value text,
	primary key (param))`)
	errCheck(err, "create params table")
	_, err = dbExec(db, `create table if not exists todo
	( todo_id integer primary key autoincrement
	, title text not null
	, handle text
	, creation_date datetime not null
  , done_date datetime)`)
	errCheck(err, "create todo table")
	setParam(db, "version", "4")
	//log.Print(`version=` + getParam(db, `version`))
	return db
}

/*
 *func closeClockEntry(e *orgEntry) {
 *  if e.lType != clock {
 *    log.Panicf("This is not a clock entry: %v", e)
 *  }
 *  if e.end == nil {
 *    modEnd := effectiveTimeNow
 *    e.end = &modEnd
 *    e.duration = e.end.Sub(*e.start)
 *  }
 *}
 */

func closeAll(tx *sql.Tx) {
	res, err := txExec(tx, `update entries set end=? where end is null`, effectiveTimeNow)
	errCheck(err, `closing all end times`)
	updatedCnt, err := res.RowsAffected()
	errCheck(err, `closeAll RowsAffected`)
	_ = updatedCnt
	if updatedCnt > 0 {
		d("Closed entries: ", updatedCnt)
	}
}

func modifyOpen(tx *sql.Tx, argv []string) {
	if *modifyEffectiveTime == 0 {
		fmt.Fprintln(os.Stderr, `Modify requires an -m(odified) time!`)
		return
	}
	if *modifyEffectiveTime >= 24*time.Hour || *modifyEffectiveTime <= -24*time.Hour {
		fmt.Fprintf(os.Stderr, "Extend duration %s not realistic\n", *modifyEffectiveTime)
		return
	}

	rows, err := txQuery(tx, `select start, rowid from entries where end is null`)
	errCheck(err, `getting open entries`)
	defer rows.Close()
	var cnt int = 0
	for rows.Next() {
		cnt++
		var start time.Time
		var rowid int
		rows.Scan(&start, &rowid)
		newStart := start.Add(-*modifyEffectiveTime)
		fmt.Printf("New start: %s (added %s)\n", newStart.Format(timeFormat), *modifyEffectiveTime)
		_, err := txExec(tx, `update entries set start=? where rowid = ?`, newStart, rowid)
		errCheck(err, `modifying entry`)
	}
	if cnt == 0 {
		fmt.Printf(`Nothing open, maybe modify latest entry? [TODO]`)
	}
}

func logEntry(tx *sql.Tx, argv []string) {
	logString := strings.Join(argv, " ")
	if logString != "" {
		_, err := txExec(tx, `insert into log (creation_date, log_text) values (?,?)`,
			effectiveTimeNow, strings.Join(argv, " "))
		errCheck(err, `logging time`)
	}
}

func verifyHandle(db *sql.DB, handle string, fixit bool) string {
	if handle == "" {
		if fixit {
			rows, err := dbQuery(db, `select h.handle
			from entries e
			join headers h on e.header_id = h.header_id
			where e.end is null`)
			errCheck(err, `fetch current handle`)
			defer rows.Close()
			if rows.Next() {
				var h string
				rows.Scan(&h)
				return h
			} else {
				return ""
			}
		} else {
			return ""
		}
	} else if handle == "*" {
		return ""
	}
	rows, err := dbQuery(db, `select handle from headers where handle = ?`, handle)
	errCheck(err, `checking handle`)
	defer rows.Close()
	if !rows.Next() {
		errCheck(errors.New("handle not found"), `handle check`)
	}
	return handle
}

func addTodo(tx *sql.Tx, argv []string, handle string) {
	title := strings.Join(argv, " ")
	if len(argv) == 0 {
		panic("missing parameter: {@handle} todo text")
	}
	if handle == "" {
		panic("missing handle, TODOs need a handle")
	}
	res, err := txExec(tx, `insert into todo(handle,title,creation_date) values(?,?,?)`,
		handle, title, effectiveTimeNow)
	errCheck(err, `creating todo`)
	todoId, err := res.LastInsertId()
	fmt.Printf("Added TODO: #%d %s (%s)\n", todoId, title, handle)
}

func todoDone(tx *sql.Tx, argv []string, handle string) {
	if len(argv) != 1 {
		panic("missing or wrong parameter: NN (todo number)")
	}
	todoId, err := strconv.Atoi(argv[0])
	errCheck(err, `converting number `+argv[0])
	if todoId > 0 {
		rows, err := txQuery(tx, `
		 select todo_id, handle, title, creation_date
		 from todo
		 where done_date is null
		 and todo_id = ?
		 `, todoId)
		errCheck(err, `selecting todo`)
		defer rows.Close()

		if rows.Next() {
			var todoId int
			var handle string
			var title string
			var creation_date time.Time
			rows.Scan(&todoId, &handle, &title, &creation_date)
			_, err = txExec(tx, `update todo set done_date =  ? where todo_id= ?`, effectiveTimeNow, todoId)
			errCheck(err, `mark todo as done`)
			fmt.Printf("Done TODO: #%d: %s (@%s)\n", todoId, title, handle)
		} else {
			panic(`no valid todo with this number`)
		}
	}
}

func todoUndo(tx *sql.Tx, argv []string, handle string) {
	if len(argv) != 1 {
		panic("missing or wrong parameter: NN (todo number)")
	}
	todoId, err := strconv.Atoi(argv[0])
	errCheck(err, `converting number `+argv[0])
	if todoId > 0 {
		rows, err := txQuery(tx, `
		 select todo_id, handle, title, creation_date
		 from todo
		 where done_date is not null
		 and todo_id = ?
		 `, todoId)
		errCheck(err, `selecting todo`)
		defer rows.Close()

		if rows.Next() {
			var todoId int
			var handle string
			var title string
			var creation_date time.Time
			rows.Scan(&todoId, &handle, &title, &creation_date)
			_, err = txExec(tx, `update todo set done_date =  null where todo_id= ?`, todoId)
			errCheck(err, `mark todo as undone`)
			fmt.Printf("Undone TODO: #%d: %s (@%s)\n", todoId, title, handle)
		} else {
			panic(`no valid todo with this number`)
		}
	}
}

func showTodo(db *sql.DB, argv []string, handle string, limit int) {
	// remember: sql has a problem with null date, so it is problematic with done_date
	var rows *sql.Rows
	var err error
	if handle == "" {
		rows, err = dbQuery(db, `select todo_id, handle, title, creation_date
		from todo
		where done_date is null
		order by creation_date asc
		limit ?`, limit)
	} else {
		rows, err = dbQuery(db, `select todo_id, handle, title, creation_date
		from todo
		where done_date is null
		and handle = ?
		order by creation_date asc
		limit ?`, handle, limit)
	}
	errCheck(err, `selecting todos`)
	defer rows.Close()
	//var cnt int = 0
	for rows.Next() {
		var todoId int
		var handle string
		var title string
		var creation_date time.Time
		rows.Scan(&todoId, &handle, &title, &creation_date)
		fmt.Printf("#%d %s (@%s)\n", todoId, title, handle)
	}
}

func checkIn(tx *sql.Tx, argv []string, handle string) {
	var header string

	//fmt.Println(`checkIn`, argv, handle)
	if handle == "" {
		if len(argv) < 1 {
			panic(fmt.Errorf("Need a handle (or part of header) to check in"))
		}
		header = argv[0]
	}
	//log.Println("header to check into: " + header)
	hdr, headerText, err := findHeader(tx, header, handle)
	errCheck(err, `checkIn`)

	entry := orgEntry{
		lType: clock,
		start: &effectiveTimeNow,
		//end:
		//duration    time.Duration
		//depthChange int
	}
	addTime(tx, entry, hdr)
	fmt.Printf("Checked into %s\n", headerText)
}

func parseDateTime(s string) *time.Time {
	if s != "" {
		p, err := time.ParseInLocation(orgDateTime, s, time.Local)
		if err != nil {
			panic(fmt.Errorf("Could not parse %s with %s", s, orgDateTime))
		}
		return &p
	}
	return nil
}

func clockText(t *time.Time) string {
	if t != nil {
		return t.Format(orgDateTime) //"2006-01-02 Mon 15:04"
	}
	return ""
}

func simpleDate(t time.Time) string {
	return t.Format(simpleDateFormat)
}

func durationText(d time.Duration) string {
	h := int(d.Hours())
	m := int((d % time.Hour).Minutes())
	return fmt.Sprintf("%2d:%02d", h, m)
}

/* does not work...
 *func (d time.Duration) String() string {
 *  return durationText(d)
 *}
 */

func (oe orgEntry) String() string {
	if oe.lType == clock {
		ret := fmt.Sprintf("%s CLOCK: [%s]",
			strings.Repeat(" ", oe.deep),
			clockText(oe.start))
		if oe.end != nil {
			ret = ret + fmt.Sprintf("--[%s] => %s",
				clockText(oe.end),
				durationText(oe.duration))
		}
		return ret
	} else if oe.lType == header {
		return fmt.Sprintf("%s %s", strings.Repeat("*", oe.deep), oe.header)
	} else {
		return oe.text
	}
}

func parseLine(line string, deep int) (entry orgEntry) {
	var dateMatch = `\d{4}-\d{2}-\d{2}`
	var timeMatch = `\d{2}:\d{2}`
	var durationMatch = `-?\d+:\d{2}`
	var dateTimeMatch = `(` + dateMatch + ` [[:alpha:]]{1,3} ` + timeMatch + `)`
	var clockMatch = `CLOCK: \[` + dateTimeMatch + `\](--\[` + dateTimeMatch + `\]( =>\s*(` + durationMatch + `))?)?`
	var headerMatch = `^(\*+)\s+(.*)$`
	//var dateTimeMatchDet = `(\d{4})-(\d{2})-(\d{2}) [a-z]{2,3} (\d{2}):(\d{2})`
	headerRE := regexp.MustCompile(headerMatch)
	clockRE := regexp.MustCompile(clockMatch)

	if s := headerRE.FindStringSubmatch(line); s != nil {
		entry = orgEntry{
			lType:  header,
			deep:   len(s[1]),
			header: s[2],
			text:   line,
		}
	} else if s := clockRE.FindStringSubmatch(line); s != nil {
		var dur time.Duration
		startTime := parseDateTime(s[1])
		endTime := parseDateTime(s[3])
		if endTime != nil {
			dur = endTime.Sub(*startTime)
		} else {
			dur = time.Since(*startTime)
		}
		entry = orgEntry{
			lType:    clock,
			start:    startTime,
			end:      endTime,
			duration: dur,
			text:     line,
			deep:     deep,
		}
	} else {
		entry = orgEntry{
			lType: text,
			text:  line,
			deep:  deep,
		}
	}
	return entry
}

func touchTimeData(data orgData, argv []string) orgData {
	data[0].modified = true
	return data
}

func loadOrgFile(clockfile string, c chan orgEntry) {
	//log.Print("loading " + clockfile)
	//defer func() { close(c) }()
	defer close(c) //close channel when done
	cf, err := os.Open(clockfile)
	errCheck(err, `Could not open file: `+clockfile)
	defer cf.Close()

	scanner := bufio.NewScanner(cf)
	currentDeep := 0
	for scanner.Scan() {
		line := scanner.Text()
		entry := parseLine(line, currentDeep)
		//if entry.lType == header {
		//fmt.Printf("Current depth %d, new depth %d\n", currentDeep, entry.deep)
		//}
		entry.depthChange = entry.deep - currentDeep
		currentDeep = entry.deep
		c <- entry
	}
}

func resetDb(tx *sql.Tx) {
	if !*force {
		panic("You did not use 'force', aborting")
	}
	fmt.Println("Erasing all data")
	_, err := txExec(tx, `delete from entries`)
	errCheck(err, `delete entries`)
	_, err = txExec(tx, `delete from headers`)
	errCheck(err, `delete header`)
}

func importOrgData(tx *sql.Tx, clockfile string) {
	headerStack := make([]RowId, 1, 10)
	c := make(chan orgEntry)
	go loadOrgFile(clockfile, c)
	for entry := range c {
		//fmt.Printf("len=%d, headerStack=%+v, dc=%d\n", len(headerStack), headerStack, entry.depthChange)
		switch entry.depthChange {
		case 1:
			headerStack = append(headerStack, RowId(0))
		case 0:
			_ = 0
		default:
			headerStack = headerStack[:len(headerStack)+entry.depthChange]
		}
		//fmt.Printf("len=%d, headerStack=%+v", len(headerStack), headerStack)
		switch entry.lType {
		case header:
			headerStack[len(headerStack)-1] = addHeader(tx, entry.header, headerStack[len(headerStack)-2], entry.deep)
		case clock:
			addTime(tx, entry, headerStack[len(headerStack)-1])
		}
	}
}

func loadTimeFile(clockfile string,
	doer func(data orgData, argv []string) orgData,
	argv []string) {
	data := make([]orgEntry, 0, 100)

	c := make(chan orgEntry)
	go loadOrgFile(clockfile, c)
	for entry := range c {
		data = append(data, entry)
	}
	data = doer(data, argv)
}

func showHeaders(db *sql.DB, argv []string) {
	rows, err := dbQuery(db, `select rowid, header, depth from headers where active=1`)
	errCheck(err, `Selecting headers`)
	defer rows.Close()
	for rows.Next() {
		var id int
		var head string
		var depth int
		rows.Scan(&id, &head, &depth)
		fmt.Printf("[%2d] %s %s\n", id, strings.Repeat("   ", depth-1), head)
	}
}

func decodeTimeFrame(argv []string) (from, to time.Time) {
	var str string
	if len(argv) > 0 {
		str = argv[0]
	} else {
		str = ""
	}
	parts := strings.Split(str, `-`)
	var unit string
	var x int
	y, m, d := effectiveTimeNow.Date() // Day only
	from = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	var err error
	if len(parts) > 0 {
		unit = parts[0]
	}
	if unit == "" {
		unit = "week"
	}
	if len(parts) > 1 {
		x, err = strconv.Atoi(parts[1])
		errCheck(err, `converting time frame`)
	} else {
		x = 0
	}
	if unit == "yesterday" {
		unit = "today"
		x++
	}
	switch unit {
	case "month":
		from = time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
		from = from.AddDate(0, -x, 0)
		to = from.AddDate(0, 1, 0)
	case "today", "day":
		from = time.Date(y, m, d-x, 0, 0, 0, 0, time.Local)
		to = from.AddDate(0, 0, 1)
	case "week":
		//Sunday = 0
		from = time.Date(y, m, d-7*x-(int(effectiveTimeNow.Weekday())+6)%7, 0, 0, 0, 0, time.Local)
		to = from.AddDate(0, 0, 7)
	case "year":
		from = time.Date(y, 1, 1, 0, 0, 0, 0, time.Local)
		from = from.AddDate(-x, 0, 0)
		to = from.AddDate(1, 0, 0)
	case "all":
		from = time.Date(1970, 11, 24, 0, 0, 0, 0, time.Local)
		to = time.Now()
		to = to.AddDate(0, 0, 1)
	}
	return
}

func timeFrame(from, to *time.Time) string {
	if to == nil {
		return fmt.Sprintf("%s --\n", simpleDate(*from))
	} else {
		return fmt.Sprintf("%s -- %s\n", simpleDate(*from), simpleDate(to.AddDate(0, 0, -1)))
	}
}

func running(db *sql.DB, argv []string, extra string) {
	rows, err := dbQuery(db, `select e.start, h.header, '@'||h.handle handle
	from entries e
	join headers h on h.header_id = e.header_id
	where e.end is null`)
	errCheck(err, `selecting running`)
	defer rows.Close()
	for rows.Next() {
		var start time.Time
		var header string
		var handle string
		rows.Scan(&start, &header, &handle)
		fmt.Printf("%s: %s%s\n", handle, myDuration(effectiveTimeNow.Sub(start)), extra)
	}
}

func listLogEntries(db *sql.DB, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := dbQuery(db, `
select log_text, creation_date from log
where creation_date between ? and ?
and lower(log_text) like lower('%'||?||'%')
`, from, to, filter)
	errCheck(err, `selecting log entries`)
	defer rows.Close()
	for rows.Next() {
		var txt string
		var logTime time.Time
		rows.Scan(&txt, &logTime)
		fmt.Printf("%s: %s\n", simpleDate(logTime), txt)
	}
}

func showTimes(db *sql.DB, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := dbQuery(db, `
select rowid, header, depth,
  (select sum(strftime('%s',end)-strftime('%s',start)) sum_duration
	from entries e
	where e.header_id = h.header_id
  and start between ? and ?) sum_duration
from headers h
where sum_duration is not null
and lower(header) like lower('%'||?||'%')
and h.active=1
order by sum_duration desc
`, from, to, filter)
	errCheck(err, `showing times`)
	defer rows.Close()
	total := time.Duration(0)

	fmt.Printf(timeFrame(&from, &to))
	for rows.Next() {
		var id int
		var head string
		var depth int
		var duration int64
		rows.Scan(&id, &head, &depth, &duration)
		dur := time.Duration(duration * 1000000000)
		total += dur
		fmt.Printf("%14s %s\n", myDuration(dur), head)
	}
	fmt.Printf("Total: %7s\n", myDuration(total))
}

func showDays(db *sql.DB, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := dbQuery(db, `
with b as (select h.header, h.depth, date(start) start_date, (strftime('%s',end)-strftime('%s',start)) duration
from entries e
join headers h on h.header_id = e.header_id and h.active=1
where e.end is not null
and e.start between ? and ?)
select start_date, header, depth, sum(duration)
from b
where lower(header) like lower('%'||?||'%')
group by header, depth, start_date
order by start_date asc
`, from, to, filter)
	errCheck(err, `showing date times`)
	defer rows.Close()
	total := time.Duration(0)

	fmt.Printf(timeFrame(&from, &to))
	for rows.Next() {
		var start string
		var head string
		var depth int
		var duration int64
		// FIXME
		rows.Scan(&start, &head, &depth, &duration)
		dur := time.Duration(duration * 1000000000)
		total += dur
		fmt.Printf("%s: %6s %s\n", start, myDuration(dur), head)
	}
	fmt.Printf("     Total: %6s\n", myDuration(total))
}

func showOrg(db *sql.DB, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	hdrs, err := dbQuery(db, `select header_id, header, depth
	from headers
	where active=1
	and lower(header) like lower('%'||?||'%')`, filter)
	errCheck(err, `fetching headers`)
	defer hdrs.Close()
	for hdrs.Next() {
		var hid int
		var headerTxt string
		var depth int
		hdrs.Scan(&hid, &headerTxt, &depth)
		headEntry := orgEntry{
			lType:  header,
			deep:   depth,
			header: headerTxt,
		}
		entr, err := dbQuery(db, `select start, end, strftime('%s',end)-strftime('%s',start) duration
		from entries
		where header_id = ?
		and start between ? and ?
		order by start desc`, hid, from, to)
		errCheck(err, `Fetching entries for `+string(hid)+` = `+headerTxt)
		first := true
		fmt.Printf("%s\n", headEntry)
		for entr.Next() {
			if first {
				//fmt.Printf("%s\n", headEntry)
				first = false
			}
			var start time.Time
			var end time.Time
			var dur int64
			entr.Scan(&start, &end, &dur)
			clockEntry := orgEntry{
				lType:    clock,
				start:    &start,
				end:      &end,
				duration: time.Duration(dur * 1000000000),
				deep:     depth,
			}
			fmt.Printf("%s\n", clockEntry)
		}
		entr.Close()
	}
}
func listClock(data orgData, argv []string) orgData {
	for _, v := range data {
		//sv := fmt.Sprintf("%s", v)

		//if c := strings.Compare(sv, v.text); c != 0 {
		if v.String() != v.text {
			fmt.Println(">", v)
			fmt.Println("<", v.text) //"%#v\n", v)
		}
	}
	return data
}

func main() {
	var tx *sql.Tx
	var db *sql.DB
	defer func() {
		if r := recover(); r != nil {
			if tx != nil {
				tx.Rollback()
			}
			fmt.Fprintln(os.Stderr, "Aborting: ", r)
		}
	}()

	flag.Parse()
	argv := flag.Args()

	if modifyEffectiveTime != nil {
		effectiveTimeNow = effectiveTimeNow.Add(-*modifyEffectiveTime)
	}
	if roundTime != nil {
		effectiveTimeNow = effectiveTimeNow.Round(time.Minute * time.Duration(*roundTime))
	}
	//fmt.Printf("argv=%v, flag=%v, force=%v\n", argv, flag.Args(), *force)
	defaultArgs := []string{`help`} //[len(argv):]
	if len(argv) < len(defaultArgs) {
		defaultArgs := defaultArgs[len(argv):]
		argv = append(argv, defaultArgs...)
	}
	cmd, argv := argv[0], argv[1:]
	var handle string
	if len(argv) >= 1 && strings.HasPrefix(argv[0], `@`) {
		handle = strings.ToLower(argv[0][1:])
		argv = argv[1:]
	}
	clockfile := os.Getenv(`CLOCKFILE`)
	clockdb := clockfile + `.db`
	d(`Clockfile:`, clockdb)
	//fmt.Println(clockfile)
	if cmd == `init` {
		db = prepareDB(clockdb)
	} else {
		db = openDB(clockdb)
	}
	defer db.Close()
	switch cmd {
	case `init`:
		fmt.Printf("Initialized: %s\n", clockdb)
	case `head`:
		showHeaders(db, argv)
	case `sum`, `show`:
		showTimes(db, argv)
	case `day`, `days`:
		showDays(db, argv)
	case `print`, `org`:
		showOrg(db, argv)
	case `ru`, `running`:
		running(db, argv, "")
	case `pro`, `prompt`:
		handle = verifyHandle(db, handle, true)
		if handle != "" {
			showTodo(db, argv, handle, 1)
		}
		running(db, argv, "\\n")
	case `ll`:
		listLogEntries(db, argv)
	case `out`:
		tx = getTx(db)
		closeAll(tx)
	case `mod`:
		tx = getTx(db)
		modifyOpen(tx, argv)
	case `log`:
		tx = getTx(db)
		logEntry(tx, argv)
	case `in`:
		tx = getTx(db)
		handle = verifyHandle(db, handle, false)
		closeAll(tx)
		checkIn(tx, argv, handle)
	case `do`:
		tx = getTx(db)
		handle = verifyHandle(db, handle, true)
		addTodo(tx, argv, handle)
	case `todo`, `ls`:
		handle = verifyHandle(db, handle, true)
		showTodo(db, argv, handle, 9999)
	case `done`:
		handle = verifyHandle(db, handle, true)
		tx = getTx(db)
		todoDone(tx, argv, handle)
	case `undo`:
		handle = verifyHandle(db, handle, true)
		tx = getTx(db)
		todoUndo(tx, argv, handle)
	case `import`:
		tx = getTx(db)
		resetDb(tx)
		//os.Remove(clockdb)
		importOrgData(tx, argv[0])
	default:
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(os.Stderr, `
parameters: {<flags>} <command> {<time range> {, {filter> ...}}

commands:
  h[elp]       show this message
  init         initialize $CLOCKFILE.db
  import       imports an org-mode file (requires force)
  head         lists all active headers
  sum/show     lists and sums up headers time entries
  print        prints all time entries in org-mode format
  ru[nning]    shows the currently running entry
  pro[mpt]     shows the currently running entry for bash PROMPT_COMMAND
  in <task>    check in (start timer) for task (also stops all other timers)
  out          check out (stops ALL timers)

	## LOG handling
  log          add a log entry
  ll           show log entries

	## TODO handling
  do {@h} xxx  adds a TODO
  todo {@h}    shows all TODOs for the current or specified handle
  done <nn>    marks a TODO as done
  undo <nn>    marks a TODO as undone again

You need to set the environment variable CLOCKFILE
Optional parameters:`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "Force (-f): %v, effective time (-m): %v\n", *force, effectiveTimeNow)
		fmt.Fprintf(os.Stderr, "Handle (header shortcut): %s\n", handle)
		fmt.Fprintln(os.Stderr, `
-- Punch 2016 by jramb --`)
	}

	if tx != nil {
		tx.Commit() // not using defer
	}
}
