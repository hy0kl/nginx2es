package tools

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	SECONDADAY      int64 = 3600 * 24
	MILLSSECONDADAY       = SECONDADAY * 1000
)

func GetDate(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}

	tm := time.Unix(timestamp, 0)
	local, _ := time.LoadLocation("Local")
	return tm.In(local).Format("2006-01-02")
}

// GetDateParse 用于跑批, 或者需要以 UTC时区为基准的时间解析
func GetDateParse(dates string) int64 {
	if "" == dates {
		return 0
	}
	loc, _ := time.LoadLocation("Local")
	parse, _ := time.ParseInLocation("2006-01-02", dates, loc)
	return parse.Unix()
}

func NaturalDay(offset int64) (um int64) {
	t := time.Now()
	date := GetDate(t.Unix())
	baseUm := GetDateParse(date) * 1000
	offsetUm := MILLSSECONDADAY * offset

	um = baseUm + offsetUm

	return
}

func LocalYearMonth(timestamp int64) string {
	tmp := timestamp / 1000

	if tmp <= 0 {
		return "-"
	}

	tm := time.Unix(tmp, 0)
	local, _ := time.LoadLocation("Local")
	return tm.In(local).Format("200601")
}

// Str2TimeByLayout 使用layout将时间字符串转unix时间戳(毫秒)
func Str2TimeByLayout(layout, timeStr string) int64 {
	if "" == timeStr {
		return 0
	}

	loc, _ := time.LoadLocation("Local")
	parse, _ := time.ParseInLocation(layout, timeStr, loc)
	return parse.UnixNano() / 1000000
}

// StrReplace 在 origin 中搜索 search 组,替换成 replace
func StrReplace(origin string, search []string, replace string) (s string) {
	s = origin
	for _, find := range search {
		s = strings.Replace(s, find, replace, -1)
	}

	return
}

func Str2Int(str string) (int, error) {
	number, err := strconv.ParseInt(str, 10, 0)
	return int(number), err
}

func Hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	} else {
		return name
	}
}
