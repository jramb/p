package tools

/**
*
* 2016 by J Ramb
*
**/

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	//"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/satori/go.uuid"
	"github.com/spf13/viper"
	// go get github.com/mattn/go-sqlite3
	_ "github.com/mattn/go-sqlite3"
	//"github.com/ttacon/chalk"
	"github.com/jramb/chalk"
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

type TimeDurationEntry struct {
	Start    string
	Head     string
	Handle   string
	Depth    int
	Duration int64
}

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
		if viper.GetBool("colour") {
			fmt.Println(chalk.Cyan.Color(fmt.Sprint(args...)))
		} else {
			fmt.Println(fmt.Sprint(args...))
		}
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

func GetParam(tx *sql.Tx, param string, whenNew string) string {
	rows := dbQ(tx.Query, `select value from params where param=?`, param)
	defer rows.Close()
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
					(header_uuid, header, handle, active, creation_date, revision, depth, parent)
					values (?, ?, ?, ?, ?, ?, 0, 0)`,
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

func newUUID() string {
	//return uuid.NewV4(). String()
	u := uuid.NewV4()
	return strings.Trim(base64.URLEncoding.EncodeToString(u.Bytes()), "=")
}

func AddHeader(tx *sql.Tx, header string, handle string, parent RowId, depth int) (RowId, error) {
	headerUUUID := newUUID()
	res := dbX(tx.Exec, `insert into headers (header_uuid, header, handle, parent, depth, creation_date, active)
	values(?,?,?,?,?,?,1)`,
		headerUUUID, header, handle, parent, depth, time.Now())
	rowid, err := res.LastInsertId()
	if err != nil {
		return RowId(rowid), err
	}
	fmt.Printf("Inserted %s\n", header)
	return RowId(rowid), nil
}

func addTime(tx *sql.Tx, entry orgEntry, headerId RowId) {
	entryUUID := newUUID()
	_ = dbX(tx.Exec, `insert into entries (entry_uuid, header_id, start, end) values(?,?,?,?)`,
		entryUUID, headerId, entry.start, entry.end)
	//log.Print(fmt.Sprintf("Inserted %s\n", entry))
}

func GetTx(db *sql.DB) (*sql.Tx, error) {
	tx, err := db.Begin()
	return tx, err
}

func OpenDB(checkExists bool) (*sql.DB, error) {
	var dbfile string
	dbfile = viper.GetString("clockfile")
	force := viper.GetBool("force")
	d("clockfile=" + dbfile)
	if !force && (checkExists || dbfile == "") {
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
		autoSync := viper.GetBool("timeserver.autosync")
		if autoSync {
			//FIXME
		}
		r := fn(db, tx)
		if autoSync {
			//FIXME
		}
		return r
	})
}

func PrepareDB(db *sql.DB, tx *sql.Tx) error {
	_ = dbX(tx.Exec, `create table if not exists params
	(param text,value text, primary key (param))`)

	oldVersion := GetParamInt(tx, "version", 0)
	currentVersion := 7

	//fmt.Printf("database type: %T\n", tx)
	_ = dbX(tx.Exec, `create table if not exists headers
	( header_id integer primary key autoincrement unique
	, header_uuid text
	, revision int
	, handle text
	, header text
	, depth int
	, parent integer
	, active boolean
	, creation_date datetime
	)`)
	_ = dbX(tx.Exec, `create table if not exists entries
	( entry_id integer primary key autoincrement unique
	, entry_uuid text
	, revision int
	, header_id integer
	, start datetime not null
	, end datetime)`)
	_ = dbX(tx.Exec, `create table if not exists log ( creation_date datetime, log_text text)`)
	_ = dbX(tx.Exec, `create table if not exists todo
	( todo_id integer primary key autoincrement
	, title text not null
	, handle text
	, creation_date datetime not null
  , done_date datetime)`)
	_ = dbX(tx.Exec, `create unique index if not exists headers_u1 on headers (header_uuid)`)
	_ = dbX(tx.Exec, `create unique index if not exists entries_u1 on entries (entry_uuid)`)

	if oldVersion == currentVersion {
		return nil
	}

	//if oldVersion < 6 && currentVersion >= 6 {
	//_ = dbX(tx.Exec, `alter table headers add revision int`)
	//_ = dbX(tx.Exec, `alter table entries add revision int`)
	//}

	SetParamInt(tx, "version", currentVersion)
	fmt.Println("Initialized database with version", GetParamInt(tx, `version`, 0))

	rows := dbQ(tx.Query, `select rowid from headers where header_uuid is null`)
	defer rows.Close()
	for rows.Next() {
		var rowid int
		rows.Scan(&rowid)
		_ = dbX(tx.Exec, `update headers set header_uuid=?, revision=null where rowid = ? and header_uuid is null`,
			newUUID(), rowid)
	}
	rowsE := dbQ(tx.Query, `select rowid from entries where entry_uuid is null`)
	defer rowsE.Close()
	for rowsE.Next() {
		var rowid int
		rowsE.Scan(&rowid)
		_ = dbX(tx.Exec, `update entries set entry_uuid=?, revision=null where rowid = ? and entry_uuid is null`,
			newUUID(), rowid)
	}

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
	res := dbX(tx.Exec, `update entries set end=?, revision=null where end is null`, effectiveTimeNow)
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
		_ = dbX(tx.Exec, `update entries set start=?, revision=null where rowid = ?`, newStart, rowid)
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

func ChangeCheckIn(tx *sql.Tx, argv []string, handle string, effectiveTimeNow time.Time) error {
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

	res := dbX(tx.Exec, `update entries
set header_id=(select header_id from headers where rowid=?)
, revision=null
where end is null`, hdr)
	updatedCnt, err := res.RowsAffected()
	errCheck(err, `fetching RowsAffected`)
	if updatedCnt > 0 {
		fmt.Println("Switched to " + headerText)
		// d("Changed entries: ", updatedCnt)
	}
	return nil

	// entry := orgEntry{
	// 	lType: clock,
	// 	start: &effectiveTimeNow,
	// 	//end:
	// 	//duration    time.Duration
	// 	//depthChange int
	// }
	// addTime(tx, entry, hdr)
	// fmt.Printf("Checked into %s\n", headerText)
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
		if oe.end != nil && !oe.end.IsZero() {
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
		panic("You did not use the 'force', aborting")
	}
	fmt.Println("Erasing all data")
	_ = dbX(tx.Exec, `delete from entries`)
	_ = dbX(tx.Exec, `delete from headers`)
}

func importOrgData(tx *sql.Tx, clockfile string) {
	headerStack := make([]RowId, 0, 10)
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

func FirstOrEmpty(argv []string) string {
	if len(argv) > 0 {
		return argv[0]
	} else {
		return ""
	}
}

func DecodeTimeFrame(str string) (from, to time.Time, err error) {
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

func printTimeFrame(from, to *time.Time) string {
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
		if viper.GetBool("colour") {
			if handle != "" {
				fmt.Printf(chalk.Green.Color("@%s: ")+chalk.Magenta.Color("%s%s")+"\n", handle, formatDuration(effectiveTimeNow.Sub(start)), extra)
			} else {
				fmt.Printf(chalk.Green.Color("%s: ")+chalk.Magenta.Color("%s%s")+"\n", header, formatDuration(effectiveTimeNow.Sub(start)), extra)
			}
		} else {
			if handle != "" {
				fmt.Printf("@%s: %s%s\n", handle, formatDuration(effectiveTimeNow.Sub(start)), extra)
			} else {
				fmt.Printf("%s: %s%s\n", header, formatDuration(effectiveTimeNow.Sub(start)), extra)
			}
		}
	}
}

func ListLogEntries(db *sql.DB, argv []string) error {
	from, to, err := DecodeTimeFrame(FirstOrEmpty(argv))
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
	// a simple unrounded.Round(rnd) would not consider bias
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

func ShowTimes(db *sql.DB, timeFrame string, argv []string, rounding time.Duration, bias time.Duration) (err error) {
	from, to, err := DecodeTimeFrame(timeFrame) //FirstOrEmpty(argv))
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	rows := dbQ(db.Query, `
select rowid, header, handle, depth,
  (select sum(strftime('%s',coalesce(end,current_timestamp))-strftime('%s',start)) sum_duration
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

	fmt.Println("Headers:", printTimeFrame(&from, &to))
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

func QueryDays(db *sql.DB, from, to time.Time, filter string, rounding time.Duration, bias time.Duration) ([]TimeDurationEntry, error) {
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

	ret := make([]TimeDurationEntry, 0, 16)

	for rows.Next() {
		var e TimeDurationEntry
		rows.Scan(&e.Start, &e.Head, &e.Handle, &e.Depth, &e.Duration)
		dur := time.Duration(e.Duration * 1000000000)
		rounded := DurationRound(dur, rounding, bias)
		diff := dur - rounded
		rounderr += diff
		dur = rounded
		ret = append(ret, e)
		total += dur
	}
	return ret, nil
}

// type listDays map[int]time.Duration
type listOfWeekDays [8]time.Duration

type headerDays map[string]*listOfWeekDays

func printWeek(week headerDays) {
	maxLen := 0
	withSub := viper.GetBool("show.subheaders")
	// calculate sum of days
	var sumDays listOfWeekDays
	for _, days := range week {
		for n, v := range days {
			sumDays[n] = sumDays[n] + v
		}
	}
	// find max length of header
	for header, _ := range week {
		currLen := 0
		if withSub {
			headerParts := strings.Split(header, ":")
			currLen = 2*(len(headerParts)-1) + len(headerParts[len(headerParts)-1]) + 1
		} else {
			currLen = len(header) + 1
		}
		if l := currLen; l > maxLen {
			maxLen = l
		}
	}
	if maxLen == 0 {
		return
	}
	if withSub {
		// add subtotals  A:B:C -> A:B and A
		for header, days := range week {
			headerParts := strings.Split(header, ":")
			_ = days
			for i := 1; i < len(headerParts); i++ {
				subHdr := strings.Join(headerParts[:i], ":")
				if _, ok := week[subHdr]; !ok {
					week[subHdr] = new(listOfWeekDays)
				}
				for n, v := range days {
					week[subHdr][n] = week[subHdr][n] + v
				}
			}
		}
	}
	hdrFmt := fmt.Sprintf("%%-%ds", maxLen)
	fmt.Printf(hdrFmt+"| %6s | %6s | %6s | %6s | %6s | %6s | %6s | %6s\n",
		"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun", "SUM")
	divider := fmt.Sprintf(strings.Repeat("-", maxLen) + strings.Repeat("+--------", 8))
	fmt.Println(divider)
	var keys []string
	for k := range week {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, header := range keys {
		if withSub {
			headerParts := strings.Split(header, ":")
			fmt.Printf(hdrFmt, strings.Repeat("  ", len(headerParts)-1)+headerParts[len(headerParts)-1])
		} else {
			fmt.Printf(hdrFmt, header)
		}
		for _, v := range week[header] {
			if v != 0 {
				fmt.Printf("| %6s ", formatDuration(v))
			} else {
				fmt.Printf("| %6s ", "")
			}
		}
		fmt.Println()
	}
	fmt.Println(divider)
	fmt.Printf(hdrFmt, "TOTAL")
	for _, v := range sumDays {
		if v != 0 {
			fmt.Printf("| %6s ", formatDuration(v))
		} else {
			fmt.Printf("| %6s ", "")
		}
	}
	fmt.Println()

}

func ShowWeek(db *sql.DB, timeFrame string, argv []string, rounding time.Duration, bias time.Duration) error {
	from, to, err := DecodeTimeFrame(timeFrame) //FirstOrEmpty(argv))
	days := to.Sub(from) / time.Hour / 24
	if days != 7 {
		fmt.Printf("Number days = %d, currently only single weeks are supported\n", int64(days))
		return nil
	}
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	//fmt.Println("From, to:", from, to)
	rows := dbQ(db.Query, `
with p as (select ? pfrom, ? pto),
b as (select h.header, h.handle, date(start) start_date, (strftime('%s',coalesce(end,current_timestamp))-strftime('%s',start)) duration
from entries e
join headers h on h.header_id = e.header_id and h.active=1
join p
where 1=1 --e.end is not null
and e.start between p.pfrom and p.pto)
select start_date, round(julianday(start_date)-julianday(p.pfrom)) diff, header, handle, sum(duration)
from b, p
where lower(header) like lower('%'||?||'%')
or '@'||handle = ?
group by header, handle, start_date
order by header, start_date asc
`, from, to, filter, filter)
	defer rows.Close()
	total := time.Duration(0)
	rounderr := time.Duration(0)

	week := make(headerDays)

	fmt.Println("Week:", printTimeFrame(&from, &to))
	for rows.Next() {
		var start string //time.Time //string
		var offset int   //time.Time //string
		var head string
		var handle string
		var duration int64
		if error := rows.Scan(&start, &offset, &head, &handle, &duration); error != nil {
			fmt.Println(error)
		}
		dur := time.Duration(duration * 1000000000)
		rounded := DurationRound(dur, rounding, bias)
		diff := dur - rounded
		rounderr += diff
		dur = rounded
		if _, ok := week[head]; !ok {
			week[head] = new(listOfWeekDays)
		}
		week[head][offset] = time.Duration(dur)
		week[head][7] += time.Duration(dur)

		total += dur
	}
	printWeek(week)
	fmt.Printf("     Total: %9s%s\n", formatDuration(total), formatRoundErr(rounderr))
	return nil
}

func ShowDays(db *sql.DB, timeFrame string, argv []string, rounding time.Duration, bias time.Duration) error {
	from, to, err := DecodeTimeFrame(timeFrame) //FirstOrEmpty(argv))
	days := to.Sub(from) / time.Hour / 24
	fmt.Printf("Number days = %d\n", int64(days))
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	//fmt.Println("From, to:", from, to)
	rows := dbQ(db.Query, `
with p as (select ? pfrom, ? pto),
b as (select h.header, h.handle, date(start) start_date, (strftime('%s',coalesce(end,current_timestamp))-strftime('%s',start)) duration
from entries e
join headers h on h.header_id = e.header_id and h.active=1
join p
where 1=1 --e.end is not null
and e.start between p.pfrom and p.pto)
select start_date, round(julianday(start_date)-julianday(p.pfrom)) diff, header, handle, sum(duration)
from b, p
where lower(header) like lower('%'||?||'%')
or '@'||handle = ?
group by header, handle, start_date
order by header, start_date asc
`, from, to, filter, filter)
	defer rows.Close()
	total := time.Duration(0)
	rounderr := time.Duration(0)

	fmt.Println("Daily:", printTimeFrame(&from, &to))
	for rows.Next() {
		var start string //time.Time //string
		var offset int   //time.Time //string
		var head string
		var handle string
		var duration int64
		if error := rows.Scan(&start, &offset, &head, &handle, &duration); error != nil {
			fmt.Println(error)
		}
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
	from, to, err := DecodeTimeFrame(FirstOrEmpty(argv))
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	hdrs := dbQ(db.Query, `select header_id, depth, header
	from headers
	where active=1
	and lower(header) like lower('%'||?||'%')
	and header_id in (select header_id
	from entries where
		(start between ? and ? or (end is null and current_timestamp between ? and ?)))`, filter, from, to, from, to)
	defer hdrs.Close()
	for hdrs.Next() {
		var hid int
		var headerTxt string
		var depth int
		hdrs.Scan(&hid, &depth, &headerTxt)
		headEntry := orgEntry{
			lType:  header,
			deep:   depth,
			header: headerTxt,
		}
		entr := dbQ(db.Query, `select start, end, strftime('%s',end)-strftime('%s',start) duration
		from entries
		where header_id = ?
		and (start between ? and ? or (end is null and current_timestamp between ? and ?))
		order by start desc`, hid, from, to, from, to)
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

func ShowLedger(db *sql.DB, argv []string, rounding time.Duration, bias time.Duration) (err error) {
	from, to, err := DecodeTimeFrame(FirstOrEmpty(argv))
	if err != nil {
		return err
	}
	var filter string
	if len(argv) > 1 {
		filter = argv[1]
	}
	entr := dbQ(db.Query, `select h.header_id, h.depth, h.header, h.handle, e.start, e.end
		from entries e
                join headers h on h.header_id = e.header_id
		where lower(header) like lower('%'||?||'%')
		and (start between ? and ? or (end is null and current_timestamp between ? and ?))
		order by h.header_id, e.start asc`, filter, from, to, from, to)
	defer entr.Close()
	roundDay := ""
	roundHeader := ""
	roundDur := time.Duration(0)
	for entr.Next() {
		var start *time.Time
		var end *time.Time
		var hid int
		var depth int
		var headerTxt string
		var handle *string
		entr.Scan(&hid, &depth, &headerTxt, &handle, &start, &end)
		var handleStr string
		if handle != nil {
			handleStr = "  " + *handle
		} else {
			handleStr = ""
		}
		if start == nil {
			fmt.Printf(";Error %s -- %s %s\n", start, end, headerTxt)
		} else {
			thisDay := start.Format(simpleDateFormat)
			if !(roundDay == thisDay && roundHeader == headerTxt) {
				// fmt.Printf("Switch!")
				// add rounding
				rounded := DurationRound(roundDur, rounding, bias)
				roundval := time.Duration(rounded - roundDur)
				if roundval >= time.Minute || roundval <= -time.Minute {
					// fmt.Printf("%s (%s)%s\n", roundDay, "rounding", "rounding")
					fmt.Printf("%s  %s\n", roundDay, "rounding")
					fmt.Printf("    (%s)  %ds\n", roundHeader, int64(roundval/time.Second))
				}
				roundDay = thisDay
				roundHeader = headerTxt
				roundDur = time.Duration(0)
			}
			if end == nil {
				fmt.Printf("i %s %s%s\n", start.Format(isoDateTime), headerTxt, handleStr)
			} else {
				dur := end.Sub(*start) // should be >=0 now
				if start.After(*end) { // end<start (ledger can't handle it directly)
					fmt.Printf("%s (%s)%s\n", start.Format(simpleDateFormat), "", handleStr)
					fmt.Printf("    ; %s -- %s\n", start.Format(isoDateTime), end.Format(isoDateTime))
					fmt.Printf("    (%s)   %ds\n", headerTxt, int64(dur/time.Second))
				} else { // start<end (normal)
					fmt.Printf("i %s %s%s\n", start.Format(isoDateTime), headerTxt, handleStr)
					fmt.Printf("o %s\n", end.Format(isoDateTime))
				}
				roundDur += dur
			}
		}
	}
	// add last rounding as well
	if roundDur != time.Duration(0) {
		// fmt.Printf("Switch!")
		// add rounding
		rounded := DurationRound(roundDur, rounding, bias)
		roundval := time.Duration(rounded - roundDur)
		if roundval >= time.Minute || roundval <= -time.Minute {
			fmt.Printf("%s (%s)%s\n", roundDay, "rounding", "")
			fmt.Printf("    (%s)  %ds\n", roundHeader, int64(roundval/time.Second))
		}
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
