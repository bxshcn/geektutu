package log

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestSetLevel(t *testing.T) {
	testCases := []struct {
		desc  string
		level int
	}{
		{
			desc:  "info level",
			level: InfoLevel,
		},
		{
			desc:  "error level",
			level: ErrorLevel,
		},
		{
			desc:  "more high level",
			level: 3,
		},
		{
			desc:  "more high level",
			level: 3,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			SetLevel(tC.level)
			switch tC.level {
			case InfoLevel:
				if infoLog.Writer() != os.Stdout && errorLog.Writer() != os.Stdout {
					t.Errorf("should log info and error")
				}
			case ErrorLevel:
				if infoLog.Writer() != ioutil.Discard && errorLog.Writer() != os.Stdout {
					t.Errorf("should log only error")
				}
			case Disabled:
				if infoLog.Writer() != ioutil.Discard && errorLog.Writer() != ioutil.Discard {
					t.Errorf("should not log")
				}
			default:
				if infoLog.Writer() != os.Stdout && errorLog.Writer() != os.Stdout {
					t.Errorf("should log info and error")
				}
			}
		})
	}
}
