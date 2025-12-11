package butterflymx

import (
	"encoding"
	"fmt"
	"time"
)

// Weekday represents a day of the week.
type Weekday string

const (
	Monday    Weekday = "mon"
	Tuesday   Weekday = "tue"
	Wednesday Weekday = "wed"
	Thursday  Weekday = "thu"
	Friday    Weekday = "fri"
	Saturday  Weekday = "sat"
	Sunday    Weekday = "sun"
)

// ToTimeWeekday converts the Weekday to [time.Weekday].
func (w Weekday) ToTimeWeekday() time.Weekday {
	switch w {
	case Monday:
		return time.Monday
	case Tuesday:
		return time.Tuesday
	case Wednesday:
		return time.Wednesday
	case Thursday:
		return time.Thursday
	case Friday:
		return time.Friday
	case Saturday:
		return time.Saturday
	case Sunday:
		return time.Sunday
	default:
		return -1
	}
}

// Datestamp represents a date in year, month, day format and without a
// timezone.
type Datestamp struct {
	Year  int
	Month time.Month
	Day   int
}

const DatestampLayout = "2006-01-02"

var (
	_ encoding.TextMarshaler   = (*Datestamp)(nil)
	_ encoding.TextUnmarshaler = (*Datestamp)(nil)
	_ fmt.Stringer             = (*Datestamp)(nil)
)

// UnmarshalText implements [encoding.TextUnmarshaler].
func (d *Datestamp) UnmarshalText(text []byte) error {
	tt, err := time.ParseInLocation(DatestampLayout, string(text), time.UTC)
	if err != nil {
		return err
	}
	*d = Datestamp{Year: tt.Year(), Month: tt.Month(), Day: tt.Day()}
	return nil
}

// MarshalText implements [encoding.TextMarshaler].
func (d Datestamp) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// String returns the string representation of the Datestamp.
func (d Datestamp) String() string {
	return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
}

// ToTime converts the Datestamp to a time.Time in the given timezone at
// midnight.
func (d Datestamp) ToTime(tz *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, tz)
}

// Timestamp represents a time of day in the format [TimestampLayout].
type Timestamp struct {
	Hour   int
	Minute int
}

const TimestampLayout = "15:04"

var (
	_ encoding.TextMarshaler   = (*Timestamp)(nil)
	_ encoding.TextUnmarshaler = (*Timestamp)(nil)
	_ fmt.Stringer             = (*Timestamp)(nil)
)

// UnmarshalText implements [encoding.TextUnmarshaler].
func (wt *Timestamp) UnmarshalText(text []byte) error {
	t, err := time.Parse(TimestampLayout, string(text))
	if err != nil {
		return err
	}
	*wt = Timestamp{Hour: t.Hour(), Minute: t.Minute()}
	return nil
}

// MarshalText implements [encoding.TextMarshaler].
func (wt Timestamp) MarshalText() ([]byte, error) {
	return []byte(wt.String()), nil
}

// String returns the string representation of the WatchTime.
func (wt Timestamp) String() string {
	return fmt.Sprintf("%02d:%02d", wt.Hour, wt.Minute)
}

// ToTime converts the WatchTime to a time.Time on the given date using that
// date's timezone.
func (wt Timestamp) ToTime(date time.Time) time.Time {
	date = date.Truncate(24 * time.Hour)
	date = date.Add(time.Duration(wt.Hour) * time.Hour)
	date = date.Add(time.Duration(wt.Minute) * time.Minute)
	return date
}
