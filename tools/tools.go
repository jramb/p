package tools

/**
*
* 2016 by J Ramb
*
**/

import (
	"bufio"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	//"log"
	"github.com/spf13/viper"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	// go get github.com/mattn/go-sqlite3
	_ "github.com/mattn/go-sqlite3"
	"github.com/ttacon/chalk"
)

var orgDateTime = "2006-01-02 Mon 15:04"
var isoDateTime = "2006-01-02 15:04:05"
var simpleDateFormat = `2006-01-02`
var timeFormat = `15:04`

//var effectiveTimeNow = time.Now() //.Round(time.Minute)

var force = flag.Bool("force", false, "force the action")

//var modifyEffectiveTime = flag.Duration("m", time.Duration(0), "modified effective time, e.g. -m 7m subtracts 7 minutes")
//var roundTime = flag.Int64("r", 1, "round multiple of minutes")

var debug = flag.Bool("d", false, "debug")
var all = flag.Bool("a", false, "show all")

type lineType int
type RowId int64

//type myDuration time.Duration

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

func d(args ...interface{}) {
	if viper.GetBool("debug") {
		//log.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
		fmt.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
	}
}

/*
These are only to be able to log what is being executed
*/

/*
func (d myDuration) String() string {
	if viper.GetString("durationStyle") == "time" {
		var sign string
		if d < 0 {
			sign = "-"
			d = -d
		}
		td := time.Duration(d)
		mins := (td / time.Minute) % 60
		hours := (td - mins*time.Minute) / time.Hour
		return fmt.Sprintf("%s%d:%02d", sign, hours, mins)
	} else {
		hours := time.Duration(d).Minutes() / 60.
		return fmt.Sprintf("%3.1fh", hours)
	}
	//return fmt.Sprintf("%4d:%02d %s", hours, mins, ds)
	//return strings.Replace(ds, "m0s", "m", 1)
}
*/

func dbDebug(action string, elapsed time.Duration, query string, args ...interface{}) {
	d(chalk.Green.Color(action)+": [", chalk.Blue, elapsed, chalk.Reset, "] \n", chalk.Blue.Color(query), " ", chalk.Red, args, chalk.Reset)
}

func dbQ(dbF func(string, ...interface{}) (*sql.Rows, error), query string, args ...interface{}) *sql.Rows {
	start := time.Now()
	res, err := dbF(query, args...)
	errCheck(err, query)
	elapsed := time.Since(start)
	dbDebug("db", elapsed, query, args)
	return res
}

func dbX(dbF func(string, ...interface{}) (sql.Result, error), query string, args ...interface{}) sql.Result {
	start := time.Now()
	res, err := dbF(query, args...)
	errCheck(err, query)
	elapsed := time.Since(start)
	dbDebug("db", elapsed, query, args)
	return res
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
	rows := dbQ(db.Query, `select value from params where param=?`, param)
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
	//_ := dbX(db.Exec, `insert into params (param, value) values(?,?)`, param, value)
	//if err != nil {
	_ = dbX(db.Exec, `update params set value = ? where param= ?`, value, param)
}

func findHeader(tx *sql.Tx, header string, handle string) (hdr RowId, headerText string, err error) {
	var rows *sql.Rows
	if handle != "" {
		d(`Find using handle: `, handle)
		rows = dbQ(tx.Query, `select rowid, header from headers
		where handle = ?`, handle)
	} else {
		d(`Find using part of title: `, header)
		rows = dbQ(tx.Query, `select rowid, header from headers
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

func AddHeader(tx *sql.Tx, header string, handle string, parent RowId, depth int) (RowId, error) {
	res := dbX(tx.Exec, `insert into headers (header, handle, parent, depth, creation_date, active) values(?,?,?,?,?,1)`,
		header, handle, parent, depth, time.Now())
	rowid, err := res.LastInsertId()
	if err != nil {
		return RowId(rowid), err
	}
	fmt.Printf("Inserted %s\n", header)
	return RowId(rowid), nil
}

func addTime(tx *sql.Tx, entry orgEntry, headerId RowId) {
	_ = dbX(tx.Exec, `insert into entries (header_id, start, end) values(?,?,?)`,
		headerId, entry.start, entry.end)
	//log.Print(fmt.Sprintf("Inserted %s\n", entry))
}

func GetTx(db *sql.DB) (*sql.Tx, error) {
	tx, err := db.Begin()
	return tx, err
}

func OpenDB(checkExists bool) (*sql.DB, error) {
	var dbfile string
	dbfile = viper.GetString("clockfile")
	d("clockfile=" + dbfile)
	if checkExists || dbfile == "" {
		if _, err := os.Stat(dbfile); os.IsNotExist(err) {
			return nil, fmt.Errorf("Could not find your clockfile, please verify setup in your configuration\n*** %s %s", err, dbfile)
		}
	}
	return sql.Open("sqlite3", dbfile)
}

func WithOpenDB(checkExists bool, fn func(*sql.DB) error) error {
	if db, err := OpenDB(checkExists); err == nil {
		defer db.Close()
		return fn(db)
	} else {
		return err
	}
}

func WithTransaction(fn func(*sql.DB, *sql.Tx) error) error {
	return WithOpenDB(true, func(db *sql.DB) error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer RollbackOnError(tx)
		return fn(db, tx)
	})
}

func PrepareDB() error {
	db, err := OpenDB(false)
	if err != nil {
		return err
	}
	defer db.Close()
	//fmt.Printf("database type: %T\n", db)
	_ = dbX(db.Exec, `create table if not exists headers
	( header_id integer primary key autoincrement
	, handle text
	, header text
	, depth int
	, parent integer
	, active boolean
	, creation_date datetime
	)`)
	_ = dbX(db.Exec, `create table if not exists entries
	( header_id integer
	, start datetime not null
	, end datetime)`)
	_ = dbX(db.Exec, `create table if not exists log
	( creation_date datetime, log_text text)`)
	_ = dbX(db.Exec, `create table if not exists params
	(param text,value text,
	primary key (param))`)
	_ = dbX(db.Exec, `create table if not exists todo
	( todo_id integer primary key autoincrement
	, title text not null
	, handle text
	, creation_date datetime not null
  , done_date datetime)`)
	setParam(db, "version", "4")
	//log.Print(`version=` + getParam(db, `version`))
	fmt.Println("Initialized database with version", getParam(db, `version`))
	return nil
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

func CloseAll(tx *sql.Tx, effectiveTimeNow time.Time) error {
	res := dbX(tx.Exec, `update entries set end=? where end is null`, effectiveTimeNow)
	updatedCnt, err := res.RowsAffected()
	errCheck(err, `fetching RowsAffected`)
	if updatedCnt > 0 {
		d("Closed entries: ", updatedCnt)
	}
	return nil
}

func modifyOpen(tx *sql.Tx, argv []string, modifyEffectiveTime *time.Duration) {
	if *modifyEffectiveTime == 0 {
		fmt.Fprintln(os.Stderr, `Modify requires an -m(odified) time!`)
		return
	}
	if *modifyEffectiveTime >= 24*time.Hour || *modifyEffectiveTime <= -24*time.Hour {
		fmt.Fprintf(os.Stderr, "Extend duration %s not realistic\n", *modifyEffectiveTime)
		return
	}

	rows := dbQ(tx.Query, `select start, rowid from entries where end is null`)
	defer rows.Close()
	var cnt int = 0
	for rows.Next() {
		cnt++
		var start time.Time
		var rowid int
		rows.Scan(&start, &rowid)
		newStart := start.Add(-*modifyEffectiveTime)
		fmt.Printf("New start: %s (added %s)\n", newStart.Format(timeFormat), *modifyEffectiveTime)
		_ = dbX(tx.Exec, `update entries set start=? where rowid = ?`, newStart, rowid)
	}
	if cnt == 0 {
		fmt.Printf(`Nothing open, maybe modify latest entry? [TODO]`)
	}
}

func LogEntry(tx *sql.Tx, argv []string, effectiveTimeNow time.Time) error {
	logString := strings.Join(argv, " ")
	if logString != "" {
		_ = dbX(tx.Exec, `insert into log (creation_date, log_text) values (?,?)`,
			effectiveTimeNow, strings.Join(argv, " "))
	}
	return nil
}

func VerifyHandle(db *sql.DB, handle string, fixit bool) (string, error) {
	if handle == "" {
		if fixit {
			rows := dbQ(db.Query, `select h.handle
			from entries e
			join headers h on e.header_id = h.header_id
			where e.end is null`)
			defer rows.Close()
			if rows.Next() {
				var h string
				rows.Scan(&h)
				return h, nil
			} else {
				return "", nil
			}
		} else {
			return "", nil
		}
	} else if handle == "*" {
		return "", nil
	}
	rows := dbQ(db.Query, `select handle from headers where handle = ?`, handle)
	defer rows.Close()
	if !rows.Next() {
		return "", errors.New("handle not found: " + handle)
		//errCheck(errors.New("handle not found"), `handle check`)
	}
	return handle, nil
}

func AddTodo(tx *sql.Tx, title string, handle string, effectiveTimeNow time.Time) error {
	//title := strings.Join(argv, " ")
	if len(title) == 0 {
		panic("missing parameter: todo text")
	}
	if handle == "" {
		panic("missing handle, TODOs need a handle")
	}
	res := dbX(tx.Exec, `insert into todo(handle,title,creation_date) values(?,?,?)`,
		handle, title, effectiveTimeNow)
	todoId, err := res.LastInsertId()
	if err != nil {
		return err
	}
	fmt.Printf("Added TODO: #%d %s (@%s)\n", todoId, title, handle)
	return nil
}

func TodoDone(tx *sql.Tx, argv []string, handle string, effectiveTimeNow time.Time) error {
	if len(argv) == 0 {
		panic("missing or wrong parameter: NN (todo number)")
	}
	for _, nn := range argv {
		todoId, err := strconv.Atoi(nn)
		errCheck(err, `converting number `+nn)
		if todoId > 0 {
			rows := dbQ(tx.Query, `
		 select todo_id, handle, title, creation_date
		 from todo
		 where done_date is null
		 and todo_id = ?
		 `, todoId)
			defer rows.Close()

			if rows.Next() {
				var todoId int
				var handle string
				var title string
				var creation_date time.Time
				rows.Scan(&todoId, &handle, &title, &creation_date)
				_ = dbX(tx.Exec, `update todo set done_date =  ? where todo_id= ?`, effectiveTimeNow, todoId)
				fmt.Printf("Done TODO: #%d: %s (@%s)\n", todoId, title, handle)
			} else {
				return fmt.Errorf("No valid TODO with this number %d", todoId)
			}
		}
	}
	return nil
}

func TodoUndo(tx *sql.Tx, argv []string, handle string) error {
	if len(argv) == 0 {
		panic("missing or wrong parameter: NN (todo number)")
	}
	for _, nn := range argv {
		todoId, err := strconv.Atoi(nn)
		errCheck(err, `converting number `+nn)
		if todoId > 0 {
			rows := dbQ(tx.Query, `
		 select todo_id, handle, title, creation_date
		 from todo
		 where done_date is not null
		 and todo_id = ?
		 `, todoId)
			defer rows.Close()

			if rows.Next() {
				var todoId int
				var handle string
				var title string
				var creation_date time.Time
				rows.Scan(&todoId, &handle, &title, &creation_date)
				_ = dbX(tx.Exec, `update todo set done_date =  null where todo_id= ?`, todoId)
				fmt.Printf("Undone TODO: #%d: %s (@%s)\n", todoId, title, handle)
			} else {
				return fmt.Errorf("No valid TODO with this number %d", todoId)
			}
		}
	}
	return nil
}

func ShowTodo(db *sql.DB, argv []string, handle string, limit int) error {
	// remember: sql has a problem with null date, so it is problematic with done_date
	var rows *sql.Rows
	var orderBy string
	if limit == 1 {
		orderBy = "random()"
	} else {
		orderBy = "creation_date asc"
	}
	if handle == "" {
		rows = dbQ(db.Query, fmt.Sprintf(`select todo_id, handle, title, creation_date
		from todo
		where done_date is null
		order by %s
		limit ?`, orderBy), limit)
	} else {
		rows = dbQ(db.Query, fmt.Sprintf(`select todo_id, handle, title, creation_date
		from todo
		where done_date is null
		and handle = ?
		order by %s
		limit ?`, orderBy), handle, limit)
	}
	defer rows.Close()
	//var cnt int = 0
	for rows.Next() {
		var todoId int
		var handle string
		var title string
		var creation_date time.Time
		rows.Scan(&todoId, &handle, &title, &creation_date)
		fmt.Printf(chalk.Cyan.Color("#%d %s (@%s)\n"), todoId, title, handle)
	}
	return nil
}

func CheckIn(tx *sql.Tx, argv []string, handle string, effectiveTimeNow time.Time) error {
	var header string

	if handle == "" {
		if len(argv) < 1 {
			return fmt.Errorf("Need a handle (or part of header) to check in")
		}
		header = argv[0]
	}
	//log.Println("header to check into: " + header)
	hdr, headerText, err := findHeader(tx, header, handle)
	if err != nil {
		return err
	}

	entry := orgEntry{
		lType: clock,
		start: &effectiveTimeNow,
		//end:
		//duration    time.Duration
		//depthChange int
	}
	addTime(tx, entry, hdr)
	fmt.Printf("Checked into %s\n", headerText)
	return nil
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
	_ = dbX(tx.Exec, `delete from entries`)
	_ = dbX(tx.Exec, `delete from headers`)
}

func importOrgData(tx *sql.Tx, clockfile string) {
	headerStack := make([]RowId, 1, 10)
	c := make(chan orgEntry)
	go loadOrgFile(clockfile, c)
	for entry := range c {
		var err error
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
			headerStack[len(headerStack)-1], err = AddHeader(tx, entry.header, "", headerStack[len(headerStack)-2], entry.deep)
			if err != nil {
				panic(err)
			}
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

func ShowHeaders(db *sql.DB) error {
	rows := dbQ(db.Query, `select rowid, header, handle, depth from headers where active=1`)
	defer rows.Close()
	for rows.Next() {
		var id int
		var head string
		var handle string
		var depth int
		rows.Scan(&id, &head, &handle, &depth)

		fmt.Printf("[%2d] %s %s\n", id, strings.Repeat("   ", depth), formatHeader(head, handle))
	}
	return nil
}

func decodeTimeFrame(argv []string) (from, to time.Time, err error) {
	var str string
	if len(argv) > 0 {
		str = argv[0]
	} else {
		str = ""
	}
	parts := strings.Split(str, `-`)
	var unit string
	var x int
	y, m, d := time.Now().Date() // Day only
	from = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	if len(parts) > 0 {
		unit = parts[0]
	}
	if unit == "" {
		unit = "week"
	}
	if len(parts) > 1 {
		x, err = strconv.Atoi(parts[1])
		if err != nil {
			return
		}
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
		from = time.Date(y, m, d-7*x-(int(time.Now().Weekday())+6)%7, 0, 0, 0, 0, time.Local)
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
		return fmt.Sprintf("%s --", simpleDate(*from))
	} else {
		return fmt.Sprintf("%s -- %s", simpleDate(*from), simpleDate(to.AddDate(0, 0, -1)))
	}
}

func Running(db *sql.DB, argv []string, extra string, effectiveTimeNow time.Time) {
	rows := dbQ(db.Query, `select e.start, h.header, h.handle
	from entries e
	join headers h on h.header_id = e.header_id
	where e.end is null`)
	defer rows.Close()
	for rows.Next() {
		var start time.Time
		var header string
		var handle string
		rows.Scan(&start, &header, &handle)
		if handle != "" {
			fmt.Printf(chalk.Green.Color("@%s: ")+chalk.Magenta.Color("%s%s")+"\n", handle, formatDuration(effectiveTimeNow.Sub(start)), extra)
		} else {
			fmt.Printf(chalk.Green.Color("%s: ")+chalk.Magenta.Color("%s%s")+"\n", header, formatDuration(effectiveTimeNow.Sub(start)), extra)
		}
	}
}

func ListLogEntries(db *sql.DB, argv []string) error {
	from, to, err := decodeTimeFrame(argv)
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows := dbQ(db.Query, `select log_text, creation_date from log
where creation_date between ? and ?
and lower(log_text) like lower('%'||?||'%')
`, from, to, filter)
	defer rows.Close()
	for rows.Next() {
		var txt string
		var logTime time.Time
		rows.Scan(&txt, &logTime)
		fmt.Printf("%s: %s\n", logTime.Format(isoDateTime), txt)
	}
	return nil
}

func DurationRound(unrounded time.Duration, rnd time.Duration, bias time.Duration) time.Duration {
	var zero time.Time // zero.IsZero!
	if bias > rnd/2 {
		bias = rnd / 2
	}
	return zero.Add(unrounded).Add(bias).Round(rnd).Sub(zero)
}

func formatDuration(d time.Duration) string {
	if viper.GetString("show.style") == "time" {
		var sign string
		if d < 0 {
			sign = "-"
			d = -d
		}
		mins := (d / time.Minute) % 60
		hours := (d - mins*time.Minute) / time.Hour
		return fmt.Sprintf("%s%d:%02d", sign, hours, mins)
	} else {
		hours := time.Duration(d).Minutes() / 60.
		return fmt.Sprintf("%3.1f h", hours)
	}
}

func formatRoundErr(rounderr time.Duration) string {
	if viper.GetBool("show.display-rounding") {
		if viper.GetString("show.style") == "time" {
			//rounderr = -rounderr
			if rounderr >= 0 {
				return fmt.Sprintf("  +%s", formatDuration(rounderr))
			} else {
				return fmt.Sprintf("  %s", formatDuration(rounderr))
			}
		} else {
			minutes := DurationRound(rounderr, time.Minute, time.Duration(0)) / time.Minute
			return fmt.Sprintf("%+5dm", minutes)
		}
	} else {
		return ""
	}
}

func formatHeader(head, handle string) string {
	if handle != "" {
		return head + " @" + handle
	} else {
		return head
	}
}

func ShowTimes(db *sql.DB, argv []string, rounding time.Duration, bias time.Duration) (err error) {
	from, to, err := decodeTimeFrame(argv)
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows := dbQ(db.Query, `
select rowid, header, handle, depth,
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
	defer rows.Close()
	total := time.Duration(0)
	rounderr := time.Duration(0)

	fmt.Println("Headers:", timeFrame(&from, &to))
	for rows.Next() {
		var id int
		var head string
		var handle string
		var depth int
		var duration int64
		rows.Scan(&id, &head, &handle, &depth, &duration)
		dur := time.Duration(duration * 1000000000)
		rounded := DurationRound(dur, rounding, bias)
		diff := dur - rounded
		dur = rounded
		rounderr += diff
		fmt.Printf("%21s%s  %s\n", formatDuration(dur), formatRoundErr(diff), formatHeader(head, handle))
		total += dur
	}
	fmt.Printf("     Total: %9s%s\n", formatDuration(total), formatRoundErr(rounderr))
	return nil
}

func ShowDays(db *sql.DB, argv []string, rounding time.Duration, bias time.Duration) error {
	from, to, err := decodeTimeFrame(argv)
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows := dbQ(db.Query, `
with b as (select h.header, h.handle, h.depth, date(start) start_date, (strftime('%s',end)-strftime('%s',start)) duration
from entries e
join headers h on h.header_id = e.header_id and h.active=1
where e.end is not null
and e.start between ? and ?)
select start_date, header, handle, depth, sum(duration)
from b
where lower(header) like lower('%'||?||'%')
group by header, handle, depth, start_date
order by start_date asc
`, from, to, filter)
	defer rows.Close()
	total := time.Duration(0)
	rounderr := time.Duration(0)

	fmt.Println("Daily:", timeFrame(&from, &to))
	for rows.Next() {
		var start string
		var head string
		var handle string
		var depth int
		var duration int64
		rows.Scan(&start, &head, &handle, &depth, &duration)
		dur := time.Duration(duration * 1000000000)
		rounded := DurationRound(dur, rounding, bias)
		diff := dur - rounded
		rounderr += diff
		dur = rounded
		fmt.Printf("%s: %9s%s  %s\n", start, formatDuration(dur), formatRoundErr(diff), formatHeader(head, handle))
		total += dur
	}
	fmt.Printf("     Total: %9s%s\n", formatDuration(total), formatRoundErr(rounderr))
	return nil
}

func ShowOrg(db *sql.DB, argv []string) error {
	from, to, err := decodeTimeFrame(argv)
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	hdrs := dbQ(db.Query, `select header_id, header, depth
	from headers
	where active=1
	and lower(header) like lower('%'||?||'%')`, filter)
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
		entr := dbQ(db.Query, `select start, end, strftime('%s',end)-strftime('%s',start) duration
		from entries
		where header_id = ?
		and start between ? and ?
		order by start desc`, hid, from, to)
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
	return nil
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

// defer RollbackOnError(tx)
func RollbackOnError(tx *sql.Tx) {
	if r := recover(); r != nil {
		if tx != nil {
			tx.Rollback()
		}
		fmt.Fprintln(os.Stderr, "Aborting: ", r)
	} else {
		if tx != nil {
			tx.Commit()
		}
	}
}

func ParseHandle(args []string) (string, []string) {
	for n, a := range args {
		if strings.HasPrefix(a, `@`) {
			return a[1:], append(args[:n], args[n+1:]...) // remove the found element
		}
	}
	return "", args
}
