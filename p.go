package main

/* 2016 by J Ramb */

import (
	"bufio"
	"database/sql"
	"errors"
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
var programStartTime = time.Now().Round(time.Minute)

type lineType int
type RowId int64

type myDuration time.Duration

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
		log.Fatalf("%s: %s", msg, err)
	}
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
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
	rows, err := db.Query(`select value from params where param=?`, param)
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
	_, err := db.Exec(`insert into params (param, value) values(?,?)`, param, value)
	if err != nil {
		_, err = db.Exec(`update params set value = ? where param= ?`, value, param)
		errCheck(err, `setParam`)
	}
}

func findHeader(tx *sql.Tx, header string) (hdr RowId, headerText string, err error) {
	rows, err := tx.Query(`select rowid, header from headers where lower(header) like lower('%'||?||'%')`,
		header)
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
	res, err := tx.Exec(`insert into headers (header, parent, depth, creation_date, active) values(?,?,?,?,1)`,
		header, parent, depth, programStartTime)
	errCheck(err, `inserting header`)
	rowid, err := res.LastInsertId()
	fmt.Printf("Inserted %s\n", header)
	return RowId(rowid)
}

func addTime(tx *sql.Tx, entry orgEntry, headerId RowId) {
	_, err := tx.Exec(`insert into entries (header_id, start, end) values(?,?,?)`,
		headerId, entry.start, entry.end)
	errCheck(err, "inserting entry")
	//log.Print(fmt.Sprintf("Inserted %s\n", entry))
}

func prepareDB(dbfile string) *sql.DB {
	db, err := sql.Open("sqlite3", dbfile)
	errCheck(err, "Open database")
	//NOTdefer db.Close()
	//fmt.Printf("database type: %T\n", db)
	_, err = db.Exec(`create table if not exists headers
	( header_id integer primary key autoincrement
	, header text
	, depth int
	, parent integer
	, active boolean
	, creation_date datetime
	)`)
	errCheck(err, "create header table")
	_, err = db.Exec(`create table if not exists entries
	( header_id integer
	, start datetime not null
	, end datetime)`)
	errCheck(err, "create entries table")
	_, err = db.Exec(`create table if not exists log
	( creation_date datetime, log_text text)`)
	errCheck(err, "create log table")
	_, err = db.Exec(`create table if not exists params
	(param text,value text,
	primary key (param))`)
	errCheck(err, "create params table")
	setParam(db, "version", "2")
	//log.Print(`version=` + getParam(db, `version`))
	return db
}

func closeClockEntry(e *orgEntry) {
	if e.lType != clock {
		log.Fatalf("This is not a clock entry: %v", e)
	}
	if e.end == nil {
		e.end = &programStartTime
		e.duration = e.end.Sub(*e.start)
	}
}

func closeAll(tx *sql.Tx, argv []string) {
	res, err := tx.Exec(`update entries set end=? where end is null`, programStartTime)
	errCheck(err, `closing all end times`)
	updatedCnt, err := res.RowsAffected()
	errCheck(err, `closeAll RowsAffected`)
	_ = updatedCnt
	//fmt.Printf("Closed %d entries\n", updatedCnt)
}

func logEntry(tx *sql.Tx, argv []string) {
	_, err := tx.Exec(`insert into log (creation_date, log_text) values (?,?)`,
		programStartTime, strings.Join(argv, " "))
	errCheck(err, `logging time`)
}

func checkIn(tx *sql.Tx, argv []string) {
	closeAll(tx, argv)

	if len(argv) < 1 {
		log.Fatal("Need a header (or part of it) to check in")
	}
	header := argv[0]
	//log.Println("header to check into: " + header)
	hdr, headerText, err := findHeader(tx, header)
	errCheck(err, `checkIn`)

	entry := orgEntry{
		lType: clock,
		start: &programStartTime,
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
			log.Fatalf("Could not parse %s with %s", s, orgDateTime)
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
	fmt.Println("Erasing all data")
	_, err := tx.Exec(`delete from entries`)
	errCheck(err, `delete entries`)
	_, err = tx.Exec(`delete from headers`)
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

func showHeaders(tx *sql.Tx, argv []string) {
	rows, err := tx.Query(`select rowid, header, depth from headers where active=1`)
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
	y, m, d := programStartTime.Date() // Day only
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
		from = time.Date(y, m, d-7*x-(int(programStartTime.Weekday())+6)%7, 0, 0, 0, 0, time.Local)
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

func running(tx *sql.Tx, argv []string, extra string) {
	rows, err := tx.Query(`select e.start, h.header
	from entries e
	join headers h on h.header_id = e.header_id
	where e.end is null`)
	errCheck(err, `selecting running`)
	defer rows.Close()
	for rows.Next() {
		var start time.Time
		var header string
		rows.Scan(&start, &header)
		fmt.Printf("%s: %s%s\n", header, myDuration(programStartTime.Sub(start)), extra)
	}
}

func listLogEntries(tx *sql.Tx, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := tx.Query(`
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

func showTimes(tx *sql.Tx, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := tx.Query(`
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

func showDays(tx *sql.Tx, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows, err := tx.Query(`
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

func showOrg(tx *sql.Tx, argv []string) {
	from, to := decodeTimeFrame(argv)
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	hdrs, err := tx.Query(`select header_id, header, depth
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
		entr, err := tx.Query(`select start, end, strftime('%s',end)-strftime('%s',start) duration
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
	argv := os.Args[1:]                // without prog name
	defaultArgs := []string{`default`} //[len(argv):]
	if len(argv) < len(defaultArgs) {
		defaultArgs := defaultArgs[len(argv):]
		argv = append(argv, defaultArgs...)
	}
	cmd, argv := argv[0], argv[1:]
	clockfile := os.Getenv(`CLOCKFILE`)
	clockdb := clockfile + `.db`
	//fmt.Println(clockfile)
	db := prepareDB(clockdb)
	defer db.Close()
	tx, err := db.Begin()
	errCheck(err, "begin transaction")
	switch cmd {
	case `import`:
		resetDb(tx)
		//os.Remove(clockdb)
		importOrgData(tx, argv[0])
	case `head`:
		showHeaders(tx, argv)
	case `sum`, `ls`, `show`:
		showTimes(tx, argv)
	case `day`, `days`:
		showDays(tx, argv)
	case `print`, `org`:
		showOrg(tx, argv)
	case `ru`, `running`:
		running(tx, argv, "")
	case `pro`, `prompt`:
		running(tx, argv, "\\n")
	case `out`:
		closeAll(tx, argv)
	case `log`:
		logEntry(tx, argv)
	case `ll`:
		listLogEntries(tx, argv)
	case `in`:
		checkIn(tx, argv)
	default:
		fmt.Println(`Punch 2016 by jramb
-------------------
Usage: p <command> {<opt>, ...}

commands:
  h[elp]       show this message
  import       imports an org-mode file
  head         lists all active headers
  sum/ls/show  lists and sums up headers time entries
  print        prints all time entries in org-mode format
  ru[nning]    shows the currently running entry
  pro[mpt]     shows the currently running entry with '\\n'
  in <task>    check in (start timer) for task (also stops all other timers)
  out          check out (stops ALL timers)

  log          add a log entry
  ll           show log entries

You need to set the environment variable CLOCKFILE`)
	}
	tx.Commit() // not using defer
}
